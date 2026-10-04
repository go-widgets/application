package application

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

// TestEveryCapabilityIsNamedInThePackageDoc.
//
// ⛔ A capability list that is wrong is worse than no list: a reader who counts
// six does not go looking for a seventh. This one had drifted THREE behind --
// ModifiedClicker and ModifiedKeyer were added without it, and
// NativeControlProvider had been missing for longer.
//
// So the list is measured rather than maintained. "A capability" is not a
// judgement here: it is an exported interface this package type-asserts off a
// Handler, which is exactly what makes it optional, and the walk below finds
// those in the source rather than from a list someone has to remember to
// update.
func TestEveryCapabilityIsNamedInThePackageDoc(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parsing this package: %v", err)
	}
	pkg, ok := pkgs["application"]
	if !ok {
		t.Fatal("this package did not parse as application")
	}

	interfaces := map[string]bool{}
	asserted := map[string]bool{}
	var doc string
	for name, f := range pkg.Files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		if f.Doc != nil && strings.Contains(f.Doc.Text(), "optional capability interfaces") {
			doc = f.Doc.Text()
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.TypeSpec:
				if _, isIface := v.Type.(*ast.InterfaceType); isIface && v.Name.IsExported() {
					interfaces[v.Name.Name] = true
				}
			case *ast.TypeAssertExpr:
				// h.(Name): the Name is what makes the capability optional.
				if id, ok := v.Type.(*ast.Ident); ok && id.IsExported() {
					asserted[id.Name] = true
				}
			}
			return true
		})
	}
	if doc == "" {
		t.Fatal("no file in this package carries the doc naming the capability interfaces")
	}
	if len(interfaces) == 0 || len(asserted) == 0 {
		t.Fatalf("the walk found %d exported interfaces and %d type assertions; "+
			"a guard that can see nothing reports no drift",
			len(interfaces), len(asserted))
	}

	var capabilities []string
	for name := range asserted {
		if interfaces[name] {
			capabilities = append(capabilities, name)
		}
	}
	sort.Strings(capabilities)
	if len(capabilities) < 5 {
		t.Fatalf("only %d capabilities found (%v); the walk is not seeing the code",
			len(capabilities), capabilities)
	}

	// ⛔ The README carries the SAME list, and it had drifted the same three
	// behind. One source checked and the other left to rot is how a reader ends
	// up with two lists that disagree and no way to tell which is current.
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("reading README.md: %v", err)
	}
	for _, where := range []struct {
		name, text string
	}{
		{"the package doc", doc},
		{"README.md", string(readme)},
	} {
		list, ok := capabilityParagraph(where.text)
		if !ok {
			t.Errorf("%s has no paragraph naming the capability interfaces", where.name)
			continue
		}
		var missing []string
		for _, name := range capabilities {
			if !strings.Contains(list, name) {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			t.Errorf("%s does not name %v where it lists the capabilities.\nEach is "+
				"asserted off a Handler, so a handler may implement it and both lists "+
				"must say so.\nAll %d found: %v\nThe paragraph read:\n%s",
				where.name, missing, len(capabilities), capabilities, list)
		}
	}
}

// capabilityParagraph is the one paragraph that LISTS the capabilities.
//
// ⛔ Not the whole text. The README names ModifiedKeyer twice -- once in the
// list and once in a paragraph explaining why it exists -- so a search over the
// file lets the LIST lose an entry while the prose keeps the check quiet. That
// is what the first version of this guard did, and an ablation that removed a
// name from the list passed.
func capabilityParagraph(text string) (string, bool) {
	for _, para := range strings.Split(text, "\n\n") {
		if strings.Contains(para, "capability interfaces") {
			return para, true
		}
	}
	return "", false
}
