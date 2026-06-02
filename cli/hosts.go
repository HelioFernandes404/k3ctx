package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/systemframe/k3ctx/internal/application/usecases"
	"github.com/systemframe/k3ctx/internal/domain"
)

var hostsCmd = &cobra.Command{
	Use:   "hosts CLIENT [query]",
	Short: "List or search hosts in one client",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runHosts,
}

var (
	hostsLimit      int
	hostsCursor     string
	hostsHostFilter string
	hostsIDFilter   string
	hostsAddrFilter string
	hostsAll        bool
)

func init() {
	rootCmd.AddCommand(hostsCmd)
	hostsCmd.Flags().IntVar(&hostsLimit, "limit", 20, "Max results per page")
	hostsCmd.Flags().StringVar(&hostsCursor, "cursor", "", "Pagination cursor")
	hostsCmd.Flags().StringVar(&hostsHostFilter, "host", "", "Filter by host name (substring)")
	hostsCmd.Flags().StringVar(&hostsIDFilter, "id", "", "Filter by systemframe_id")
	hostsCmd.Flags().StringVar(&hostsAddrFilter, "addr", "", "Filter by address (FQDN)")
	hostsCmd.Flags().BoolVar(&hostsAll, "all", false, "Return all hosts without pagination")
}

func runHosts(cmd *cobra.Command, args []string) error {
	client := args[0]

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
	if hostsAddrFilter != "" {
		q.Addr = &hostsAddrFilter
	}

	if hostsAll {
		page, err := usecases.SearchHosts(svcs.Catalog, q, 0, "")
		if err != nil {
			return err
		}
		items := hostItemsToMaps(page.Items)
		if jsonOutput {
			enc := json.NewEncoder(cmd.OutOrStdout())
			return enc.Encode(map[string]any{"items": items})
		}
		for _, item := range page.Items {
			printHostLine(cmd, item)
		}
		return nil
	}

	page, err := usecases.SearchHosts(svcs.Catalog, q, hostsLimit, hostsCursor)
	if err != nil {
		return err
	}

	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(hostPageToMap(page))
	}

	for _, item := range page.Items {
		printHostLine(cmd, item)
	}
	if page.Page.HasMore {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "(more: --cursor %s)\n", *page.Page.NextCursor)
	}
	return nil
}

func printHostLine(cmd *cobra.Command, item domain.HostRecord) {
	addr := ""
	if item.Addr != nil {
		addr = *item.Addr
	}
	sfID := ""
	if item.SystemframeID != nil {
		sfID = *item.SystemframeID
	}
	status := ""
	if item.Status != nil {
		status = "[" + *item.Status + "] "
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s%-40s  %-45s  %s\n", status, item.ContextName, addr, sfID)
}

func hostItemsToMaps(items []domain.HostRecord) []map[string]any {
	result := make([]map[string]any, len(items))
	for i, h := range items {
		var addr, sfID, status any
		if h.Addr != nil {
			addr = *h.Addr
		}
		if h.SystemframeID != nil {
			sfID = *h.SystemframeID
		}
		if h.Status != nil {
			status = *h.Status
		}
		result[i] = map[string]any{
			"client":         h.Client,
			"host_name":      h.HostName,
			"context_name":   h.ContextName,
			"addr":           addr,
			"systemframe_id": sfID,
			"status":         status,
		}
	}
	return result
}

func hostPageToMap(p domain.HostPage) map[string]any {
	cursor := any(nil)
	if p.Page.NextCursor != nil {
		cursor = *p.Page.NextCursor
	}
	return map[string]any{
		"items": hostItemsToMaps(p.Items),
		"page": map[string]any{
			"limit":       p.Page.Limit,
			"returned":    p.Page.Returned,
			"total":       p.Page.Total,
			"has_more":    p.Page.HasMore,
			"next_cursor": cursor,
		},
	}
}
