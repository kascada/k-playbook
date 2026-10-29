package kephalaion

import "os/exec"

// BinaryName ist der Programmname von Kephalaion im PATH.
const BinaryName = "kephalaion"

// InstallCommand ist die Installation aus dem README von Kephalaion: das
// install.sh des neuesten Releases. Gebraucht wird er dort, wo es keinen Clone
// mit bin/install-kephalaion gibt.
const InstallCommand = "curl -fsSL https://github.com/kephalaion/kephalaion/releases/latest/download/install.sh | sh"

// FindBinary sucht kephalaion im PATH. Die Prüfung der Anbindung (Check)
// braucht das Binary nicht — sie spricht über HTTP mit dem Node —, aber ohne
// Binary lässt sich auf diesem Rechner kein Node einrichten.
func FindBinary() (string, bool) {
	path, err := exec.LookPath(BinaryName)
	return path, err == nil
}
