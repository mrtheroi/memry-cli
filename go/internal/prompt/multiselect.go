package prompt

import (
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Choice is an option of a multiselect: the value it selects and the
// label shown.
type Choice struct {
	Value, Label string
}

// MultiSelect asks to pick any of the choices, starting from defaults, the
// way Laravel Prompts' multiselect does on a terminal: arrows (or j/k,
// tab) move, space toggles, Ctrl+A toggles all, enter submits and Ctrl+C
// cancels with ErrAborted. Like Laravel Prompts, it answers with the
// defaults without asking when the input is not a terminal.
func (t *Terminal) MultiSelect(label, hint string, choices []Choice, defaults []string) ([]string, error) {
	if t.raw == nil {
		return defaults, nil
	}
	restore, err := t.raw()
	if err != nil {
		return defaults, nil
	}
	defer restore()
	m := &multiSelect{label: label, hint: hint, choices: choices, values: slices.Clone(defaults)}
	screen := &screen{out: t.out}
	_, _ = io.WriteString(t.out, hideCursor)
	defer func() { _, _ = io.WriteString(t.out, showCursor) }()
	screen.draw(m.render())
	buf := make([]byte, 16)
	for {
		n, err := t.in.Read(buf)
		if err != nil {
			m.state = "cancel"
			screen.draw(m.render())
			return nil, ErrAborted
		}
		m.press(string(buf[:n]))
		screen.draw(m.render())
		switch m.state {
		case "submit":
			return m.values, nil
		case "cancel":
			return nil, ErrAborted
		}
	}
}

// The keys Laravel Prompts' multiselect knows.
var (
	previousKeys = []string{"\x1b[A", "\x1bOA", "\x1b[D", "\x1bOD", "\x1b[Z", "\x10", "\x02", "k", "h"}
	nextKeys     = []string{"\x1b[B", "\x1bOB", "\x1b[C", "\x1bOC", "\t", "\x0e", "\x06", "j", "l"}
	homeKeys     = []string{"\x1b[1~", "\x1bOH", "\x1b[H", "\x1b[7~"}
	endKeys      = []string{"\x1b[4~", "\x1bOF", "\x1b[F", "\x1b[8~"}
)

// multiSelect is the state of a multiselect question.
type multiSelect struct {
	label, hint string
	choices     []Choice
	// values are the selected values, in the order they were selected.
	values      []string
	highlighted int
	// state is "active", "submit" or "cancel".
	state string
}

func (m *multiSelect) press(key string) {
	count := len(m.choices)
	switch {
	case slices.Contains(previousKeys, key):
		m.highlighted = (m.highlighted - 1 + count) % count
	case slices.Contains(nextKeys, key):
		m.highlighted = (m.highlighted + 1) % count
	case slices.Contains(homeKeys, key):
		m.highlighted = 0
	case slices.Contains(endKeys, key):
		m.highlighted = count - 1
	case key == " ":
		value := m.choices[m.highlighted].Value
		if i := slices.Index(m.values, value); i >= 0 {
			m.values = slices.Delete(m.values, i, i+1)
		} else {
			m.values = append(m.values, value)
		}
	case key == "\x01":
		if len(m.values) == count {
			m.values = []string{}
		} else {
			m.values = []string{}
			for _, choice := range m.choices {
				m.values = append(m.values, choice.Value)
			}
		}
	case key == "\r" || key == "\n":
		m.state = "submit"
	case key == "\x03":
		m.state = "cancel"
	}
}

// ANSI sequences the question is drawn with.
const (
	hideCursor = "\x1b[?25l"
	showCursor = "\x1b[?25h"
	cyan       = "\x1b[36m"
	gray       = "\x1b[90m"
	red        = "\x1b[31m"
	dim        = "\x1b[2m"
	reset      = "\x1b[0m"
)

// render draws the question like Laravel Prompts' default theme.
func (m *multiSelect) render() []string {
	switch m.state {
	case "submit":
		// The labels in the order the values were selected.
		var body []string
		for _, value := range m.values {
			for _, choice := range m.choices {
				if choice.Value == value {
					body = append(body, choice.Label)
				}
			}
		}
		if len(body) == 0 {
			body = []string{gray + "None" + reset}
		}
		return box(dim+m.label+reset, body, gray)
	case "cancel":
		return append(box(m.label, m.options(), red), red+"  ⚠ Cancelled."+reset)
	}
	return append(box(cyan+m.label+reset, m.options(), gray), gray+"  "+m.hint+reset)
}

// options are the lines of the choices.
func (m *multiSelect) options() []string {
	lines := make([]string, len(m.choices))
	for i, choice := range m.choices {
		active, selected := i == m.highlighted, slices.Contains(m.values, choice.Value)
		switch {
		case m.state == "cancel":
			mark := "◻"
			if selected {
				mark = "◼"
			}
			pointer := " "
			if active {
				pointer = "›"
			}
			lines[i] = dim + pointer + " " + mark + " " + choice.Label + "  " + reset
		case active && selected:
			lines[i] = cyan + "› ◼" + reset + " " + choice.Label + "  "
		case active:
			lines[i] = cyan + "›" + reset + " ◻ " + choice.Label + "  "
		case selected:
			lines[i] = "  " + cyan + "◼" + reset + " " + dim + choice.Label + reset + "  "
		default:
			lines[i] = "  " + dim + "◻" + reset + " " + dim + choice.Label + reset + "  "
		}
	}
	return lines
}

// minBoxWidth is the width of a box's content in Laravel Prompts.
const minBoxWidth = 60

// box draws title and body in a box at least minBoxWidth wide.
func box(title string, body []string, color string) []string {
	width := max(minBoxWidth, visibleWidth(title))
	for _, line := range body {
		width = max(width, visibleWidth(line))
	}
	lines := []string{color + " ┌" + reset + " " + title + " " + color + strings.Repeat("─", width-visibleWidth(title)) + "┐" + reset}
	for _, line := range body {
		lines = append(lines, color+" │"+reset+" "+line+strings.Repeat(" ", width-visibleWidth(line))+" "+color+"│"+reset)
	}
	return append(lines, color+" └"+strings.Repeat("─", width+2)+"┘"+reset)
}

// visibleWidth is the number of characters of s shown, without its ANSI
// sequences.
func visibleWidth(s string) int {
	width := 0
	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			end := strings.IndexByte(s[i:], 'm')
			if end < 0 {
				break
			}
			i += end + 1
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
		width++
	}
	return width
}

// screen redraws a frame of lines in place. In raw mode a newline does
// not return the cursor, so lines end with "\r\n".
type screen struct {
	out   io.Writer
	lines int
}

func (s *screen) draw(frame []string) {
	var b strings.Builder
	if s.lines == 0 {
		b.WriteString("\r\n")
	} else {
		// Back to the first line of the previous frame, which is cleared.
		b.WriteString("\x1b[" + strconv.Itoa(s.lines) + "A\r\x1b[J")
	}
	for _, line := range frame {
		b.WriteString(line + "\r\n")
	}
	s.lines = len(frame)
	_, _ = io.WriteString(s.out, b.String())
}
