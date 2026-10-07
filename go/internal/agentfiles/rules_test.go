package agentfiles_test

import (
	"path/filepath"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/agentfiles"
)

// Ported from tests/Unit/RulesFileTest.php.

const rulesBlock = "<!-- memry:start -->\nUse memry.\n<!-- memry:end -->\n"

func rulesPath(t *testing.T) string {
	return filepath.Join(t.TempDir(), "agent", "AGENTS.md")
}

func put(t *testing.T, rules *agentfiles.RulesFile) bool {
	t.Helper()
	ok, err := rules.Put("Use memry.")
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	return ok
}

func remove(t *testing.T, rules *agentfiles.RulesFile) agentfiles.Removal {
	t.Helper()
	removed, err := rules.Remove()
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	return removed
}

func TestRulesFileCreatesTheFileAndItsDirectoriesWithOnlyTheMemryBlock(t *testing.T) {
	path := rulesPath(t)

	if !put(t, agentfiles.NewRulesFile(path)) {
		t.Error("Put = false, want true")
	}
	if got := readFile(t, path); got != rulesBlock {
		t.Errorf("contents = %q, want %q", got, rulesBlock)
	}
}

func TestRulesFileAppendsTheBlockAfterTheExistingRulesKeepingThem(t *testing.T) {
	tests := map[string]struct{ existing, separator string }{
		"ending with a newline":   {"# Rules\n\nBe concise. Diseño ✓\n", "\n"},
		"without a final newline": {"# Rules", "\n\n"},
		"ending with blank lines": {"# Rules\n\n\n", ""},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			path := rulesPath(t)
			writeFile(t, path, tt.existing)

			put(t, agentfiles.NewRulesFile(path))

			if got, want := readFile(t, path), tt.existing+tt.separator+rulesBlock; got != want {
				t.Errorf("contents = %q, want %q", got, want)
			}
		})
	}
}

func TestRulesFileReplacesAnExistingBlockInPlaceKeepingTheRulesAroundIt(t *testing.T) {
	path := rulesPath(t)
	writeFile(t, path, "# Before\n\n<!-- memry:start -->\nOld rules.\nMore.\n<!-- memry:end -->\n\n# After\n")

	put(t, agentfiles.NewRulesFile(path))

	if got, want := readFile(t, path), "# Before\n\n"+rulesBlock+"\n# After\n"; got != want {
		t.Errorf("contents = %q, want %q", got, want)
	}
}

func TestRulesFileLeavesExactlyTheSameFileAfterWritingTheSameBlockTwice(t *testing.T) {
	path := rulesPath(t)
	rules := agentfiles.NewRulesFile(path)
	put(t, rules)
	put(t, rules)

	if got := readFile(t, path); got != rulesBlock {
		t.Errorf("contents = %q, want %q", got, rulesBlock)
	}
}

func TestRulesFileLeavesTheFileUntouchedWhenItsMarkersAreBroken(t *testing.T) {
	tests := map[string]string{
		"start without end": "# Rules\n<!-- memry:start -->\nOld.\n",
		"end without start": "# Rules\nOld.\n<!-- memry:end -->\n",
		"end before start":  "<!-- memry:end -->\n<!-- memry:start -->\n",
		"two blocks":        "<!-- memry:start -->\nA\n<!-- memry:end -->\n<!-- memry:start -->\nB\n<!-- memry:end -->\n",
	}
	for name, contents := range tests {
		t.Run(name, func(t *testing.T) {
			path := rulesPath(t)
			writeFile(t, path, contents)

			if put(t, agentfiles.NewRulesFile(path)) {
				t.Error("Put = true, want false")
			}
			if got := readFile(t, path); got != contents {
				t.Errorf("contents = %q, want %q", got, contents)
			}
		})
	}
}

func TestRulesFileRemovesOnlyTheBlockAndTheBlankLineBeforeIt(t *testing.T) {
	tests := map[string]struct{ existing, want string }{
		"ending with a newline":   {"# Rules\n\nBe concise. Diseño ✓\n", "# Rules\n\nBe concise. Diseño ✓\n"},
		"without a final newline": {"# Rules", "# Rules\n"},
		"ending with blank lines": {"# Rules\n\n\n", "# Rules\n\n"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			path := rulesPath(t)
			writeFile(t, path, tt.existing)
			rules := agentfiles.NewRulesFile(path)
			put(t, rules)

			if got := remove(t, rules); got != agentfiles.Removed {
				t.Errorf("Remove = %v, want Removed", got)
			}
			if got := readFile(t, path); got != tt.want {
				t.Errorf("contents = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRulesFileRemovesABlockBetweenOtherRulesWithoutLeavingADoubleBlankLine(t *testing.T) {
	path := rulesPath(t)
	writeFile(t, path, "# Before\n\n"+rulesBlock+"\n# After\n")

	remove(t, agentfiles.NewRulesFile(path))

	if got, want := readFile(t, path), "# Before\n\n# After\n"; got != want {
		t.Errorf("contents = %q, want %q", got, want)
	}
}

func TestRulesFileDeletesTheFileWhenOnlyTheBlockWasInIt(t *testing.T) {
	path := rulesPath(t)
	rules := agentfiles.NewRulesFile(path)
	put(t, rules)

	if got := remove(t, rules); got != agentfiles.Removed {
		t.Errorf("Remove = %v, want Removed", got)
	}
	if exists(t, path) {
		t.Error("the file still exists")
	}
}

func TestRulesFileRemovesNothingWhenThereIsNoBlockOrNoFile(t *testing.T) {
	t.Run("no file", func(t *testing.T) {
		path := rulesPath(t)

		if got := remove(t, agentfiles.NewRulesFile(path)); got != agentfiles.NotFound {
			t.Errorf("Remove = %v, want NotFound", got)
		}
		if exists(t, path) {
			t.Error("the file was created")
		}
	})
	t.Run("no block", func(t *testing.T) {
		path := rulesPath(t)
		writeFile(t, path, "# Rules\n")

		if got := remove(t, agentfiles.NewRulesFile(path)); got != agentfiles.NotFound {
			t.Errorf("Remove = %v, want NotFound", got)
		}
		if got := readFile(t, path); got != "# Rules\n" {
			t.Errorf("contents = %q, want it untouched", got)
		}
	})
}

func TestRulesFileLeavesTheFileUntouchedWhenRemovingWithBrokenMarkers(t *testing.T) {
	path := rulesPath(t)
	contents := "# Rules\n<!-- memry:start -->\nOld.\n"
	writeFile(t, path, contents)

	if got := remove(t, agentfiles.NewRulesFile(path)); got != agentfiles.Unsafe {
		t.Errorf("Remove = %v, want Unsafe", got)
	}
	if got := readFile(t, path); got != contents {
		t.Errorf("contents = %q, want %q", got, contents)
	}
}

func TestRulesFileReplacesTheFileAtomicallyKeepingItsPermissions(t *testing.T) {
	for _, method := range []string{"put", "remove"} {
		t.Run(method, func(t *testing.T) {
			path := rulesPath(t)
			contents := "# Rules\n"
			if method == "remove" {
				contents += "\n" + rulesBlock
			}
			writeFile(t, path, contents)
			before := chmod0640(t, path)

			rules := agentfiles.NewRulesFile(path)
			if method == "put" {
				put(t, rules)
			} else {
				remove(t, rules)
			}

			assertReplacedKeepingMode(t, path, before)
		})
	}
}
