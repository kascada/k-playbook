//go:build linux

package guiproc

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// clockTicksPerSecond ist USER_HZ, die Einheit von starttime in
// /proc/<pid>/stat. Auf allen gängigen Linux-Ports 100.
const clockTicksPerSecond = 100

// ProcessStartTime liest die Startzeit aus /proc/<pid>/stat (Feld 22,
// Clock-Ticks seit dem Systemstart) und rechnet sie über btime aus /proc/stat
// in eine absolute Zeit um.
func ProcessStartTime(pid int) (time.Time, error) {
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return time.Time{}, err
	}
	ticks, err := parseStartTicks(string(stat))
	if err != nil {
		return time.Time{}, fmt.Errorf("/proc/%d/stat: %w", pid, err)
	}
	boot, err := bootTime()
	if err != nil {
		return time.Time{}, err
	}
	return boot.Add(time.Duration(ticks) * time.Second / clockTicksPerSecond), nil
}

// parseStartTicks holt Feld 22 aus einer stat-Zeile. Feld 2 (comm) steht in
// Klammern und darf Leerzeichen und Klammern enthalten, deshalb wird hinter
// der letzten schließenden Klammer aufgesetzt: dort beginnt Feld 3.
func parseStartTicks(stat string) (uint64, error) {
	end := strings.LastIndex(stat, ")")
	if end < 0 {
		return 0, errors.New("kein comm-Feld")
	}
	fields := strings.Fields(stat[end+1:])
	// fields[0] ist Feld 3; Feld 22 liegt damit bei Index 19.
	const index = 22 - 3
	if len(fields) <= index {
		return 0, fmt.Errorf("nur %d Felder", len(fields)+2)
	}
	return strconv.ParseUint(fields[index], 10, 64)
}

// bootTime liest btime aus /proc/stat: die Boot-Zeit in Unix-Sekunden.
func bootTime() (time.Time, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return time.Time{}, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[0] == "btime" {
			seconds, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				return time.Time{}, fmt.Errorf("/proc/stat: btime: %w", err)
			}
			return time.Unix(seconds, 0), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return time.Time{}, err
	}
	return time.Time{}, errors.New("/proc/stat: kein btime")
}
