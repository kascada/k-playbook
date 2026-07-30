package ui

import (
	"fmt"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/pathcontract"
)

func RenderPathStatus(result pathcontract.Result, styled bool) string {
	_ = styled

	var builder strings.Builder

	title := "Path contract"
	builder.WriteString(title)
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("  expected: %s\n", result.Expected))

	current := result.Current
	if current == "" {
		current = "not detected"
	}
	builder.WriteString(fmt.Sprintf("  current:  %s\n", current))

	code := string(result.Code)
	builder.WriteString(fmt.Sprintf("  result:   %s\n", code))

	if result.ExpectedIsSymlink {
		builder.WriteString(fmt.Sprintf("  symlink:  %s\n", result.ExpectedSymlinkTarget))
	}

	if result.Message != "" {
		builder.WriteString("\n")
		builder.WriteString(result.Message)
		builder.WriteString("\n")
	}

	if result.FixCommand != "" {
		builder.WriteString("\n")
		fix := "Fix: " + result.FixCommand
		builder.WriteString(fix)
		builder.WriteString("\n")
	}

	return builder.String()
}
