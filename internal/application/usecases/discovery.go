package usecases

import "github.com/systemframe/k3ctx/internal/domain"

// LoadHostRecords loads targets from the catalog and projects them to HostRecords.
func LoadHostRecords(catalog domain.InventoryCatalog) ([]domain.HostRecord, error) {
	targets, err := catalog.ListTargets()
	if err != nil {
		return nil, err
	}
	return domain.BuildHostRecords(targets), nil
}

// ListClientSummaries returns paginated client summaries sorted by client name.
func ListClientSummaries(catalog domain.InventoryCatalog, limit int, cursor string) (domain.ClientPage, error) {
	records, err := LoadHostRecords(catalog)
	if err != nil {
		return domain.ClientPage{}, err
	}
	return domain.BuildClientPage(records, limit, cursor), nil
}

// SearchHosts returns paginated host records filtered by query.
func SearchHosts(catalog domain.InventoryCatalog, query domain.HostQuery, limit int, cursor string) (domain.HostPage, error) {
	records, err := LoadHostRecords(catalog)
	if err != nil {
		return domain.HostPage{}, err
	}
	return domain.SearchHostRecords(records, query, limit, cursor), nil
}

// ResolveHost resolves a query to a unique context or returns an ambiguous/no-match result.
func ResolveHost(catalog domain.InventoryCatalog, query domain.HostQuery, limit int) (domain.HostResolutionResult, error) {
	records, err := LoadHostRecords(catalog)
	if err != nil {
		return domain.HostResolutionResult{}, err
	}
	return domain.ResolveHostRecords(records, query, limit), nil
}
