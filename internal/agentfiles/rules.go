package agentfiles

import (
	"os"
	"strings"

	"github.com/mrtheroi/memry-cli/internal/fsx"
)

// The markers around memry's block in an instructions file.
const (
	RulesStart = "<!-- memry:start -->"
	RulesEnd   = "<!-- memry:end -->"
)

// RulesFile is a Markdown instructions file of an agent, holding memry's
// rules in a marker-delimited block so the rest of the file is never
// touched.
type RulesFile struct {
	path string
}

// NewRulesFile returns the instructions file at path.
func NewRulesFile(path string) *RulesFile {
	return &RulesFile{path: path}
}

// Path is the file's path.
func (r *RulesFile) Path() string {
	return r.path
}

// Put writes rules as the memry block, creating the file and its
// directories when missing. It returns false, leaving the file untouched,
// when its markers do not delimit exactly one block.
func (r *RulesFile) Put(rules string) (bool, error) {
	contents, err := readOrEmpty(r.path)
	if err != nil {
		return false, err
	}
	block := RulesStart + "\n" + rules + "\n" + RulesEnd + "\n"
	start, end, found, broken := blockSpan(contents)
	if broken {
		return false, nil
	}
	if !found {
		return true, fsx.Replace(r.path, []byte(blankLineAfter(contents)+block))
	}
	return true, fsx.Replace(r.path, []byte(contents[:start]+block+contents[end:]))
}

// blockSpan returns where the memry block starts and ends, including the
// newline after its end marker, whether there is one, and whether the
// markers are broken: they do not delimit exactly one block.
func blockSpan(contents string) (start, end int, found, broken bool) {
	starts := strings.Count(contents, RulesStart)
	ends := strings.Count(contents, RulesEnd)
	if starts == 0 && ends == 0 {
		return 0, 0, false, false
	}
	start = strings.Index(contents, RulesStart)
	end = strings.Index(contents, RulesEnd)
	if starts != 1 || ends != 1 || end < start {
		return 0, 0, false, true
	}
	end += len(RulesEnd)
	if end < len(contents) && contents[end] == '\n' {
		end++
	}
	return start, end, true, false
}

// Remove removes the memry block and the blank line setting it apart,
// deleting the file when nothing else is left in it.
func (r *RulesFile) Remove() (Removal, error) {
	contents, err := readOrEmpty(r.path)
	if err != nil {
		return NotFound, err
	}
	start, end, found, broken := blockSpan(contents)
	if broken {
		return Unsafe, nil
	}
	if !found {
		return NotFound, nil
	}
	rest := blankLineJoin(contents[:start], contents[end:])
	// Nothing but the block means memry created the file.
	if phpTrim(rest) == "" {
		return Removed, os.Remove(r.path)
	}
	return Removed, fsx.Replace(r.path, []byte(rest))
}
