package payload

import (
	"io/fs"
	"testing"
)

func TestPayloadComplete(t *testing.T) {
	if v := Version(); v != "0.4.0" {
		t.Errorf("Version = %q", v)
	}
	want := []string{
		"commands/k-review.md",
		"commands/_shared/path-resolution.md",
		"commands/_shared/overlay-resolution.md",
		"commands/_details/k-status.md",
		"skills/enforcement/SKILL.md",
		"rules/docs-sync.md",
		"reviews/review-tech.md",
		"checks/check_no_obvious_secrets.sh",
		"checks/lib/common.py",
		"scripts/install-security-tools.sh",
		"bin/k-check",
		"security-tools.tsv",
	}
	for _, name := range want {
		if _, err := fs.Stat(FS(), name); err != nil {
			t.Errorf("fehlt im Payload: %s", name)
		}
	}
	n := 0
	fs.WalkDir(FS(), ".", func(_ string, e fs.DirEntry, _ error) error {
		if e != nil && !e.IsDir() {
			n++
		}
		return nil
	})
	t.Logf("%d Dateien eingebettet", n)
}
