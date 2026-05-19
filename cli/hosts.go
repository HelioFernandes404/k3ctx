package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/systemframe/k3ctx/internal/application/usecases"
	"github.com/systemframe/k3ctx/internal/config"
	"github.com/systemframe/k3ctx/internal/domain"
)

var hostsCmd = &cobra.Command{
	Use:   "hosts CLIENT [query]",
	Short: "List or search hosts in one client",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runHosts,
}

var (
	hostsLimit            int
	hostsCursor           string
	hostsRefreshInventory bool
	hostsHostFilter       string
	hostsIDFilter         string
	hostsIPFilter         string
)

func init() {
	rootCmd.AddCommand(hostsCmd)
	hostsCmd.Flags().IntVar(&hostsLimit, "limit", 20, "Max results per page")
	hostsCmd.Flags().StringVar(&hostsCursor, "cursor", "", "Pagination cursor")
	hostsCmd.Flags().BoolVar(&hostsRefreshInventory, "refresh-inventory", false, "Refresh inventory before listing")
	hostsCmd.Flags().StringVar(&hostsHostFilter, "host", "", "Filter by host name (substring)")
	hostsCmd.Flags().StringVar(&hostsIDFilter, "id", "", "Filter by systemframe_id")
	hostsCmd.Flags().StringVar(&hostsIPFilter, "ip", "", "Filter by IP address")
}

func runHosts(cmd *cobra.Command, args []string) error {
	client := args[0]
	projectDir, _ := os.Getwd()
	cfg, err := config.LoadEffectiveConfig(projectDir, os.Getenv("CONFIG_FILE"))
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	if hostsRefreshInventory {
		usecases.RefreshInventoryIfPossible(cfg.InventoryPath, svcs.Refresher)
	}

	q := domain.HostQuery{Client: &client}
	if len(args) > 1 {
		q.HostName = &args[1]
	}
	if hostsHostFilter != "" {
		q.HostName = &hostsHostFilter
	}
	if hostsIDFilter != "" {
		q.SystemframeID = &hostsIDFilter
	}
	if hostsIPFilter != "" {
		q.AddrIP = &hostsIPFilter
	}

	page, err := usecases.SearchHosts(cfg.InventoryPath, svcs.Catalog, q, hostsLimit, hostsCursor)
	if err != nil {
		return err
	}

	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(jsonEnvelope(cmd, hostPageToMap(page)))
	}

	for _, item := range page.Items {
		ip := ""
		if item.AddrIP != nil {
			ip = *item.AddrIP
		}
		sfID := ""
		if item.SystemframeID != nil {
			sfID = *item.SystemframeID
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-30s  %-15s  %s\n", item.ContextName, ip, sfID)
	}
	if page.Page.HasMore {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "(more: --cursor %s)\n", *page.Page.NextCursor)
	}
	return nil
}

func hostPageToMap(p domain.HostPage) map[string]any {
	items := make([]map[string]any, len(p.Items))
	for i, h := range p.Items {
		var ip, sfID any
		if h.AddrIP != nil {
			ip = *h.AddrIP
		}
		if h.SystemframeID != nil {
			sfID = *h.SystemframeID
		}
		items[i] = map[string]any{
			"client":         h.Client,
			"host_name":      h.HostName,
			"context_name":   h.ContextName,
			"addr_ip":        ip,
			"systemframe_id": sfID,
		}
	}
	cursor := any(nil)
	if p.Page.NextCursor != nil {
		cursor = *p.Page.NextCursor
	}
	return map[string]any{
		"items": items,
		"page": map[string]any{
			"limit":       p.Page.Limit,
			"returned":    p.Page.Returned,
			"total":       p.Page.Total,
			"has_more":    p.Page.HasMore,
			"next_cursor": cursor,
		},
	}
}
