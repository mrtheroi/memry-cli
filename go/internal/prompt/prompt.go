// Package prompt asks questions on the terminal like the PHP CLI.
package prompt

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// ErrAborted is returned when the input ends before an answer, where
// Symfony aborts with "Aborted.".
var ErrAborted = errors.New("the input ended before an answer")

// Terminal asks questions on in, printing them to out.
type Terminal struct {
	in  *bufio.Reader
	out io.Writer
	// ended is set once a read reaches the end of the input.
	ended bool
	// readHidden reads a hidden answer when in is a terminal.
	readHidden func() ([]byte, error)
}

// New returns a Terminal reading answers from in. When in is a terminal,
// hidden answers are read with its echo turned off.
func New(in io.Reader, out io.Writer) *Terminal {
	t := &Terminal{in: bufio.NewReader(in), out: out}
	if file, ok := in.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		t.readHidden = func() ([]byte, error) { return term.ReadPassword(int(file.Fd())) }
	}
	return t
}

// Ask prints the question like Symfony's question helper and reads one
// line, trimmed like PHP's trim().
func (t *Terminal) Ask(label string) (string, error) {
	_, _ = fmt.Fprintf(t.out, "\n %s:\n > ", label)
	line, err := t.readLine()
	if err != nil {
		return "", err
	}
	_, _ = io.WriteString(t.out, "\n")
	return phpTrim(line), nil
}

// readHiddenLine reads a line without echo on a terminal, else like
// readLine (Symfony turns the echo off with stty, which a pipe ignores).
func (t *Terminal) readHiddenLine() (string, error) {
	if t.readHidden == nil {
		return t.readLine()
	}
	line, err := t.readHidden()
	if err != nil {
		t.ended = true
		return "", ErrAborted
	}
	return string(line), nil
}

// readLine reads one line like Symfony's QuestionHelper: once the input
// has ended, every answer is empty; reaching the end before any character
// aborts.
func (t *Terminal) readLine() (string, error) {
	if t.ended {
		return "", nil
	}
	line, err := t.in.ReadString('\n')
	if err != nil {
		t.ended = true
		if line == "" {
			return "", ErrAborted
		}
	}
	return line, nil
}

// phpTrim trims the characters PHP's trim() does.
func phpTrim(s string) string {
	return strings.Trim(s, " \t\n\r\x00\x0B")
}

// Secret asks like Ask for an answer it does not show. It never fails:
// like Symfony, it then falls back to a plain read, which finds the input
// ended and answers empty.
func (t *Terminal) Secret(label string) string {
	_, _ = fmt.Fprintf(t.out, "\n %s:\n > ", label)
	line, err := t.readHiddenLine()
	if err != nil {
		line, _ = t.readLine()
		_, _ = io.WriteString(t.out, "\n")
		return phpTrim(line)
	}
	// The newline the hidden answer did not show, then the one after it.
	_, _ = io.WriteString(t.out, "\n\n")
	return phpTrim(line)
}
