// Package version exposes build metadata injected at link time.
package version

import "fmt"

var (
	// Version is the semantic version or git description of the build.
	Version = "dev"
	// Commit is the short git commit hash of the build.
	Commit = "none"
	// Date is the UTC timestamp of the build.
	Date = "unknown"
)

// String renders the build metadata as a single line.
func String() string {
	return fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, Date)
}
