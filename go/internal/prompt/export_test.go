package prompt

import "io"

// NewWithHiddenReader returns a Terminal that reads hidden answers with
// readHidden, as it does on a terminal.
func NewWithHiddenReader(in io.Reader, out io.Writer, readHidden func() ([]byte, error)) *Terminal {
	t := New(in, out)
	t.readHidden = readHidden
	return t
}
