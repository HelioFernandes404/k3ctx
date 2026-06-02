package domain

import (
	"sort"
	"strings"
)

// HostRecord is a read-only view of a single cluster host.
type HostRecord struct {
	Client        string
	HostName      string
	SystemframeID *string
	Addr          *string
	ContextName   string
	Group         string
	Status        *string // peer connectivity status (e.g. "Connected", "Connecting", "Idle")
}

// ClientSummary holds the host count for a client.
type ClientSummary struct {
	Client    string
	HostCount int
}

// PageInfo describes a paginated result set.
type PageInfo struct {
	Limit      int
	Returned   int
	Total      int
	HasMore    bool
	NextCursor *string
}

// HostQuery is a filter for host lookups.
type HostQuery struct {
	Client        *string
	HostName      *string
	SystemframeID *string
	Addr          *string
	ContextName   *string
	Query         *string
	Exact         bool
}

// ClientPage is a paginated list of clients.
type ClientPage struct {
	Items []ClientSummary
	Page  PageInfo
}

// HostPage is a paginated list of hosts.
type HostPage struct {
	Items []HostRecord
	Page  PageInfo
	Query HostQuery
}

// ResolutionStatus describes the outcome of a host resolution attempt.
type ResolutionStatus string

// Resolution outcomes.
const (
	ResolutionUnique    ResolutionStatus = "unique"
	ResolutionNoMatch   ResolutionStatus = "no_match"
	ResolutionAmbiguous ResolutionStatus = "ambiguous"
)

// HostResolutionResult is the structured output of a host resolution query.
type HostResolutionResult struct {
	Status      ResolutionStatus
	Query       HostQuery
	Matches     []HostRecord
	Page        PageInfo
	ContextName *string
	Hint        *string
}

// ProjectTargetToHostRecord converts a ClusterTarget to a HostRecord.
func ProjectTargetToHostRecord(target ClusterTarget) HostRecord {
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

	return HostRecord{
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
func BuildHostRecords(targets []ClusterTarget) []HostRecord {
	records := make([]HostRecord, len(targets))
	for i, t := range targets {
		records[i] = ProjectTargetToHostRecord(t)
	}
	return records
}

// BuildClientPage groups host records by client and returns a paginated page sorted by client name.
func BuildClientPage(records []HostRecord, limit int, cursor string) ClientPage {
	counts := map[string]int{}
	for _, r := range records {
		counts[r.Client]++
	}

	clients := make([]string, 0, len(counts))
	for c := range counts {
		clients = append(clients, c)
	}
	sort.Strings(clients)

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

	items := make([]ClientSummary, len(page))
	for i, c := range page {
		items[i] = ClientSummary{Client: c, HostCount: counts[c]}
	}

	return ClientPage{
		Items: items,
		Page: PageInfo{
			Limit:      limit,
			Returned:   len(items),
			Total:      total,
			HasMore:    hasMore,
			NextCursor: nextCursor,
		},
	}
}

// SearchHostRecords filters and paginates an in-memory record set.
func SearchHostRecords(records []HostRecord, query HostQuery, limit int, cursor string) HostPage {
	filtered := filterRecords(records, query)
	total := len(filtered)

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

	return HostPage{
		Items: page,
		Page: PageInfo{
			Limit:      limit,
			Returned:   len(page),
			Total:      total,
			HasMore:    hasMore,
			NextCursor: nextCursor,
		},
		Query: query,
	}
}

// ResolveHostRecords resolves a query against in-memory records, returning a unique/ambiguous/no-match outcome.
func ResolveHostRecords(records []HostRecord, query HostQuery, limit int) HostResolutionResult {
	hostPage := SearchHostRecords(records, query, limit, "")
	matches := hostPage.Items
	total := hostPage.Page.Total

	switch total {
	case 0:
		hint := "No hosts matched the provided identifiers."
		return HostResolutionResult{
			Status:  ResolutionNoMatch,
			Query:   query,
			Matches: []HostRecord{},
			Page:    hostPage.Page,
			Hint:    &hint,
		}
	case 1:
		cn := matches[0].ContextName
		return HostResolutionResult{
			Status:      ResolutionUnique,
			Query:       query,
			Matches:     matches,
			Page:        hostPage.Page,
			ContextName: &cn,
		}
	default:
		hint := "Multiple hosts matched. Refine with --addr, --id, or a more specific host name."
		return HostResolutionResult{
			Status:  ResolutionAmbiguous,
			Query:   query,
			Matches: matches,
			Page:    hostPage.Page,
			Hint:    &hint,
		}
	}
}

func filterRecords(records []HostRecord, q HostQuery) []HostRecord {
	var out []HostRecord
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
