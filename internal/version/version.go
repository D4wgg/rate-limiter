package version

var (
	// Version is the application version (set at build time via ldflags)
	Version = "dev"
	// BuildTime is the build timestamp (set at build time via ldflags)
	BuildTime = "unknown"
	// GitCommit is the git commit hash (set at build time via ldflags)
	GitCommit = "unknown"
)
