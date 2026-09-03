//go:build linux

package guiproc

import (
	"os"
	"strings"
	"testing"
	"time"
)

// comm darf Leerzeichen und Klammern enthalten; das Parsen setzt hinter der
// letzten schließenden Klammer auf.
func TestParseStartTicks(t *testing.T) {
	// Felder 3 bis 52 einer echten stat-Zeile; Feld 22 ist starttime.
	rest := []string{"S", "1", "2", "3", "0", "-1", "4194304", "529", "0", "0", "0", "0", "0", "0", "0",
		"20", "0", "1", "0", "37347560", "16637952", "1808"}
	tests := []struct {
		name string
		stat string
		want uint64
	}{
		{name: "einfach", stat: "1234 (k-playbook) " + strings.Join(rest, " "), want: 37347560},
		{name: "comm mit Leerzeichen und Klammern", stat: "1234 (my (odd) name) " + strings.Join(rest, " "), want: 37347560},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseStartTicks(test.stat)
			if err != nil {
				t.Fatalf("Fehler: %v", err)
			}
			if got != test.want {
				t.Errorf("starttime = %d, erwartet %d", got, test.want)
			}
		})
	}

	if _, err := parseStartTicks("1234 (kurz) S 1"); err == nil {
		t.Error("zu wenige Felder, aber kein Fehler")
	}
	if _, err := parseStartTicks("kaputt"); err == nil {
		t.Error("ohne comm, aber kein Fehler")
	}
}

// Die Startzeit des eigenen Prozesses liegt in der Vergangenheit, aber nicht
// weit — das Test-Binary ist eben erst gestartet.
func TestProcessStartTimeEigenerProzess(t *testing.T) {
	start, err := ProcessStartTime(os.Getpid())
	if err != nil {
		t.Fatalf("Fehler: %v", err)
	}
	age := time.Since(start)
	if age < 0 || age > time.Hour {
		t.Errorf("Startzeit %s liegt %s zurück", start, age)
	}
}
