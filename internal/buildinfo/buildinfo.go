// Package buildinfo exposes release metadata that can be replaced with Go
// linker flags without changing source files.
package buildinfo

import "runtime"

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

type Info struct {
	Product, Version, Commit, BuildTime, GoVersion string
}

func Current() Info {
	return Info{Product: "Agent Audit Console", Version: Version, Commit: Commit, BuildTime: BuildTime, GoVersion: runtime.Version()}
}
