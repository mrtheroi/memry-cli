package prompt_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/prompt"
)

// Symfony's question helper prints the question in a block and reads one
// line, trimmed.
func TestAskPrintsTheQuestionAndReadsATrimmedLine(t *testing.T) {
	var out bytes.Buffer
	terminal := prompt.New(strings.NewReader("  Ana@Example.com \nnext\n"), &out)

	answer, err := terminal.Ask("Email")
	if err != nil || answer != "Ana@Example.com" {
		t.Errorf("Ask = %q, %v; want Ana@Example.com, nil", answer, err)
	}
	if want := "\n Email:\n > \n"; out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
}

// Symfony aborts when the input ends before an answer, unless an earlier
// read already reached the end: then the answer is empty, as when the last
// line has no newline.
func TestAskAbortsAtTheEndOfTheInput(t *testing.T) {
	terminal := prompt.New(strings.NewReader("first\n"), &bytes.Buffer{})
	if _, err := terminal.Ask("Email"); err != nil {
		t.Fatal(err)
	}
	if answer, err := terminal.Ask("Login code"); !errors.Is(err, prompt.ErrAborted) {
		t.Errorf("Ask at the end = %q, %v; want ErrAborted", answer, err)
	}

	terminal = prompt.New(strings.NewReader("ana@example.com"), &bytes.Buffer{})
	if answer, err := terminal.Ask("Email"); answer != "ana@example.com" || err != nil {
		t.Fatalf("Ask = %q, %v; want ana@example.com, nil", answer, err)
	}
	for range 2 {
		if answer, err := terminal.Ask("Login code"); answer != "" || err != nil {
			t.Errorf("Ask after the end = %q, %v; want an empty answer", answer, err)
		}
	}
}

// Ported from "asks for the token when --token is given without a value":
// the hidden answer is read and trimmed, never printed. Symfony prints a
// newline for the hidden answer, then the one after every answer.
func TestSecretReadsAnAnswerWithoutPrintingIt(t *testing.T) {
	var out bytes.Buffer
	terminal := prompt.New(strings.NewReader(" admin-token \n"), &out)

	if answer := terminal.Secret("Token"); answer != "admin-token" {
		t.Errorf("Secret = %q, want admin-token", answer)
	}
	if want := "\n Token:\n > \n\n"; out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
}

// Symfony falls back to a plain read when the hidden one aborts, which then
// finds the input ended: the answer is empty, never an abort.
func TestSecretIsEmptyAtTheEndOfTheInput(t *testing.T) {
	var out bytes.Buffer
	if answer := prompt.New(strings.NewReader(""), &out).Secret("Token"); answer != "" {
		t.Errorf("Secret = %q, want an empty answer", answer)
	}
	if want := "\n Token:\n > \n"; out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
}

// On a terminal the hidden answer is read with echo off (golang.org/x/term),
// never from the plain line reader.
func TestSecretReadsWithEchoOffOnATerminal(t *testing.T) {
	var out bytes.Buffer
	terminal := prompt.NewWithHiddenReader(strings.NewReader("visible\n"), &out, func() ([]byte, error) {
		return []byte(" admin-token "), nil
	})

	if answer := terminal.Secret("Token"); answer != "admin-token" {
		t.Errorf("Secret = %q, want admin-token", answer)
	}
	if want := "\n Token:\n > \n\n"; out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
	if answer, _ := terminal.Ask("Email"); answer != "visible" {
		t.Errorf("Ask after Secret = %q, want the line Secret left: visible", answer)
	}
}

// Symfony's confirmation question: the default (no) is shown, and only an
// answer starting with y (in any case) confirms.
func TestConfirmAsksAYesNoQuestionDefaultingToNo(t *testing.T) {
	answers := map[string]bool{"yes\n": true, "y\n": true, " Yes \n": true, "YES\n": true, "no\n": false, "\n": false, "nope\n": false, "sure\n": false}
	for input, want := range answers {
		var out bytes.Buffer
		confirmed, err := prompt.New(strings.NewReader(input), &out).Confirm("Remove memry?")

		if err != nil || confirmed != want {
			t.Errorf("Confirm(%q) = %v, %v; want %v", input, confirmed, err, want)
		}
		if want := "\n Remove memry? (yes/no) [no]:\n > \n"; out.String() != want {
			t.Errorf("output = %q, want %q", out.String(), want)
		}
	}
	if _, err := prompt.New(strings.NewReader(""), &bytes.Buffer{}).Confirm("Remove memry?"); !errors.Is(err, prompt.ErrAborted) {
		t.Errorf("Confirm at the end of the input = %v, want ErrAborted", err)
	}
}
