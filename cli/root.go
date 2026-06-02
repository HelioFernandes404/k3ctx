package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/systemframe/k3ctx/internal/bootstrap"
	"github.com/systemframe/k3ctx/internal/config"
	"github.com/systemframe/k3ctx/internal/paths"
	"github.com/systemframe/k3ctx/internal/telemetry"
	buildversion "github.com/systemframe/k3ctx/internal/version"
)

var (
	svcs           bootstrap.ServiceContainer
	jsonOutput     bool
	cmdStart       time.Time
	telWriter      *telemetry.Writer
	calledCobraCmd *cobra.Command
	calledArgs     []string
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

// writeError prints a structured error to out — JSON when jsonOutput, plain text otherwise.
func writeError(out io.Writer, ecode, msg, hint string) {
	if jsonOutput {
		enc := json.NewEncoder(out)
		_ = enc.Encode(map[string]any{
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
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		calledCobraCmd = cmd
		calledArgs = args
		if !jsonOutput {
			jsonOutput = !isTerminal(os.Stdout)
		}
		projectDir, _ := os.Getwd()
		bootstrapCfg, _ := config.LoadEffectiveConfig(projectDir, os.Getenv("CONFIG_FILE"))
		svcs = bootstrap.Build(bootstrapCfg)
		cmdStart = time.Now()
		telDir := paths.TelemetryDir()
		w, err := telemetry.NewWriter(telDir, 10*1024*1024, 3)
		if err == nil {
			telWriter = w
		}
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if telWriter == nil {
			return nil
		}
		defer telWriter.Close()

		flags := []string{}
		cmd.Flags().Visit(func(f *pflag.Flag) {
			flags = append(flags, "--"+f.Name)
		})

		_ = telWriter.Record(telemetry.Event{
			Cmd:        cmd.Name(),
			Args:       args,
			Flags:      flags,
			DurationMs: time.Since(cmdStart).Milliseconds(),
			Ok:         true,
			Version:    buildversion.Version,
		})
		return nil
	},
}

// Execute runs the root command and handles exit codes.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		recordErrorTelemetry(err)
		var exitErr *ExitErr
		if errors.As(err, &exitErr) {
			writeError(os.Stderr, exitErr.ECode, exitErr.Msg, exitErr.Hint)
			os.Exit(exitErr.Code)
		}
		writeError(os.Stderr, "COMMAND_ERROR", err.Error(), "")
		os.Exit(1)
	}
}

func recordErrorTelemetry(err error) {
	if telWriter == nil {
		return
	}
	defer telWriter.Close()

	cmdName := ""
	var flags []string
	if calledCobraCmd != nil {
		cmdName = calledCobraCmd.Name()
		calledCobraCmd.Flags().Visit(func(f *pflag.Flag) {
			flags = append(flags, "--"+f.Name)
		})
	}

	_ = telWriter.Record(telemetry.Event{
		Cmd:        cmdName,
		Args:       calledArgs,
		Flags:      flags,
		DurationMs: time.Since(cmdStart).Milliseconds(),
		Ok:         false,
		Error:      err.Error(),
		Version:    buildversion.Version,
	})
}

func init() {
	rootCmd.Version = buildversion.Version
	rootCmd.SetVersionTemplate(versionJSON())
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
}
