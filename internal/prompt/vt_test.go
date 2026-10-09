package prompt_test

import (
	"bytes"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/prompt"
)

// S1.8.a: when the console cannot process escape sequences, MultiSelect
// answers with the defaults and says so in plain text.
func TestMultiSelectAnswersTheDefaultsInPlainTextWhenTheTerminalHasNoVT(t *testing.T) {
	defer prompt.SetEnableVT(func(io.Writer) bool { return false })()
	var out bytes.Buffer
	terminal := prompt.NewTerminalForTest(&keys{pressed: []string{"j", " ", "\r"}}, &out)

	selected, err := terminal.MultiSelect("Which agents do you use?", hint, agentChoices, []string{"claude-code", "windsurf"})

	if err != nil || !reflect.DeepEqual(selected, []string{"claude-code", "windsurf"}) {
		t.Errorf("MultiSelect = %q, %v; want the defaults, nil", selected, err)
	}
	if want := "Agents: Claude Code, Windsurf (pass --agents to choose).\n"; out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
	if strings.Contains(out.String(), "\x1b[") {
		t.Errorf("output = %q, want no escape sequences", out.String())
	}
}

// S1.8.b (pin): a terminal that enables VT is asked on the writer it prints
// to and draws as it does today (the Laravel bytes test covers the drawing).
func TestNewEnablesVTOnTheOutputItPrintsTo(t *testing.T) {
	var asked io.Writer
	defer prompt.SetEnableVT(func(out io.Writer) bool { asked = out; return true })()
	var out bytes.Buffer

	prompt.New(strings.NewReader(""), &out)

	if asked != &out {
		t.Errorf("enableVT got %v, want the output", asked)
	}
}

func TestMultiSelectWithoutVTAndWithoutDefaultsSaysNone(t *testing.T) {
	defer prompt.SetEnableVT(func(io.Writer) bool { return false })()
	var out bytes.Buffer
	terminal := prompt.NewTerminalForTest(&keys{}, &out)

	selected, err := terminal.MultiSelect("Which agents do you use?", hint, agentChoices, []string{})

	if err != nil || len(selected) != 0 {
		t.Errorf("MultiSelect = %q, %v; want none, nil", selected, err)
	}
	if want := "Agents: none (pass --agents to choose).\n"; out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
}
