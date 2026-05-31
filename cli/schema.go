package cli

import (
	"encoding/json"
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Print machine-readable command schema (always JSON)",
	RunE:  runSchema,
}

func init() { rootCmd.AddCommand(schemaCmd) }

func runSchema(cmd *cobra.Command, _ []string) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(jsonEnvelope(cmd, map[string]any{
		"version": Version,
		"behavior": map[string]any{
			"json_auto": "JSON output is enabled automatically when stdout is not a TTY",
			"envelope":  "success: {ok,command,data}  error: {ok,error:{code,message,hint,retryable}}",
		},
		"exit_codes": map[string]string{
			"0": "success",
			"1": "usage error or requires confirmation (--yes)",
			"2": "connection or preflight error",
			"3": "ambiguous match — multiple hosts matched the query",
			"4": "no match found",
		},
		"error_codes": []string{
			"NO_MATCH",
			"AMBIGUOUS_MATCH",
			"CONNECTION_FAILED",
			"NETBIRD_NOT_READY",
			"REQUIRES_CONFIRMATION",
			"TARGET_NOT_FOUND",
			"USAGE_ERROR",
			"COMMAND_ERROR",
		},
		"commands": buildCommandList(rootCmd),
	}))
}

func buildCommandList(root *cobra.Command) []map[string]any {
	var result []map[string]any
	for _, c := range root.Commands() {
		if c.Hidden || c.Use == "help [command]" {
			continue
		}
		entry := map[string]any{
			"name":  c.Name(),
			"use":   c.Use,
			"short": c.Short,
			"flags": buildFlagList(c),
		}
		if subs := c.Commands(); len(subs) > 0 {
			entry["subcommands"] = buildCommandList(c)
		}
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i]["name"].(string) < result[j]["name"].(string)
	})
	return result
}

func buildFlagList(cmd *cobra.Command) []map[string]any {
	var flags []map[string]any
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		flags = append(flags, map[string]any{
			"name":        f.Name,
			"type":        f.Value.Type(),
			"default":     f.DefValue,
			"description": f.Usage,
		})
	})
	return flags
}
