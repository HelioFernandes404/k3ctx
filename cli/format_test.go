package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systemframe/k3ctx/internal/domain"
)

// --- clientPageToMap ---

func TestClientPageToMap_MapsItemsAndPagination(t *testing.T) {
	next := "acme"
	page := domain.ClientPage{
		Items: []domain.ClientSummary{
			{Client: "acme", HostCount: 3},
			{Client: "beta", HostCount: 1},
		},
		Page: domain.PageInfo{
			Limit:      2,
			Returned:   2,
			Total:      5,
			HasMore:    true,
			NextCursor: &next,
		},
	}

	got := clientPageToMap(page)

	items := got["items"].([]map[string]any)
	require.Len(t, items, 2)
	assert.Equal(t, "acme", items[0]["client"])
	assert.Equal(t, 3, items[0]["host_count"])

	p := got["page"].(map[string]any)
	assert.Equal(t, 2, p["limit"])
	assert.Equal(t, 5, p["total"])
	assert.Equal(t, true, p["has_more"])
	assert.Equal(t, "acme", p["next_cursor"])
}

func TestClientPageToMap_NilCursorBecomesJSONNull(t *testing.T) {
	page := domain.ClientPage{
		Items: []domain.ClientSummary{},
		Page:  domain.PageInfo{HasMore: false, NextCursor: nil},
	}
	got := clientPageToMap(page)
	p := got["page"].(map[string]any)
	assert.Nil(t, p["next_cursor"])
}

// --- hostItemsToMaps ---

func TestHostItemsToMaps_NilOptionalFieldsBecomeNilAny(t *testing.T) {
	items := []domain.HostRecord{
		{Client: "acme", HostName: "prod", ContextName: "acme-prod", Group: "k3s"},
	}
	got := hostItemsToMaps(items)
	require.Len(t, got, 1)
	assert.Equal(t, "acme", got[0]["client"])
	assert.Nil(t, got[0]["addr"])
	assert.Nil(t, got[0]["systemframe_id"])
	assert.Nil(t, got[0]["status"])
}

func TestHostItemsToMaps_DereferencesSetOptionalFields(t *testing.T) {
	addr := "10.0.0.10"
	sfID := "sf-1042"
	status := "Connected"
	items := []domain.HostRecord{
		{Client: "acme", HostName: "prod", ContextName: "acme-prod",
			Addr: &addr, SystemframeID: &sfID, Status: &status},
	}
	got := hostItemsToMaps(items)
	assert.Equal(t, "10.0.0.10", got[0]["addr"])
	assert.Equal(t, "sf-1042", got[0]["systemframe_id"])
	assert.Equal(t, "Connected", got[0]["status"])
}

// --- hostPageToMap ---

func TestHostPageToMap_MapsItemsAndPagination(t *testing.T) {
	addr := "10.0.0.1"
	next := "acme-prod"
	page := domain.HostPage{
		Items: []domain.HostRecord{
			{Client: "acme", HostName: "prod", ContextName: "acme-prod", Addr: &addr},
		},
		Page: domain.PageInfo{
			Limit:      1,
			Returned:   1,
			Total:      2,
			HasMore:    true,
			NextCursor: &next,
		},
	}

	got := hostPageToMap(page)
	items := got["items"].([]map[string]any)
	require.Len(t, items, 1)
	assert.Equal(t, "acme-prod", items[0]["context_name"])

	p := got["page"].(map[string]any)
	assert.Equal(t, true, p["has_more"])
	assert.Equal(t, "acme-prod", p["next_cursor"])
}

// --- printHostLine ---

func TestPrintHostLine_IncludesContextAddrAndSystemframeID(t *testing.T) {
	addr := "10.0.0.10"
	sfID := "sf-1042"
	rec := domain.HostRecord{ContextName: "acme-prod", Addr: &addr, SystemframeID: &sfID}

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	printHostLine(cmd, rec)
	line := buf.String()
	assert.Contains(t, line, "acme-prod")
	assert.Contains(t, line, "10.0.0.10")
	assert.Contains(t, line, "sf-1042")
	assert.NotContains(t, line, "[")
}

func TestPrintHostLine_PrependsStatusWhenSet(t *testing.T) {
	status := "Connected"
	rec := domain.HostRecord{ContextName: "acme-prod", Status: &status}

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	printHostLine(cmd, rec)
	assert.Contains(t, buf.String(), "[Connected]")
}

func TestPrintHostLine_HandlesNilOptionalFields(t *testing.T) {
	rec := domain.HostRecord{ContextName: "acme-prod"}

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	printHostLine(cmd, rec)
	assert.Contains(t, buf.String(), "acme-prod")
}
