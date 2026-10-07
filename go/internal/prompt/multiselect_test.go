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
		// Like Laravel Prompts, a blank line follows the submitted question.
		" │ Codex (not installed)" + strings.Repeat(" ", 60-len("Codex (not installed)")) + " │\n └" + strings.Repeat("─", 62) + "┘\n\n",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("output does not contain %q:\n%s", want, plain)
		}
	}
}

// The bytes Laravel Prompts writes on a terminal for the same keys,
// recorded from the PHP CLI under a pseudo-terminal: its colors, how it
// redraws the question in place (only when it changes) and the blank line
// after it.
func TestMultiSelectWritesWhatLaravelPromptsWrites(t *testing.T) {
	var out bytes.Buffer
	terminal := prompt.NewTerminalForTest(&keys{pressed: []string{"x", "j", "\r"}}, &out)
	choices := []prompt.Choice{{Value: "claude-code", Label: "Claude Code"}, {Value: "codex", Label: "Codex (not installed)"}}

	if _, err := terminal.MultiSelect("Which agents do you use?", hint, choices, []string{"claude-code"}); err != nil {
		t.Fatal(err)
	}

	gray := func(s string) string { return "\x1b[90m" + s + "\x1b[39m" }
	cyan := func(s string) string { return "\x1b[36m" + s + "\x1b[39m" }
	dim := func(s string) string { return "\x1b[2m" + s + "\x1b[22m" }
	pad := func(s string, width int) string { return s + strings.Repeat(" ", 60-width) }
	top := func(title string) string {
		return gray(" ┌") + " " + title + " " + gray(strings.Repeat("─", 60-len("Which agents do you use?"))+"┐") + "\r\n"
	}
	row := func(s string, width int) string {
		return gray(" │") + " " + pad(s, width) + " " + gray("│") + "\r\n"
	}
	bottom := gray(" └"+strings.Repeat("─", 62)+"┘") + "\r\n"
	redraw := "\x1b[1G\x1b[6A\x1b[J\r\n"
	want := "\x1b[?25l\r\n" +
		top(cyan("Which agents do you use?")) +
		row(cyan("› ◼")+" Claude Code  ", 17) +
		row("  "+dim("◻")+" "+dim("Codex (not installed)")+"  ", 27) +
		bottom + gray("  "+hint) + "\r\n" +
		redraw +
		top(cyan("Which agents do you use?")) +
		row("  "+cyan("◼")+" "+dim("Claude Code")+"  ", 17) +
		row(cyan("›")+" ◻ Codex (not installed)  ", 27) +
		bottom + gray("  "+hint) + "\r\n" +
		"\x1b[1G\x1b[6A\x1b[J\r\n" +
		top(dim("Which agents do you use?")) +
		row("Claude Code", 11) +
		bottom + "\r\n" +
		"\x1b[?25h"
	if out.String() != want {
		t.Errorf("output =\n%q\nwant\n%q", out.String(), want)
	}
}

// Like Laravel Prompts, the answered question lists the selected labels
// in the order of the choices, while the answer keeps the order they were
// selected in.
func TestMultiSelectListsTheSelectedLabelsInTheOrderOfTheChoices(t *testing.T) {
	selected, output, _ := multiSelect(t, []string{"windsurf"}, " ", "\r")

	if !reflect.DeepEqual(selected, []string{"windsurf", "claude-code"}) {
		t.Errorf("MultiSelect = %q, want [windsurf claude-code]", selected)
	}
	plain := ansi.ReplaceAllString(output, "")
	if !strings.Contains(plain, " │ Claude Code"+strings.Repeat(" ", 49)+" │\r\n │ Windsurf ") {
		t.Errorf("output does not list Claude Code before Windsurf:\n%s", plain)
	}
}

// One read may hold several keys, or only part of an escape sequence:
// the keys are split out of the bytes read, an incomplete sequence kept
// for the next read. (Laravel Prompts takes each read for one key.)
func TestMultiSelectSplitsTheKeysOfEachRead(t *testing.T) {
	tests := []struct {
		name    string
		pressed []string
		want    []string
	}{
		{"two keys in one read", []string{"j ", "\r"}, []string{"codex"}},
		{"an arrow split across reads", []string{"\x1b[", "B", " ", "\r"}, []string{"codex"}},
		{"an escape alone, then the rest", []string{"\x1b", "[B", " \r"}, []string{"codex"}},
		{"a pasted sequence", []string{"j j \x1b[A\x1bOB\x1b[A \r"}, []string{"windsurf"}},
		{"keys after enter are left", []string{" \rj "}, []string{"claude-code"}},
	}
	for _, tt := range tests {
		selected, _, err := multiSelect(t, nil, tt.pressed...)

		if err != nil || !reflect.DeepEqual(selected, tt.want) {
			t.Errorf("%s: MultiSelect = %q, %v; want %q", tt.name, selected, err, tt.want)
		}
	}
}

// Repeated defaults are one selection: toggling the choice unselects it,
// and Ctrl+A checks which choices are selected, not how many values.
func TestMultiSelectTakesRepeatedDefaultsOnce(t *testing.T) {
	selected, _, _ := multiSelect(t, []string{"codex", "codex"}, "j", " ", "\r")
	if len(selected) != 0 {
		t.Errorf("MultiSelect after unselecting = %q, want none", selected)
	}

	selected, _, _ = multiSelect(t, []string{"codex", "codex", "windsurf"}, "\x01", "\r")
	if !reflect.DeepEqual(selected, []string{"claude-code", "codex", "windsurf"}) {
		t.Errorf("MultiSelect after Ctrl+A = %q, want all", selected)
	}

	selected, _, _ = multiSelect(t, []string{"codex", "codex"}, "\r")
	if !reflect.DeepEqual(selected, []string{"codex"}) {
		t.Errorf("MultiSelect = %q, want [codex]", selected)
	}
}
