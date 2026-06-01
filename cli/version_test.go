package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	buildversion "github.com/systemframe/k3ctx/internal/version"
)

func TestVersionCommand(t *testing.T) {
	buildversion.Version = "v0.1.0"
	buildversion.Commit = "abc1234"
	buildversion.Date = "2026-06-01T00:00:00Z"
	rootCmd.SetVersionTemplate(versionJSON())

	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"--version"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got map[string]string
	if err := json.NewDecoder(buf).Decode(&got); err != nil {
		t.Fatalf("decode version JSON: %v", err)
	}
	if got["version"] != "v0.1.0" || got["commit"] != "abc1234" || got["date"] != "2026-06-01T00:00:00Z" {
		t.Errorf("unexpected version JSON: %#v", got)
	}
}
