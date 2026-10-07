package prompt

import "io"

// NewWithHiddenReader returns a Terminal that reads hidden answers with
// readHidden, as it does on a terminal.
func NewWithHiddenReader(in io.Reader, out io.Writer, readHidden func() ([]byte, error)) *Terminal {
	t := New(in, out)
	t.readHidden = readHidden
	return t
}

// NewTerminalForTest returns a Terminal that takes in for a terminal, without
// switching it to raw mode.
func NewTerminalForTest(in io.Reader, out io.Writer) *Terminal {
	t := New(in, out)
	t.raw = func() (func(), error) { return func() {}, nil }
	return t
}
