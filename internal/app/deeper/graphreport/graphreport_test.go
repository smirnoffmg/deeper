package graphreport

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender_ProducesValidDocument(t *testing.T) {
	html, err := Render(
		[]Node{{ID: 1, Label: "root.com", Type: "domain"}},
		[]Edge{},
	)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(strings.TrimSpace(html), "<!DOCTYPE html>"))
	assert.Contains(t, html, `id="graph-data"`)
}

func TestRender_EmbedsVisNetworkLibrary(t *testing.T) {
	html, err := Render([]Node{{ID: 1, Label: "root.com", Type: "domain"}}, nil)
	require.NoError(t, err)
	assert.Contains(t, html, "vis-network")
	assert.Contains(t, html, "new vis.Network(")
}

func TestRender_EmptyGraph(t *testing.T) {
	html, err := Render(nil, nil)
	require.NoError(t, err)
	assert.Contains(t, html, `id="graph-data"`)
}

func TestRender_EmbedsGraphDataAsRoundTrippableJSON(t *testing.T) {
	nodes := []Node{
		{ID: 1, Label: "root.com", Type: "domain"},
		{ID: 2, Label: "leaf.root.com", Type: "subdomain"},
	}
	edges := []Edge{
		{From: 1, To: 2, Label: "subdomain_finder"},
	}

	html, err := Render(nodes, edges)
	require.NoError(t, err)

	payload := extractGraphDataJSON(t, html)

	var got graphData
	require.NoError(t, json.Unmarshal([]byte(payload), &got))
	assert.Equal(t, nodes, got.Nodes)
	assert.Equal(t, edges, got.Edges)
}

// TestRender_EscapesMaliciousValues proves a trace value scraped from an
// untrusted source (e.g. a bio field) cannot break out of the embedded
// <script> block and execute as markup.
func TestRender_EscapesMaliciousValues(t *testing.T) {
	malicious := `</script><script>alert(1)</script>`
	nodes := []Node{{ID: 1, Label: malicious, Type: "username"}}

	html, err := Render(nodes, nil)
	require.NoError(t, err)

	assert.NotContains(t, html, "<script>alert(1)</script>")

	payload := extractGraphDataJSON(t, html)
	var got graphData
	require.NoError(t, json.Unmarshal([]byte(payload), &got))
	require.Len(t, got.Nodes, 1)
	assert.Equal(t, malicious, got.Nodes[0].Label)
}

func extractGraphDataJSON(t *testing.T, html string) string {
	t.Helper()
	const marker = `id="graph-data">`
	start := strings.Index(html, marker)
	require.NotEqual(t, -1, start, "graph-data script tag not found")
	start += len(marker)
	end := strings.Index(html[start:], "</script>")
	require.NotEqual(t, -1, end, "closing script tag not found")
	return html[start : start+end]
}
