// Package console renders output the way the PHP CLI (Symfony Console)
// does.
package console

import (
	"io"
	"strings"
)

// ErrorBlock renders an error the way Symfony does, as a padded block
// (here without its colors).
func ErrorBlock(out io.Writer, message string) {
	blank := strings.Repeat(" ", len(message)+4)
	_, _ = io.WriteString(out, "\n"+blank+"\n  "+message+"  \n"+blank+"\n\n")
}
