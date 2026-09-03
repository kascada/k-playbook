//go:build darwin

package guiproc

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ProcessStartTime fragt ps nach der Startzeit: macOS hat kein /proc, und
// lstart liefert sie sekundengenau in der lokalen Zeitzone.
func ProcessStartTime(pid int) (time.Time, error) {
	output, err := exec.Command("ps", "-o", "lstart=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return time.Time{}, fmt.Errorf("ps -o lstart= -p %d: %w", pid, err)
	}
	return parseLstart(strings.TrimSpace(string(output)))
}

// parseLstart liest das Format von lstart, etwa "Tue Sep  3 15:20:46 2026".
func parseLstart(text string) (time.Time, error) {
	if text == "" {
		return time.Time{}, errors.New("ps: keine Startzeit")
	}
	start, err := time.ParseInLocation("Mon Jan _2 15:04:05 2006", text, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("ps: Startzeit %q: %w", text, err)
	}
	return start, nil
}
