package health

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

// withSources ersetzt die registrierten Quellen für einen Test.
func withSources(t *testing.T, list ...Source) {
	t.Helper()
	sourcesMu.Lock()
	original := sources
	sources = list
	sourcesMu.Unlock()
	t.Cleanup(func() {
		sourcesMu.Lock()
		sources = original
		sourcesMu.Unlock()
	})
}

func fixed(problems ...Problem) Source {
	return func(context.Context, string) []Problem { return problems }
}

func TestCheckOhneProblemeIstLeerUndOK(t *testing.T) {
	withSources(t, fixed(), func(context.Context, string) []Problem { return nil })

	report := Check(context.Background(), "/projekt")
	if !report.OK {
		t.Error("OK ist false ohne Probleme")
	}
	if report.Problems == nil || len(report.Problems) != 0 {
		t.Errorf("Problems = %#v, erwartet leere Liste", report.Problems)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"ok":true,"problems":[]}` {
		t.Errorf("JSON: %s", encoded)
	}
}

func TestCheckOhneQuellenIstLeer(t *testing.T) {
	withSources(t)

	report := Check(context.Background(), "/projekt")
	if !report.OK || len(report.Problems) != 0 {
		t.Errorf("Report ohne Quellen: %+v", report)
	}
}

// Warnungen lassen OK wahr; erst ein Fehler macht es falsch. Check bricht
// nicht beim ersten Problem ab: alle Quellen stehen im Report.
func TestCheckSammeltAlleUndWertetStufen(t *testing.T) {
	warn := Problem{Level: LevelWarning, Area: AreaBaseTools, Message: "rg fehlt.", Hint: "installieren"}
	withSources(t, fixed(warn))
	if report := Check(context.Background(), "/p"); !report.OK || len(report.Problems) != 1 {
		t.Errorf("nur Warnung: %+v", report)
	}

	fail := Problem{Level: LevelError, Area: AreaVersionSources, Message: "unlesbar.", Hint: "korrigieren"}
	withSources(t, fixed(warn), fixed(fail))
	report := Check(context.Background(), "/p")
	if report.OK {
		t.Error("OK ist true trotz Fehler")
	}
	if len(report.Problems) != 2 {
		t.Fatalf("Problems = %+v", report.Problems)
	}
}

// Die Reihenfolge ist stabil: Bereich, dann Stufe (error vor warning), dann
// Satz — unabhängig davon, in welcher Reihenfolge die Quellen liefern.
func TestCheckSortiertNachBereichDannStufe(t *testing.T) {
	ghWarn := Problem{Level: LevelWarning, Area: AreaGH, Message: "gh fehlt."}
	kephWarn := Problem{Level: LevelWarning, Area: AreaKephalaion, Message: "nicht eingetragen."}
	kephErr := Problem{Level: LevelError, Area: AreaKephalaion, Message: "Node antwortet nicht."}
	toolB := Problem{Level: LevelWarning, Area: AreaBaseTools, Message: "b fehlt."}
	toolA := Problem{Level: LevelWarning, Area: AreaBaseTools, Message: "a fehlt."}

	withSources(t, fixed(kephWarn, ghWarn), fixed(toolB, kephErr, toolA))
	got := Check(context.Background(), "/p").Problems
	want := []Problem{toolA, toolB, ghWarn, kephErr, kephWarn}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Reihenfolge:\n%+v\nerwartet\n%+v", got, want)
	}

	withSources(t, fixed(toolA, kephErr, toolB), fixed(ghWarn, kephWarn))
	if again := Check(context.Background(), "/p").Problems; !reflect.DeepEqual(again, want) {
		t.Errorf("Reihenfolge hängt von den Quellen ab:\n%+v", again)
	}
}

func TestRegisterIgnoriertNil(t *testing.T) {
	withSources(t)
	Register(nil)
	Register(fixed(Problem{Level: LevelWarning, Area: "x", Message: "m"}))
	if report := Check(context.Background(), "/p"); len(report.Problems) != 1 {
		t.Errorf("Problems = %+v", report.Problems)
	}
}

// Die Quellen bekommen ctx und projectDir unverändert.
func TestCheckReichtKontextUndVerzeichnisDurch(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "wert")
	var gotDir string
	var gotCtx context.Context
	withSources(t, func(c context.Context, dir string) []Problem {
		gotCtx, gotDir = c, dir
		return nil
	})
	Check(ctx, "/projekt")
	if gotDir != "/projekt" || gotCtx == nil || gotCtx.Value(key{}) != "wert" {
		t.Errorf("ctx/projectDir nicht durchgereicht: %v %q", gotCtx, gotDir)
	}
}
