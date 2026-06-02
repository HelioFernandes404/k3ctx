package usecases_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systemframe/k3ctx/internal/application/usecases"
	"github.com/systemframe/k3ctx/internal/domain"
)

func strP(s string) *string { return &s }

func buildDiscoveryTarget(company, hostAlias, addr string, sfID *string) domain.ClusterTarget {
	hc := map[string]any{"addr": addr}
	if sfID != nil {
		hc["systemframe_id"] = *sfID
	}
	return domain.NewClusterTarget(company, hostAlias, "k3s_cluster", hc, nil)
}

func TestLoadHostRecords_ProjectsPublicFieldsFromClusterTargets(t *testing.T) {
	catalog := &stubCatalog{targets: []domain.ClusterTarget{
		buildDiscoveryTarget("acme", "prod", "10.0.0.10", strP("sf-1042")),
	}}

	records, err := usecases.LoadHostRecords(catalog)
	require.NoError(t, err)
	require.Len(t, records, 1)

	r := records[0]
	assert.Equal(t, "acme", r.Client)
	assert.Equal(t, "prod", r.HostName)
	assert.Equal(t, "sf-1042", *r.SystemframeID)
	assert.Equal(t, "10.0.0.10", *r.Addr)
	assert.Equal(t, "acme-prod", r.ContextName)
	assert.Equal(t, "k3s_cluster", r.Group)
}

func TestBuildHostRecords_ProjectsPublicFieldsFromTargets(t *testing.T) {
	records := domain.BuildHostRecords([]domain.ClusterTarget{
		buildDiscoveryTarget("acme", "prod", "10.0.0.10", strP("sf-1042")),
	})
	require.Len(t, records, 1)
	assert.Equal(t, "sf-1042", *records[0].SystemframeID)
}

func TestLoadHostRecords_FallsBackToGroupVarsForSystemframeID(t *testing.T) {
	target := domain.NewClusterTarget("acme", "prod", "k3s_cluster",
		map[string]any{"addr": "10.0.0.10"},
		map[string]any{"systemframe_id": "sf-group-7"},
	)
	catalog := &stubCatalog{targets: []domain.ClusterTarget{target}}

	records, err := usecases.LoadHostRecords(catalog)
	require.NoError(t, err)
	assert.Equal(t, "sf-group-7", *records[0].SystemframeID)
}

func TestListClientSummaries_ReturnsSortedCountsAndCursor(t *testing.T) {
	catalog := &stubCatalog{targets: []domain.ClusterTarget{
		buildDiscoveryTarget("beta", "api-01", "10.0.0.1", nil),
		buildDiscoveryTarget("acme", "api-01", "10.0.0.2", nil),
		buildDiscoveryTarget("acme", "api-02", "10.0.0.3", nil),
	}}

	page, err := usecases.ListClientSummaries(catalog, 1, "")
	require.NoError(t, err)

	assert.Len(t, page.Items, 1)
	assert.Equal(t, "acme", page.Items[0].Client)
	assert.Equal(t, 2, page.Items[0].HostCount)
	assert.Equal(t, 1, page.Page.Limit)
	assert.Equal(t, 1, page.Page.Returned)
	assert.Equal(t, 2, page.Page.Total)
	assert.True(t, page.Page.HasMore)
	require.NotNil(t, page.Page.NextCursor)
	assert.Equal(t, "acme", *page.Page.NextCursor)
}

func TestSearchHosts_FiltersByClientAndContextCursor(t *testing.T) {
	catalog := &stubCatalog{targets: []domain.ClusterTarget{
		buildDiscoveryTarget("acme", "api-01", "10.0.0.1", strP("sf-1")),
		buildDiscoveryTarget("acme", "api-02", "10.0.0.2", strP("sf-2")),
		buildDiscoveryTarget("acme", "db-01", "10.0.0.3", strP("sf-3")),
		buildDiscoveryTarget("beta", "api-01", "10.0.0.9", strP("sf-9")),
	}}

	q := domain.HostQuery{Client: strP("acme"), HostName: strP("api")}
	first, err := usecases.SearchHosts(catalog, q, 1, "")
	require.NoError(t, err)

	assert.Equal(t, []string{"acme-api-01"}, contextNames(first.Items))
	assert.Equal(t, 2, first.Page.Total)
	assert.True(t, first.Page.HasMore)
	require.NotNil(t, first.Page.NextCursor)
	assert.Equal(t, "acme-api-01", *first.Page.NextCursor)

	second, err := usecases.SearchHosts(catalog, q, 1, *first.Page.NextCursor)
	require.NoError(t, err)
	assert.Equal(t, []string{"acme-api-02"}, contextNames(second.Items))
	assert.False(t, second.Page.HasMore)
}

func TestSearchHosts_SupportsIDAddrAndFreeTextFilters(t *testing.T) {
	catalog := &stubCatalog{targets: []domain.ClusterTarget{
		buildDiscoveryTarget("acme", "api-prod", "10.0.0.10", strP("sf-1042")),
		buildDiscoveryTarget("acme", "api-dev", "10.0.0.11", strP("sf-2001")),
	}}

	q := domain.HostQuery{Client: strP("acme"), Query: strP("1042"), Addr: strP("10.0.0.10")}
	page, err := usecases.SearchHosts(catalog, q, 20, "")
	require.NoError(t, err)

	assert.Equal(t, []string{"acme-api-prod"}, contextNames(page.Items))
	assert.Equal(t, 1, page.Page.Total)
}

func TestResolveHost_ReturnsUniqueForSingleMatch(t *testing.T) {
	catalog := &stubCatalog{targets: []domain.ClusterTarget{
		buildDiscoveryTarget("acme", "api-prod", "10.0.0.1", strP("sf-1042")),
		buildDiscoveryTarget("acme", "db-prod", "10.0.0.2", strP("sf-2001")),
	}}

	result, err := usecases.ResolveHost(catalog, domain.HostQuery{Client: strP("acme"), HostName: strP("api")}, 20)
	require.NoError(t, err)

	assert.Equal(t, domain.ResolutionUnique, result.Status)
	require.NotNil(t, result.ContextName)
	assert.Equal(t, "acme-api-prod", *result.ContextName)
	assert.Equal(t, []string{"acme-api-prod"}, contextNames(result.Matches))
}

func TestResolveHost_ReturnsAmbiguousWithHint(t *testing.T) {
	catalog := &stubCatalog{targets: []domain.ClusterTarget{
		buildDiscoveryTarget("acme", "api-01", "10.0.0.1", strP("sf-1")),
		buildDiscoveryTarget("acme", "api-02", "10.0.0.2", strP("sf-2")),
		buildDiscoveryTarget("acme", "api-03", "10.0.0.3", strP("sf-3")),
	}}

	result, err := usecases.ResolveHost(catalog, domain.HostQuery{Client: strP("acme"), HostName: strP("api")}, 2)
	require.NoError(t, err)

	assert.Equal(t, domain.ResolutionAmbiguous, result.Status)
	assert.Nil(t, result.ContextName)
	assert.Equal(t, []string{"acme-api-01", "acme-api-02"}, contextNames(result.Matches))
	assert.Equal(t, 3, result.Page.Total)
	require.NotNil(t, result.Hint)
	assert.Contains(t, *result.Hint, "Multiple hosts matched")
}

func TestResolveHost_ReturnsNoMatchWhenFiltersDoNotOverlap(t *testing.T) {
	catalog := &stubCatalog{targets: []domain.ClusterTarget{
		buildDiscoveryTarget("acme", "api-prod", "10.0.0.10", nil),
		buildDiscoveryTarget("acme", "db-prod", "10.0.0.20", nil),
	}}

	result, err := usecases.ResolveHost(catalog, domain.HostQuery{
		Client: strP("acme"), HostName: strP("api"), Addr: strP("10.0.0.20"),
	}, 20)
	require.NoError(t, err)

	assert.Equal(t, domain.ResolutionNoMatch, result.Status)
	assert.Nil(t, result.ContextName)
	assert.Empty(t, result.Matches)
	require.NotNil(t, result.Hint)
	assert.Contains(t, *result.Hint, "No hosts matched")
}

func TestResolveHostRecords_ReturnsUniqueWithoutCatalogRoundtrip(t *testing.T) {
	records := domain.BuildHostRecords([]domain.ClusterTarget{
		buildDiscoveryTarget("acme", "api-prod", "10.0.0.1", strP("sf-1042")),
		buildDiscoveryTarget("acme", "db-prod", "10.0.0.2", strP("sf-2001")),
	})

	result := domain.ResolveHostRecords(records, domain.HostQuery{Client: strP("acme"), HostName: strP("api")}, 20)

	assert.Equal(t, domain.ResolutionUnique, result.Status)
	assert.Equal(t, "acme-api-prod", *result.ContextName)
}

func contextNames(records []domain.HostRecord) []string {
	out := make([]string, len(records))
	for i, r := range records {
		out[i] = r.ContextName
	}
	return out
}
