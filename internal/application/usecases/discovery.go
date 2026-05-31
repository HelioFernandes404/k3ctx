package usecases

import (
	"sort"
	"strings"

	"github.com/systemframe/k3ctx/internal/application"
	"github.com/systemframe/k3ctx/internal/domain"
)

// ProjectTargetToHostRecord converts a ClusterTarget to a HostRecord.
func ProjectTargetToHostRecord(target domain.ClusterTarget) domain.HostRecord {
	hc := target.HostConfig()
	gv := target.GroupVars()

	var sfID, ip, status *string
	if v, ok := hc["systemframe_id"].(string); ok {
		sfID = &v
	} else if v, ok := gv["systemframe_id"].(string); ok {
		sfID = &v
	}
	if v, ok := hc["addr"].(string); ok {
		ip = &v
	}
	if v, ok := hc["netbird_status"].(string); ok {
		status = &v
	}

	return domain.HostRecord{
		Client:        target.Company(),
		HostName:      target.HostAlias(),
		SystemframeID: sfID,
		Addr:          ip,
		ContextName:   target.ContextName(),
		Group:         target.Group(),
		Status:        status,
	}
}

// BuildHostRecords projects a slice of ClusterTargets to HostRecords.
func BuildHostRecords(targets []domain.ClusterTarget) []domain.HostRecord {
	records := make([]domain.HostRecord, len(targets))
	for i, t := range targets {
		records[i] = ProjectTargetToHostRecord(t)
	}
	return records
}

// LoadHostRecords loads targets from the catalog and projects them.
func LoadHostRecords(catalog application.InventoryCatalog) ([]domain.HostRecord, error) {
	targets, err := catalog.ListTargets()
	if err != nil {
		return nil, err
	}
	return BuildHostRecords(targets), nil
}

// ListClientSummaries returns paginated client summaries sorted by client name.
func ListClientSummaries(catalog application.InventoryCatalog, limit int, cursor string) (domain.ClientPage, error) {
	records, err := LoadHostRecords(catalog)
	if err != nil {
		return domain.ClientPage{}, err
	}

	counts := map[string]int{}
	for _, r := range records {
		counts[r.Client]++
	}

	clients := make([]string, 0, len(counts))
	for c := range counts {
		clients = append(clients, c)
	}
	sort.Strings(clients)

	// Apply cursor
	start := 0
	if cursor != "" {
		for i, c := range clients {
			if c == cursor {
				start = i + 1
				break
			}
		}
	}

	all := clients[start:]
	total := len(counts)

	end := len(all)
	if limit > 0 && limit < end {
		end = limit
	}

	page := all[:end]
	hasMore := end < len(all)

	var nextCursor *string
	if hasMore && len(page) > 0 {
		c := page[len(page)-1]
		nextCursor = &c
	}

	items := make([]domain.ClientSummary, len(page))
	for i, c := range page {
		items[i] = domain.ClientSummary{Client: c, HostCount: counts[c]}
	}

	return domain.ClientPage{
		Items: items,
		Page: domain.PageInfo{
			Limit:      limit,
			Returned:   len(items),
			Total:      total,
			HasMore:    hasMore,
			NextCursor: nextCursor,
		},
	}, nil
}

// SearchHosts returns paginated host records filtered by query.
func SearchHosts(catalog application.InventoryCatalog, query domain.HostQuery, limit int, cursor string) (domain.HostPage, error) {
	records, err := LoadHostRecords(catalog)
	if err != nil {
		return domain.HostPage{}, err
	}
	return SearchHostRecords(records, query, limit, cursor), nil
}

// SearchHostRecords filters and paginates an in-memory record set.
func SearchHostRecords(records []domain.HostRecord, query domain.HostQuery, limit int, cursor string) domain.HostPage {
	filtered := filterRecords(records, query)
	total := len(filtered)

	// Apply cursor (context_name-based)
	start := 0
	if cursor != "" {
		for i, r := range filtered {
			if r.ContextName == cursor {
				start = i + 1
				break
			}
		}
	}

	page := filtered[start:]
	end := len(page)
	if limit > 0 && limit < end {
		end = limit
	}
	page = page[:end]
	hasMore := start+end < total

	var nextCursor *string
	if hasMore && len(page) > 0 {
		c := page[len(page)-1].ContextName
		nextCursor = &c
	}

	return domain.HostPage{
		Items: page,
		Page: domain.PageInfo{
			Limit:      limit,
			Returned:   len(page),
			Total:      total,
			HasMore:    hasMore,
			NextCursor: nextCursor,
		},
		Query: query,
	}
}

// ResolveHost resolves a query to a unique context or returns an ambiguous/no-match result.
func ResolveHost(catalog application.InventoryCatalog, query domain.HostQuery, limit int) (domain.HostResolutionResult, error) {
	records, err := LoadHostRecords(catalog)
	if err != nil {
		return domain.HostResolutionResult{}, err
	}
	return ResolveHostRecords(records, query, limit), nil
}

// ResolveHostRecords resolves against in-memory records without a catalog round-trip.
func ResolveHostRecords(records []domain.HostRecord, query domain.HostQuery, limit int) domain.HostResolutionResult {
	hostPage := SearchHostRecords(records, query, limit, "")
	matches := hostPage.Items
	total := hostPage.Page.Total

	switch {
	case total == 0:
		hint := "No hosts matched the provided identifiers."
		return domain.HostResolutionResult{
			Status:  domain.ResolutionNoMatch,
			Query:   query,
			Matches: []domain.HostRecord{},
			Page:    hostPage.Page,
			Hint:    &hint,
		}
	case total == 1:
		cn := matches[0].ContextName
		return domain.HostResolutionResult{
			Status:      domain.ResolutionUnique,
			Query:       query,
			Matches:     matches,
			Page:        hostPage.Page,
			ContextName: &cn,
		}
	default:
		hint := "Multiple hosts matched. Refine with --addr, --id, or a more specific host name."
		return domain.HostResolutionResult{
			Status:  domain.ResolutionAmbiguous,
			Query:   query,
			Matches: matches,
			Page:    hostPage.Page,
			Hint:    &hint,
		}
	}
}

func filterRecords(records []domain.HostRecord, q domain.HostQuery) []domain.HostRecord {
	var out []domain.HostRecord
	for _, r := range records {
		if q.Client != nil && r.Client != *q.Client {
			continue
		}
		if q.HostName != nil {
			if q.Exact {
				if r.HostName != *q.HostName {
					continue
				}
			} else if !strings.Contains(r.HostName, *q.HostName) {
				continue
			}
		}
		if q.SystemframeID != nil {
			if r.SystemframeID == nil || *r.SystemframeID != *q.SystemframeID {
				continue
			}
		}
		if q.Addr != nil {
			if r.Addr == nil || *r.Addr != *q.Addr {
				continue
			}
		}
		if q.ContextName != nil && r.ContextName != *q.ContextName {
			continue
		}
		if q.Query != nil {
			needle := *q.Query
			match := strings.Contains(r.HostName, needle) ||
				strings.Contains(r.ContextName, needle) ||
				(r.SystemframeID != nil && strings.Contains(*r.SystemframeID, needle)) ||
				(r.Addr != nil && strings.Contains(*r.Addr, needle))
			if !match {
				continue
			}
		}
		out = append(out, r)
	}
	return out
}
