// Package graphreport renders a scan's discovery graph as a self-contained,
// interactive HTML document. The vis-network library (see vendor/NOTICE.md)
// is embedded inline, so the report has no CDN dependency and works fully
// offline.
package graphreport

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
)

//go:embed graph.html.tmpl vendor/vis-network.min.js
var templateFS embed.FS

var tmpl = template.Must(template.ParseFS(templateFS, "graph.html.tmpl"))

// visNetworkJS holds the vendored vis-network standalone UMD build (see
// vendor/NOTICE.md) so the generated report stays fully offline/self
// -contained — no CDN fetch at view time.
var visNetworkJS = func() template.JS {
	b, err := templateFS.ReadFile("vendor/vis-network.min.js")
	if err != nil {
		panic(err) // embedded at build time; a read failure means the build is broken
	}
	return template.JS(b)
}()

// Node is a graph vertex ready for rendering. Label is untrusted (it may
// originate from scraped, attacker-influenced data) and must only ever be
// embedded via the JSON payload, never interpolated directly into HTML/JS.
type Node struct {
	ID    int64  `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"`
}

// Edge is a directed graph edge; Label is the plugin that produced it.
type Edge struct {
	From  int64  `json:"from"`
	To    int64  `json:"to"`
	Label string `json:"label"`
}

type graphData struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// Render produces a complete standalone HTML document visualizing the given
// nodes and edges as an interactive node-link diagram.
func Render(nodes []Node, edges []Edge) (string, error) {
	if nodes == nil {
		nodes = []Node{}
	}
	if edges == nil {
		edges = []Edge{}
	}

	// json.Marshal HTML-escapes '<', '>' and '&' by default, which is what
	// makes it safe to drop straight into a <script> block below: an
	// attacker-controlled value like "</script><script>..." is encoded as
	// "</script>...", so it can neither close the surrounding
	// script tag nor be interpreted as markup by the HTML parser.
	payload, err := json.Marshal(graphData{Nodes: nodes, Edges: edges})
	if err != nil {
		return "", fmt.Errorf("failed to marshal graph data: %w", err)
	}

	var buf bytes.Buffer
	data := struct {
		GraphDataJSON template.JS
		VisNetworkJS  template.JS
	}{
		GraphDataJSON: template.JS(payload),
		VisNetworkJS:  visNetworkJS,
	}
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render graph template: %w", err)
	}
	return buf.String(), nil
}
