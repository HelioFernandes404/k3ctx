package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var k9sCmd = &cobra.Command{
	Use:   "k9s",
	Short: "Launch k9s after tunnel validation",
	RunE:  runK9s,
}

func init() { rootCmd.AddCommand(k9sCmd) }

func runK9s(_ *cobra.Command, _ []string) error {
	k9sBin, err := exec.LookPath("k9s")
	if err != nil {
		return fmt.Errorf("k9s not found in PATH")
	}
	proc := exec.Command(k9sBin)
	proc.Stdin = os.Stdin
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	return proc.Run()
}
