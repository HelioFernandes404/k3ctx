package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/systemframe/k3ctx/internal/bootstrap"
	"github.com/systemframe/k3ctx/internal/config"
)

var (
	svcs      bootstrap.ServiceContainer
	cfg       = mustDefaultConfig()
	jsonOutput bool
)

func mustDefaultConfig() interface{} {
	// deferred; resolved at runtime in each command
	return nil
}

var rootCmd = &cobra.Command{
	Use:   "k3ctx",
	Short: "K3s context tunnel manager",
	Long:  "Manage SSH tunnels and kubectl contexts for K3s clusters.",
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		svcs = bootstrap.Build()
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
}

func loadConfig() (interface{}, error) {
	projectDir, _ := os.Getwd()
	c, err := config.LoadEffectiveConfig(projectDir, os.Getenv("CONFIG_FILE"))
	if err != nil {
		return nil, err
	}
	return c, nil
}
