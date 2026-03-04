// Package version holds build-time version information injected via ldflags.
package version

import (
	"fmt"
	"runtime"
)

// These variables are set via -ldflags at build time.
var (
	// Version is the semantic version of the build.
	Version = "dev"
	// Commit is the git commit SHA of the build.
	Commit = "unknown"
	// Date is the build timestamp.
	Date = "unknown"
)

// Info holds structured version information.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

// Get returns the current build's version info.
func Get() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		Date:      Date,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}

// String returns a human-readable version string.
func (i Info) String() string {
	return fmt.Sprintf("github-runner %s (%s) built %s with %s on %s/%s",
		i.Version, i.Commit, i.Date, i.GoVersion, i.OS, i.Arch)
}
