module example.com/beispiel

go 1.24

toolchain go1.24.2

require (
	github.com/spf13/cobra v1.8.0
	golang.org/x/sync v0.7.0
)

replace example.com/lokal => ../lokal
