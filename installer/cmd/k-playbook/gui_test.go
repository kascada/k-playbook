package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
)

// Die Entscheidung des argumentlosen Einstiegs gegen alle fünf Ergebnisse der
// Einordnung, mit Attrappen statt Server und Browser.
func TestReuseOrStartEntscheidetNachStatus(t *testing.T) {
	record := guiproc.Record{Key: "/p", Addr: "127.0.0.1:1", PID: 7, Version: "v1", StartTime: 1}
	const path = "/run/k-playbook/abc.json"

	tests := []struct {
		name         string
		status       guiproc.Status
		stopSucceeds bool
		wantOpen     bool
		wantStop     bool
		wantDiscard  bool
		wantStart    bool
		wantErr      bool
	}{
		{name: "läuft: nur öffnen", status: guiproc.StatusRunning, wantOpen: true},
		{name: "andere Version: beenden, dann starten", status: guiproc.StatusOtherVersion, stopSucceeds: true, wantStop: true, wantStart: true},
		{name: "andere Version, endet nicht: nichts starten", status: guiproc.StatusOtherVersion, wantStop: true, wantErr: true},
		{name: "verwaist: Datei weg, dann starten", status: guiproc.StatusOrphaned, wantDiscard: true, wantStart: true},
		{name: "lebt ohne Antwort: nichts starten", status: guiproc.StatusUnresponsive, wantErr: true},
		{name: "nicht vorhanden: starten", status: guiproc.StatusAbsent, wantStart: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var opened, stopped, discarded, started bool
			var out bytes.Buffer
			actions := guiActions{
				open: func(guiproc.Record) { opened = true },
				stop: func(guiproc.Finding) bool {
					stopped = true
					return test.stopSucceeds
				},
				discard: func(string) error {
					discarded = true
					return nil
				},
				start: func() error {
					started = true
					return nil
				},
				out: &out,
			}

			err := reuseOrStart(guiproc.Finding{Status: test.status, Path: path, Record: record}, actions)
			if (err != nil) != test.wantErr {
				t.Fatalf("Fehler = %v, erwartet Fehler: %v", err, test.wantErr)
			}
			if opened != test.wantOpen || stopped != test.wantStop || discarded != test.wantDiscard || started != test.wantStart {
				t.Errorf("open=%v stop=%v discard=%v start=%v; erwartet open=%v stop=%v discard=%v start=%v",
					opened, stopped, discarded, started, test.wantOpen, test.wantStop, test.wantDiscard, test.wantStart)
			}
			if test.wantErr {
				if !strings.Contains(err.Error(), "k-playbook stop") || !strings.Contains(err.Error(), path) {
					t.Errorf("die Meldung nennt weder den Weg noch die Datei: %v", err)
				}
			}
		})
	}
}

// Kann die verwaiste Datei nicht weg, wird nicht gestartet: der nächste
// Aufruf fände sonst wieder dieselbe Datei.
func TestReuseOrStartBrichtBeiFehlendemDiscardAb(t *testing.T) {
	started := false
	err := reuseOrStart(guiproc.Finding{Status: guiproc.StatusOrphaned, Path: "/x.json"}, guiActions{
		discard: func(string) error { return errors.New("read-only") },
		start: func() error {
			started = true
			return nil
		},
		out: &bytes.Buffer{},
	})
	if err == nil || started {
		t.Errorf("Fehler = %v, gestartet = %v", err, started)
	}
}
