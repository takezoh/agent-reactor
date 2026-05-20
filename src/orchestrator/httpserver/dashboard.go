package httpserver

import (
	"html/template"
	"net/http"
	"time"

	"github.com/takezoh/agent-roost/orchestrator/scheduler"
)

var dashboardTmpl = template.Must(template.New("dashboard").Parse(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>Orchestrator Dashboard</title>
<style>
body{font-family:monospace;padding:1rem;background:#0d1117;color:#c9d1d9}
h1{color:#58a6ff}h2{color:#79c0ff;border-bottom:1px solid #30363d;padding-bottom:4px}
table{border-collapse:collapse;width:100%}
th,td{text-align:left;padding:4px 8px;border:1px solid #30363d}
th{background:#161b22;color:#8b949e}
tr:nth-child(even){background:#161b22}
.ts{color:#8b949e;font-size:.85em}
.empty{color:#6e7681}
</style>
</head>
<body>
<h1>Orchestrator Dashboard</h1>
<p class="ts">Generated at: {{.GeneratedAt}}</p>

<h2>Running ({{len .Running}})</h2>
{{if .Running}}
<table>
<tr><th>Identifier</th><th>State</th><th>Session ID</th><th>Started At</th><th>Last Event</th><th>Tokens</th></tr>
{{range .Running}}
<tr>
  <td>{{.IssueIdentifier}}</td>
  <td>{{.State}}</td>
  <td>{{.SessionID}}</td>
  <td class="ts">{{.StartedAt}}</td>
  <td>{{.LastEvent}}</td>
  <td>{{.Tokens.TotalTokens}}</td>
</tr>
{{end}}
</table>
{{else}}<p class="empty">No running issues.</p>{{end}}

<h2>Retrying ({{len .Retrying}})</h2>
{{if .Retrying}}
<table>
<tr><th>Identifier</th><th>Attempt</th><th>Due At</th><th>Error</th></tr>
{{range .Retrying}}
<tr>
  <td>{{.IssueIdentifier}}</td>
  <td>{{.Attempt}}</td>
  <td class="ts">{{.DueAt}}</td>
  <td>{{.Error}}</td>
</tr>
{{end}}
</table>
{{else}}<p class="empty">No retrying issues.</p>{{end}}

{{if .CodexTotals}}
<h2>Lifetime Totals</h2>
<table>
<tr><th>Input Tokens</th><th>Output Tokens</th><th>Total Tokens</th><th>Runtime (s)</th></tr>
<tr>
  <td>{{.CodexTotals.InputTokens}}</td>
  <td>{{.CodexTotals.OutputTokens}}</td>
  <td>{{.CodexTotals.TotalTokens}}</td>
  <td>{{printf "%.1f" .CodexTotals.SecondsRunning}}</td>
</tr>
</table>
{{end}}
</body>
</html>`))

func renderDashboard(w http.ResponseWriter, snap scheduler.StateSnapshot) {
	data := projectState(snap, time.Now())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = dashboardTmpl.Execute(w, data)
}
