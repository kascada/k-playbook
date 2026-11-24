package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Die Hilfe von k-playbook nennt jeden Unterbefehl von docs (Task 189). Die
// Liste steht nirgends abgeschrieben: Die Unterbefehle kommen aus dem Block
// „Aufruf:“ von k-playbook docs --help, und jeder Fall der Verteilung in
// runDocs muss dort stehen. Ein neuer Unterbefehl ohne Eintrag in printUsage
// macht den Test rot.
func TestUsageNenntAlleDocsUnterbefehle(t *testing.T) {
	var docsHelp bytes.Buffer
	printDocsUsage(&docsHelp)
	names := docsUsageSubcommands(t, docsHelp.String())
	if len(names) == 0 {
		t.Fatalf("kein Unterbefehl im Block Aufruf: von docs --help:\n%s", docsHelp.String())
	}

	for _, first := range docsDispatchCases(t) {
		found := false
		for _, name := range names {
			if strings.Fields(name)[0] == first {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("die Verteilung kennt docs %s, docs --help nennt es unter Aufruf: nicht", first)
		}
	}

	usage := usageText(t)
	start := strings.Index(usage, "\n  docs ")
	if start < 0 {
		t.Fatal("printUsage hat keinen Absatz docs")
	}
	paragraph := usage[start+1:]
	if next := regexp.MustCompile(`\n  \S`).FindStringIndex(paragraph); next != nil {
		paragraph = paragraph[:next[0]]
	}
	paragraph = strings.Join(strings.Fields(paragraph), " ")
	for _, name := range names {
		if !strings.Contains(paragraph, "k-playbook docs "+name) {
			t.Errorf("printUsage nennt im Absatz docs nicht: k-playbook docs %s\n%s", name, paragraph)
		}
	}
}

// docsUsageSubcommands liest die Unterbefehle aus dem Block „Aufruf:“ von
// docs --help: je Zeile die Wörter nach „k-playbook docs“ bis zum ersten
// Platzhalter, Schalter oder Hinweis — auch zweiteilige wie „order add“.
func docsUsageSubcommands(t *testing.T, help string) []string {
	t.Helper()
	_, block, ok := strings.Cut(help, "Aufruf:\n")
	if !ok {
		t.Fatalf("docs --help hat keinen Block Aufruf:\n%s", help)
	}
	block, _, _ = strings.Cut(block, "\n\n")
	seen := map[string]bool{}
	names := []string{}
	for _, line := range strings.Split(block, "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "k-playbook docs ")
		if !ok {
			continue
		}
		words := []string{}
		for _, word := range strings.Fields(rest) {
			if strings.ContainsAny(word[:1], "<[-(") {
				break
			}
			words = append(words, word)
		}
		if name := strings.Join(words, " "); name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return names
}

// docsDispatchCases liest aus docs.go die Fälle der Verteilung in runDocs
// (switch sub), ohne die Hilfe.
func docsDispatchCases(t *testing.T) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "docs.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	cases := []string{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "runDocs" {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			sw, ok := node.(*ast.SwitchStmt)
			if !ok {
				return true
			}
			if tag, ok := sw.Tag.(*ast.Ident); !ok || tag.Name != "sub" {
				return true
			}
			for _, stmt := range sw.Body.List {
				for _, expr := range stmt.(*ast.CaseClause).List {
					lit, ok := expr.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					value, err := strconv.Unquote(lit.Value)
					if err != nil || value == "-h" || value == "--help" || value == "help" {
						continue
					}
					cases = append(cases, value)
				}
			}
			return false
		})
	}
	if len(cases) == 0 {
		t.Fatal("keine Fälle der Verteilung in runDocs gefunden")
	}
	return cases
}
