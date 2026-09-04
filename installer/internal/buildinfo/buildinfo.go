// Package buildinfo holds values stamped into the executable at build time.
package buildinfo

// Version is set through -ldflags at build time. It stays empty only for an
// ad-hoc go build that intentionally omits the build flags.
var Version string
