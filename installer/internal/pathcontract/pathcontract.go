package pathcontract

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const ExpectedRelativePath = "dev/k-playbook"

type Code string

const (
	CodeOK                   Code = "OK"
	CodeMissing              Code = "MISSING"
	CodeExpectedNotDirectory Code = "EXPECTED_NOT_DIRECTORY"
	CodeExpectedNotRepo      Code = "EXPECTED_NOT_K_PLAYBOOK"
	CodeWrongTarget          Code = "WRONG_TARGET"
)

type Result struct {
	Code                  Code
	OK                    bool
	Expected              string
	Current               string
	CurrentIsRepo         bool
	ExpectedExists        bool
	ExpectedIsSymlink     bool
	ExpectedSymlinkTarget string
	ExpectedIsRepo        bool
	Fixable               bool
	FixCommand            string
	Message               string
}

func ExpectedPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home ermitteln: %w", err)
	}

	return filepath.Join(home, ExpectedRelativePath), nil
}

func Check() (Result, error) {
	expected, err := ExpectedPath()
	if err != nil {
		return Result{}, err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return Result{}, fmt.Errorf("current working directory ermitteln: %w", err)
	}

	current, currentIsRepo := DiscoverCurrentRoot(cwd)
	result := Result{
		Expected:      expected,
		Current:       current,
		CurrentIsRepo: currentIsRepo,
	}

	info, statErr := os.Lstat(expected)
	if statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			result.Code = CodeMissing
			result.Message = "Der verbindliche Pfad ~/dev/k-playbook existiert nicht."
			if currentIsRepo {
				result.Fixable = true
				result.FixCommand = fmt.Sprintf("ln -sfn %s %s", shellQuote(current), shellQuote(expected))
			}
			return result, nil
		}

		return Result{}, fmt.Errorf("%s pruefen: %w", expected, statErr)
	}

	result.ExpectedExists = true
	result.ExpectedIsSymlink = info.Mode()&os.ModeSymlink != 0

	if result.ExpectedIsSymlink {
		target, err := filepath.EvalSymlinks(expected)
		if err != nil {
			result.Code = CodeWrongTarget
			result.Message = fmt.Sprintf("Der Symlink %s kann nicht aufgeloest werden: %v", expected, err)
			return result, nil
		}

		result.ExpectedSymlinkTarget = target
		result.ExpectedIsRepo = IsKPlaybookRoot(target)

		if !result.ExpectedIsRepo {
			result.Code = CodeExpectedNotRepo
			result.Message = "Der Symlink ~/dev/k-playbook zeigt nicht auf ein k-playbook-Repo."
			return result, nil
		}

		if currentIsRepo && !samePath(current, target) {
			result.Code = CodeWrongTarget
			result.Message = "~/dev/k-playbook zeigt auf ein anderes k-playbook-Repo als das aktuell verwendete Repo."
			return result, nil
		}

		result.Code = CodeOK
		result.OK = true
		result.Message = "Pfadvertrag erfuellt: ~/dev/k-playbook ist ein Symlink auf dieses k-playbook-Repo."
		return result, nil
	}

	if !info.IsDir() {
		result.Code = CodeExpectedNotDirectory
		result.Message = "~/dev/k-playbook existiert, ist aber kein Verzeichnis und kein Symlink."
		return result, nil
	}

	result.ExpectedIsRepo = IsKPlaybookRoot(expected)
	if !result.ExpectedIsRepo {
		result.Code = CodeExpectedNotRepo
		result.Message = "~/dev/k-playbook existiert, sieht aber nicht wie ein k-playbook-Repo aus."
		return result, nil
	}

	if currentIsRepo && !samePath(current, expected) {
		result.Code = CodeWrongTarget
		result.Message = "~/dev/k-playbook ist ein anderes k-playbook-Repo als das aktuell verwendete Repo."
		return result, nil
	}

	result.Code = CodeOK
	result.OK = true
	result.Message = "Pfadvertrag erfuellt: dieses Repo ist unter ~/dev/k-playbook erreichbar."
	return result, nil
}

func Repair(result Result) error {
	if !result.Fixable {
		return errors.New("dieser Pfadvertrag-Status ist nicht automatisch reparierbar")
	}
	if result.Code != CodeMissing {
		return errors.New("automatische Reparatur ist nur erlaubt, wenn ~/dev/k-playbook fehlt")
	}
	if result.Current == "" || !result.CurrentIsRepo {
		return errors.New("aktuelles k-playbook-Repo konnte nicht sicher erkannt werden")
	}

	if err := os.MkdirAll(filepath.Dir(result.Expected), 0o755); err != nil {
		return fmt.Errorf("Zielverzeichnis anlegen: %w", err)
	}

	source := result.Current
	if realSource, err := filepath.EvalSymlinks(source); err == nil {
		source = realSource
	}

	if err := os.Symlink(source, result.Expected); err != nil {
		return fmt.Errorf("Symlink anlegen: %w", err)
	}

	return nil
}

func DiscoverCurrentRoot(start string) (string, bool) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}

	for {
		if IsKPlaybookRoot(abs) {
			return abs, true
		}

		parent := filepath.Dir(abs)
		if parent == abs {
			return "", false
		}
		abs = parent
	}
}

func IsKPlaybookRoot(path string) bool {
	markers := []string{
		"AGENTS.md",
		"README.md",
		filepath.Join("docs", "README.md"),
		filepath.Join("installer", "go.mod"),
	}

	for _, marker := range markers {
		info, err := os.Stat(filepath.Join(path, marker))
		if err != nil || info.IsDir() {
			return false
		}
	}

	commands, err := filepath.Glob(filepath.Join(path, "commands", "k-*.md"))
	if err != nil || len(commands) == 0 {
		return false
	}

	return true
}

func samePath(left string, right string) bool {
	leftReal, leftErr := filepath.EvalSymlinks(left)
	rightReal, rightErr := filepath.EvalSymlinks(right)
	if leftErr == nil {
		left = leftReal
	}
	if rightErr == nil {
		right = rightReal
	}

	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr == nil {
		left = leftAbs
	}
	if rightErr == nil {
		right = rightAbs
	}

	return filepath.Clean(left) == filepath.Clean(right)
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}

	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
