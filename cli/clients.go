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

var clientsCmd = &cobra.Command{
	Use:   "clients [query]",
	Short: "List clients with host counts",
	RunE:  runClients,
}

var (
	clientsLimit            int
	clientsCursor           string
	clientsRefreshInventory bool
)

func init() {
	rootCmd.AddCommand(clientsCmd)
	clientsCmd.Flags().IntVar(&clientsLimit, "limit", 20, "Max results per page")
	clientsCmd.Flags().StringVar(&clientsCursor, "cursor", "", "Pagination cursor")
	clientsCmd.Flags().BoolVar(&clientsRefreshInventory, "refresh-inventory", false, "Refresh inventory before listing")
}

func runClients(cmd *cobra.Command, args []string) error {
	projectDir, _ := os.Getwd()
	cfg, err := config.LoadEffectiveConfig(projectDir, os.Getenv("CONFIG_FILE"))
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	if clientsRefreshInventory {
		usecases.RefreshInventoryIfPossible(cfg.InventoryPath, svcs.Refresher)
	}

	q := domain.HostQuery{}
	if len(args) > 0 {
		s := args[0]
		q.Query = &s
	}

	page, err := usecases.ListClientSummaries(cfg.InventoryPath, svcs.Catalog, clientsLimit, clientsCursor)
	if err != nil {
		return err
	}

	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(jsonEnvelope(cmd, clientPageToMap(page)))
	}

	for _, item := range page.Items {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-30s  %d hosts\n", item.Client, item.HostCount)
	}
	if page.Page.HasMore {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "(more: --cursor %s)\n", *page.Page.NextCursor)
	}
	return nil
}

func clientPageToMap(p domain.ClientPage) map[string]any {
	items := make([]map[string]any, len(p.Items))
	for i, c := range p.Items {
		items[i] = map[string]any{"client": c.Client, "host_count": c.HostCount}
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
