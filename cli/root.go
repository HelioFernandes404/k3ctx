package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/systemframe/k3ctx/internal/bootstrap"
)

var (
	svcs       bootstrap.ServiceContainer
	jsonOutput bool
)

// ExitErr carries a structured exit code through Cobra's error return path.
type ExitErr struct {
	Code  int
	ECode string
	Msg   string
	Hint  string
}

func (e *ExitErr) Error() string { return e.Msg }

func newExitErr(code int, ecode, msg, hint string) *ExitErr {
	return &ExitErr{Code: code, ECode: ecode, Msg: msg, Hint: hint}
}

// writeError prints a structured error to out — JSON envelope when jsonOutput, plain text otherwise.
func writeError(out io.Writer, ecode, msg, hint string) {
	if jsonOutput {
		enc := json.NewEncoder(out)
		_ = enc.Encode(map[string]any{
			"ok": false,
			"error": map[string]any{
				"code":    ecode,
				"message": msg,
				"hint":    hint,
			},
		})
		return
	}
	if hint != "" {
		_, _ = fmt.Fprintf(out, "%s\n%s\n", msg, hint)
	} else {
		_, _ = fmt.Fprintln(out, msg)
	}
}

// jsonEnvelope wraps a success payload in the standard {"ok":true,"command":"...","data":...} envelope.
func jsonEnvelope(cmd *cobra.Command, data any) map[string]any {
	return map[string]any{
		"ok":      true,
		"command": cmd.Name(),
		"data":    data,
	}
}

// isTerminal reports whether f is connected to a terminal.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

var rootCmd = &cobra.Command{
	Use:   "k3ctx",
	Short: "K3s context tunnel manager",
	Long:  "Manage SSH tunnels and kubectl contexts for K3s clusters.",
	PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
		if !jsonOutput {
			jsonOutput = !isTerminal(os.Stdout)
		}
		svcs = bootstrap.Build()
		return nil
	},
}

// Execute runs the root command and handles exit codes.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		var exitErr *ExitErr
		if errors.As(err, &exitErr) {
			writeError(os.Stdout, exitErr.ECode, exitErr.Msg, exitErr.Hint)
			os.Exit(exitErr.Code)
		}
		writeError(os.Stdout, "COMMAND_ERROR", err.Error(), "")
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
}
