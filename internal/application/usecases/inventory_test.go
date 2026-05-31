package usecases_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systemframe/k3ctx/internal/application/usecases"
	"github.com/systemframe/k3ctx/internal/domain"
)

// --- Stub catalog and refresher ---

type stubCatalog struct {
	targets []domain.ClusterTarget
	calls   []string
}

func (s *stubCatalog) ListTargets(inventoryPath string) ([]domain.ClusterTarget, error) {
	s.calls = append(s.calls, inventoryPath)
	return append([]domain.ClusterTarget(nil), s.targets...), nil
}

type stubRefresher struct {
	result [2]any
	calls  []string
}

func (s *stubRefresher) Refresh(inventoryPath string) (bool, string) {
	s.calls = append(s.calls, inventoryPath)
	return s.result[0].(bool), s.result[1].(string)
}

func buildTestTarget(company, hostAlias, ansibleHost string) domain.ClusterTarget {
	return domain.NewClusterTarget(company, hostAlias, "k3s_cluster",
		map[string]any{"ansible_host": ansibleHost}, nil)
}

// --- Tests ---

func TestDeduplicateContextNames_PreservesOrder(t *testing.T) {
	result := usecases.DeduplicateContextNames([]string{"beta-dev", "acme-prod", "beta-dev"})
	assert.Equal(t, []string{"beta-dev", "acme-prod"}, result)
}

func TestDeduplicateContextNames_EmptyInput(t *testing.T) {
	assert.Empty(t, usecases.DeduplicateContextNames(nil))
}

func TestDeduplicateContextNames_SingleItem(t *testing.T) {
	assert.Equal(t, []string{"acme-prod"}, usecases.DeduplicateContextNames([]string{"acme-prod"}))
}

func TestFindTargetByContextName_ReturnsMatchingTarget(t *testing.T) {
	target := buildTestTarget("acme", "prod", "203.0.113.10")
	catalog := &stubCatalog{targets: []domain.ClusterTarget{target}}

	result, err := usecases.FindTargetByContextName("acme-prod", t.TempDir(), catalog)

	require.NoError(t, err)
	assert.Equal(t, target.ContextName(), result.ContextName())
}

func TestFindTargetByContextName_ReturnsNilWhenNotFound(t *testing.T) {
	catalog := &stubCatalog{}

	result, err := usecases.FindTargetByContextName("acme-prod", t.TempDir(), catalog)

	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestFindTargetByContextName_ReturnsNilWhenContextNotInCatalog(t *testing.T) {
	target := buildTestTarget("acme", "dev", "203.0.113.10")
	catalog := &stubCatalog{targets: []domain.ClusterTarget{target}}

	result, err := usecases.FindTargetByContextName("acme-prod", t.TempDir(), catalog)

	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestListClusterTargets_DelegatesToCatalog(t *testing.T) {
	target := buildTestTarget("acme", "prod", "203.0.113.10")
	catalog := &stubCatalog{targets: []domain.ClusterTarget{target}}

	result, err := usecases.ListClusterTargets(t.TempDir(), catalog)

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, target.ContextName(), result[0].ContextName())
}

func TestListClusterTargets_ReturnsEmptyForEmptyCatalog(t *testing.T) {
	catalog := &stubCatalog{}
	result, err := usecases.ListClusterTargets(t.TempDir(), catalog)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestSelectTargetsByContextName_ReturnsMissingContexts(t *testing.T) {
	first := buildTestTarget("acme", "prod", "203.0.113.10")
	second := buildTestTarget("beta", "staging", "203.0.113.11")
	catalog := &stubCatalog{targets: []domain.ClusterTarget{first, second}}

	selected, missing, err := usecases.SelectTargetsByContextName(
		[]string{"beta-staging", "missing", "acme-prod"}, t.TempDir(), catalog)

	require.NoError(t, err)
	require.Len(t, selected, 2)
	assert.Equal(t, []string{"beta-staging", "acme-prod"}, []string{selected[0].ContextName(), selected[1].ContextName()})
	assert.Equal(t, []string{"missing"}, missing)
}

func TestSelectTargetsByContextName_ReturnsAllMissingWhenCatalogEmpty(t *testing.T) {
	catalog := &stubCatalog{}
	selected, missing, err := usecases.SelectTargetsByContextName(
		[]string{"acme-prod", "beta-staging"}, t.TempDir(), catalog)
	require.NoError(t, err)
	assert.Empty(t, selected)
	assert.Equal(t, []string{"acme-prod", "beta-staging"}, missing)
}

func TestRefreshInventoryIfPossible_ReturnsNilWhenPathMissing(t *testing.T) {
	refresher := &stubRefresher{result: [2]any{true, "ok"}}
	result := usecases.RefreshInventoryIfPossible(filepath.Join(t.TempDir(), "missing"), refresher)
	assert.Nil(t, result)
	assert.Empty(t, refresher.calls)
}

func TestRefreshInventoryIfPossible_CallsRefresherWhenPathExists(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "inventory")
	require.NoError(t, os.Mkdir(dir, 0o755))
	refresher := &stubRefresher{result: [2]any{true, "Updated successfully"}}

	result := usecases.RefreshInventoryIfPossible(dir, refresher)

	require.NotNil(t, result)
	assert.Equal(t, true, (*result)[0])
	assert.Equal(t, "Updated successfully", (*result)[1])
	assert.Equal(t, []string{dir}, refresher.calls)
}

func TestRefreshInventoryIfPossible_PropagatesFailure(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "inventory")
	require.NoError(t, os.Mkdir(dir, 0o755))
	refresher := &stubRefresher{result: [2]any{false, "git pull failed"}}

	result := usecases.RefreshInventoryIfPossible(dir, refresher)

	require.NotNil(t, result)
	assert.Equal(t, false, (*result)[0])
	assert.Equal(t, "git pull failed", (*result)[1])
}

// --- FindTargetsByClient ---

func TestFindTargetsByClient_ReturnsEmptyWhenNoMatch(t *testing.T) {
	catalog := &stubCatalog{targets: []domain.ClusterTarget{
		buildTestTarget("acme", "host1", "10.0.0.1"),
	}}
	targets, err := usecases.FindTargetsByClient("other", "", catalog)
	require.NoError(t, err)
	assert.Empty(t, targets)
}

func TestFindTargetsByClient_ReturnsOnlyMatchingClient(t *testing.T) {
	catalog := &stubCatalog{targets: []domain.ClusterTarget{
		buildTestTarget("acme", "host1", "10.0.0.1"),
		buildTestTarget("acme", "host2", "10.0.0.2"),
		buildTestTarget("beta", "host3", "10.0.0.3"),
	}}
	targets, err := usecases.FindTargetsByClient("acme", "", catalog)
	require.NoError(t, err)
	require.Len(t, targets, 2)
	for _, tgt := range targets {
		assert.Equal(t, "acme", tgt.Company())
	}
}

func TestFindTargetsByClient_EmptyCatalogReturnsEmpty(t *testing.T) {
	catalog := &stubCatalog{}
	targets, err := usecases.FindTargetsByClient("acme", "", catalog)
	require.NoError(t, err)
	assert.Empty(t, targets)
}
