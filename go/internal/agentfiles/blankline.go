package agentfiles

import "strings"

// The blank line that sets a block memry adds to a file apart from the
// rest of it, added when the block goes in and dropped when it comes out,
// like the PHP CLI's BlankLine.

// blankLineAfter returns contents ending with a blank line unless they are
// empty, ready for a block to be appended.
func blankLineAfter(contents string) string {
	switch {
	case contents == "", strings.HasSuffix(contents, "\n\n"):
		return contents
	case strings.HasSuffix(contents, "\n"):
		return contents + "\n"
	}
	return contents + "\n\n"
}

// blankLineJoin joins the contents before and after a removed block,
// without the blank line that set it apart.
func blankLineJoin(before, after string) string {
	switch {
	case strings.HasSuffix(before, "\n\n") && after == "":
		return before[:len(before)-1]
	case strings.HasSuffix(before, "\n\n") && strings.HasPrefix(after, "\n"):
		return before + after[1:]
	}
	return before + after
}
