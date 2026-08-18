package tui

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const internalPrefix = "github.com/kido5217/yolo/internal/"

// TestImportsDirection guards TUI purity (AGENTS.md core principle 4):
// non-test files under internal/tui/ import only internal/protocol and
// internal/tui/* from within the module; _test.go files may additionally
// use the test escape hatches (internal/server/testutil for the real-stack
// blackbox suites, internal/llm{,/fake} for scripted fake turns).
func TestImportsDirection(t *testing.T) {
	var goFiles []string
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		goFiles = append(goFiles, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(goFiles) == 0 {
		t.Fatal("no .go files found under internal/tui")
	}
	fset := token.NewFileSet()
	for _, path := range goFiles {
		isTest := strings.HasSuffix(path, "_test.go")
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, imp := range f.Imports {
			p, uerr := strconv.Unquote(imp.Path.Value)
			if uerr != nil {
				t.Fatalf("%s: unquote import path %s: %v", path, imp.Path.Value, uerr)
			}
			if !strings.HasPrefix(p, internalPrefix) {
				continue
			}
			if importAllowed(path, p) {
				continue
			}
			rule := "non-test files: internal/protocol + internal/tui/*"
			if isTest {
				rule = "test files: internal/protocol + internal/tui/* + server/testutil + llm{,/fake}"
			}
			t.Errorf("%s imports %q (rule: %s)", path, p, rule)
		}
	}
}

func importAllowed(path, imp string) bool {
	switch imp {
	case "github.com/kido5217/yolo/internal/protocol":
		return true
	}
	if strings.HasPrefix(imp, "github.com/kido5217/yolo/internal/tui") {
		return true
	}
	return strings.HasSuffix(path, "_test.go") && (imp == "github.com/kido5217/yolo/internal/server/testutil" ||
		imp == "github.com/kido5217/yolo/internal/llm" ||
		imp == "github.com/kido5217/yolo/internal/llm/fake")
}
