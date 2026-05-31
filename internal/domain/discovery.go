package domain

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
