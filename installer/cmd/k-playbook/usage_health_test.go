package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// healthAreas liest die Bereiche der Umgebungsprüfung aus dem Paket health:
// jede Konstante Area… in internal/health/health.go. Die Liste steht damit in
// keinem Test abgeschrieben — ein neuer Bereich ohne Eintrag in den Hilfen
// macht die Tests unten rot.
func healthAreas(t *testing.T) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", "..", "internal", "health", "health.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var areas []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value := spec.(*ast.ValueSpec)
			for i, name := range value.Names {
				if !strings.HasPrefix(name.Name, "Area") || i >= len(value.Values) {
					continue
				}
				lit, ok := value.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					t.Fatalf("%s ist kein Zeichenkettenliteral", name.Name)
				}
				area, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatal(err)
				}
				areas = append(areas, area)
			}
		}
	}
	if len(areas) == 0 {
		t.Fatal("das Paket health nennt keinen Bereich")
	}
	sort.Strings(areas)
	return areas
}

// usageParagraph schneidet aus printUsage den Absatz eines Unterkommandos.
func usageParagraph(t *testing.T, name string) string {
	t.Helper()
	usage := usageText(t)
	start := strings.Index(usage, "\n  "+name+" ")
	if start < 0 {
		t.Fatalf("printUsage hat keinen Absatz %s", name)
	}
	paragraph := usage[start+1:]
	if next := regexp.MustCompile(`\n  \S`).FindStringIndex(paragraph); next != nil {
		paragraph = paragraph[:next[0]]
	}
	return paragraph
}

// areaWord findet einen Bereich als ganzes Wort — „github“ zählt nicht für
// „gh“.
func areaWord(text string, area string) bool {
	return regexp.MustCompile(`(^|[^A-Za-z])` + regexp.QuoteMeta(area) + `([^A-Za-z]|$)`).MatchString(text)
}

// Die drei Hilfen nennen genau die Bereiche, die eine Quelle melden kann. Ein
// Bereich gh steht in keiner mehr: Keine Quelle meldet ihn.
func TestHilfenNennenDieBereicheDerUmgebungspruefung(t *testing.T) {
	areas := healthAreas(t)

	var healthHelp bytes.Buffer
	printHealthUsage(&healthHelp)
	var briefingHelp bytes.Buffer
	printBriefingUsage(&briefingHelp)
	bullet := briefingHelp.String()
	start := strings.Index(bullet, "die Umgebungsprüfung:")
	if start < 0 {
		t.Fatalf("briefing --help hat keinen Punkt zur Umgebungsprüfung:\n%s", bullet)
	}
	bullet = bullet[start:]
	if end := strings.Index(bullet, "\n  - "); end >= 0 {
		bullet = bullet[:end]
	}

	texts := map[string]string{
		"printUsage, Absatz health":                  usageParagraph(t, "health"),
		"printBriefingUsage, Punkt Umgebungsprüfung": bullet,
		"printHealthUsage":                           healthHelp.String(),
	}
	for name, text := range texts {
		for _, area := range areas {
			if !areaWord(text, area) {
				t.Errorf("%s nennt den Bereich %s nicht:\n%s", name, area, text)
			}
		}
	}
	for _, name := range []string{"printUsage, Absatz health", "printBriefingUsage, Punkt Umgebungsprüfung"} {
		if areaWord(texts[name], "gh") {
			t.Errorf("%s nennt einen Bereich gh, den keine Quelle meldet:\n%s", name, texts[name])
		}
	}

	// Die Tabelle von health --help trägt je Bereich genau eine Zeile, und
	// keine für einen Bereich, den es nicht gibt.
	var listed []string
	for _, match := range regexp.MustCompile(`(?m)^  ([A-Za-z]+) {2,}\S`).FindAllStringSubmatch(healthHelp.String(), -1) {
		listed = append(listed, match[1])
	}
	sort.Strings(listed)
	if strings.Join(listed, " ") != strings.Join(areas, " ") {
		t.Errorf("health --help listet die Bereiche %v, das Paket health nennt %v", listed, areas)
	}
}

// Eine nicht eingetragene Anbindung meldet health als error
// (healthKephalaion); die Hilfe nannte dafür warning.
func TestHealthHilfeNenntNichtEingetrageneAnbindungAlsError(t *testing.T) {
	var help bytes.Buffer
	printHealthUsage(&help)
	text := strings.Join(strings.Fields(help.String()), " ")
	if strings.Contains(text, "nicht eingetragen: warning") {
		t.Errorf("health --help nennt die nicht eingetragene Anbindung als warning:\n%s", help.String())
	}
	if !strings.Contains(text, "jede nicht bereite Anbindung: error, auch eine nicht eingetragene") {
		t.Errorf("health --help nennt die Stufe der nicht eingetragenen Anbindung nicht:\n%s", help.String())
	}
}

// k-playbook todo kennt list, add, update und delete; ein import gibt es
// nicht, und die Hilfe nennt keinen.
func TestUsageNenntKeinTodoImport(t *testing.T) {
	paragraph := usageParagraph(t, "todo")
	if strings.Contains(paragraph, "import") {
		t.Errorf("printUsage nennt todo import:\n%s", paragraph)
	}
	if !strings.Contains(paragraph, "k-playbook todo list|add|update|delete.") {
		t.Errorf("printUsage nennt die Unterbefehle von todo nicht:\n%s", paragraph)
	}
}

// Keine Meldung des Programms empfiehlt „k-playbook gui“ als Start der
// Oberfläche: Der Aufruf ohne Unterkommando endet mit einem Fehler, gestartet
// wird sie mit dem argumentlosen Aufruf oder über /k-gui. Geprüft wird jedes
// Zeichenkettenliteral der Go-Quellen des Programms (ohne Tests); zulässig
// bleibt „k-playbook gui status“ und die Titelzeile der Hilfe des
// Unterkommandos selbst.
func TestKeineMeldungEmpfiehltGuiAlsStart(t *testing.T) {
	root := filepath.Join("..", "..")
	const needle = "k-playbook gui"
	checked := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		checked++
		ast.Inspect(file, func(node ast.Node) bool {
			lit, ok := node.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			text, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			for rest := text; ; {
				at := strings.Index(rest, needle)
				if at < 0 {
					break
				}
				after := rest[at+len(needle):]
				title := at == 0 && len(rest) == len(text) && strings.HasPrefix(after, "\n")
				if !strings.HasPrefix(after, " status") && !title {
					t.Errorf("%s nennt „%s“ ohne status: %q", fset.Position(lit.Pos()), needle, text)
				}
				rest = after
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("keine Go-Quelle geprüft")
	}
}
