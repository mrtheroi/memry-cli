package prompt_test

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mrtheroi/memry-cli/internal/prompt"
)

var agentChoices = []prompt.Choice{
	{Value: "claude-code", Label: "Claude Code"},
	{Value: "codex", Label: "Codex (not installed)"},
	{Value: "windsurf", Label: "Windsurf"},
}

const hint = "Space to select, enter to confirm."

// keys is an input that returns one key per read, as a terminal in raw
// mode does.
type keys struct {
	pressed []string
}

func (k *keys) Read(p []byte) (int, error) {
	if len(k.pressed) == 0 {
		return 0, io.EOF
	}
	n := copy(p, k.pressed[0])
	k.pressed = k.pressed[1:]
	return n, nil
}

func multiSelect(t *testing.T, defaults []string, pressed ...string) ([]string, string, error) {
	t.Helper()
	var out bytes.Buffer
	terminal := prompt.NewTerminalForTest(&keys{pressed: pressed}, &out)
	selected, err := terminal.MultiSelect("Which agents do you use?", hint, agentChoices, defaults)
	return selected, out.String(), err
}

// Like Laravel Prompts when the input is not a terminal: no question, the
// defaults are the answer.
func TestMultiSelectAnswersTheDefaultsWithoutAskingWhenNotATerminal(t *testing.T) {
	var out bytes.Buffer
	terminal := prompt.New(strings.NewReader("anything\n"), &out)

	selected, err := terminal.MultiSelect("Which agents do you use?", hint, agentChoices, []string{"codex"})

	if err != nil || !reflect.DeepEqual(selected, []string{"codex"}) {
		t.Errorf("MultiSelect = %q, %v; want [codex], nil", selected, err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want none", out.String())
	}
}

func TestMultiSelectSubmitsTheDefaultsOnEnter(t *testing.T) {
	for _, enter := range []string{"\r", "\n"} {
		selected, _, err := multiSelect(t, []string{"codex"}, enter)

		if err != nil || !reflect.DeepEqual(selected, []string{"codex"}) {
			t.Errorf("MultiSelect = %q, %v; want [codex], nil", selected, err)
		}
	}
}

// Like Laravel Prompts, a toggled value is added after the others.
func TestMultiSelectTogglesTheHighlightedChoiceWithSpace(t *testing.T) {
	selected, _, err := multiSelect(t, []string{"windsurf"}, "\x1b[B", " ", "\x1b[A", " ", "\r")

	if err != nil || !reflect.DeepEqual(selected, []string{"windsurf", "codex", "claude-code"}) {
		t.Errorf("MultiSelect = %q, %v; want [windsurf codex claude-code], nil", selected, err)
	}

	selected, _, _ = multiSelect(t, []string{"claude-code", "codex"}, " ", "\r")
	if !reflect.DeepEqual(selected, []string{"codex"}) {
		t.Errorf("MultiSelect after unselecting = %q, want [codex]", selected)
	}
}

// Up from the first choice goes to the last, down from the last to the
// first; j, k, tab and the other Laravel Prompts keys move too.
func TestMultiSelectWrapsAroundAndKnowsTheVimKeys(t *testing.T) {
	selected, _, _ := multiSelect(t, nil, "\x1b[A", " ", "j", " ", "\r")
	if !reflect.DeepEqual(selected, []string{"windsurf", "claude-code"}) {
		t.Errorf("MultiSelect = %q, want [windsurf claude-code]", selected)
	}

	selected, _, _ = multiSelect(t, nil, "\t", "\t", "k", " ", "\r")
	if !reflect.DeepEqual(selected, []string{"codex"}) {
		t.Errorf("MultiSelect = %q, want [codex]", selected)
	}
}

// Ctrl+A selects every choice, or none once all are selected.
func TestMultiSelectTogglesAllWithCtrlA(t *testing.T) {
	selected, _, _ := multiSelect(t, []string{"codex"}, "\x01", "\r")
	if !reflect.DeepEqual(selected, []string{"claude-code", "codex", "windsurf"}) {
		t.Errorf("MultiSelect = %q, want all", selected)
	}

	selected, _, _ = multiSelect(t, []string{"codex"}, "\x01", "\x01", "\r")
	if len(selected) != 0 {
		t.Errorf("MultiSelect = %q, want none", selected)
	}
}

func TestMultiSelectIsCancelledByCtrlCOrTheEndOfTheInput(t *testing.T) {
	for _, pressed := range [][]string{{"\x03"}, {" "}} {
		if selected, _, err := multiSelect(t, []string{"codex"}, pressed...); !errors.Is(err, prompt.ErrAborted) {
			t.Errorf("MultiSelect after %q = %q, %v; want ErrAborted", pressed, selected, err)
		}
	}
}

var ansi = regexp.MustCompile("\x1b\\[[0-9;?]*[A-Za-z]")

// The question is drawn like Laravel Prompts' multiselect: a box with the
// label, a line per choice, then the hint; once submitted, the box lists
// the selected labels.
func TestMultiSelectDrawsTheQuestionLikeLaravelPrompts(t *testing.T) {
	_, output, _ := multiSelect(t, []string{"codex"}, "\r")
	plain := ansi.ReplaceAllString(strings.ReplaceAll(output, "\r\n", "\n"), "")

	for _, want := range []string{
		" ┌ Which agents do you use? " + strings.Repeat("─", 60-len("Which agents do you use?")) + "┐\n",
		" │ › ◻ Claude Code" + strings.Repeat(" ", 60-utf8.RuneCountInString("› ◻ Claude Code")) + " │\n",
		" │   ◼ Codex (not installed)",
		" │   ◻ Windsurf",
		" └" + strings.Repeat("─", 62) + "┘\n  Space to select, enter to confirm.\n",
		" │ Codex (not installed)" + strings.Repeat(" ", 60-len("Codex (not installed)")) + " │\n",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("output does not contain %q:\n%s", want, plain)
		}
	}
}
