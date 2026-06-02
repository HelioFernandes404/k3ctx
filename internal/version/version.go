package version

// Build metadata injected at compile time via -ldflags.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Info is the JSON-serializable build metadata reported by `k3ctx --version`.
type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// Current returns the build metadata for the running binary.
func Current() Info {
	return Info{Version: Version, Commit: Commit, Date: Date}
}
