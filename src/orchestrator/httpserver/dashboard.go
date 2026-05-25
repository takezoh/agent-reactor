package httpserver

import "net/http"

// dashboardHTML is a static, dependency-free shell. It carries no live data:
// the page fetches the JSON REST API (/api/v1/state) client-side and renders
// tables in the browser, and POSTs /api/v1/refresh for a manual refresh. All
// dynamic values are inserted via textContent, so issue titles/messages cannot
// inject markup (XSS-safe).
const dashboardHTML = `<!DOCTYPE html>
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
.err{color:#f85149}
button{font-family:monospace;background:#21262d;color:#c9d1d9;border:1px solid #30363d;border-radius:6px;padding:2px 10px;cursor:pointer}
button:hover{background:#30363d}
</style>
</head>
<body>
<h1>Orchestrator Dashboard</h1>
<p class="ts">Generated at: <span id="generated-at">&mdash;</span> &middot; Fetched: <span id="fetched-at">&mdash;</span>
  <button id="refresh">Refresh now</button>
  <label><input type="checkbox" id="auto" checked> auto (5s)</label>
  <span id="err" class="err"></span></p>

<h2>Running (<span id="running-count">0</span>)</h2>
<table id="running-table"><thead><tr><th>Identifier</th><th>State</th><th>Session ID</th><th>Started At</th><th>Last Event</th><th>Tokens</th></tr></thead>
<tbody id="running-body"></tbody></table>
<p id="running-empty" class="empty" hidden>No running issues.</p>

<h2>Retrying (<span id="retrying-count">0</span>)</h2>
<table id="retrying-table"><thead><tr><th>Identifier</th><th>Attempt</th><th>Due At</th><th>Error</th></tr></thead>
<tbody id="retrying-body"></tbody></table>
<p id="retrying-empty" class="empty" hidden>No retrying issues.</p>

<h2>Lifetime Totals</h2>
<table><thead><tr><th>Input</th><th>Output</th><th>Total</th><th>Runtime (s)</th></tr></thead>
<tbody><tr><td id="t-in">0</td><td id="t-out">0</td><td id="t-total">0</td><td id="t-runtime">0.0</td></tr></tbody></table>

<h2>Rate Limits</h2>
<p id="rate-limits" class="empty">none</p>

<script>
const $ = id => document.getElementById(id);
function cell(v){ const td=document.createElement('td'); td.textContent = (v==null?'':String(v)); return td; }
function toggle(name, n){ $(name+'-table').hidden = (n===0); $(name+'-empty').hidden = (n!==0); }
function render(d){
  $('generated-at').textContent = d.generated_at || '—';
  $('fetched-at').textContent = new Date().toISOString();
  $('err').textContent = '';
  const running = d.running || [], retrying = d.retrying || [], counts = d.counts || {};
  $('running-count').textContent = (counts.running != null ? counts.running : running.length);
  $('retrying-count').textContent = (counts.retrying != null ? counts.retrying : retrying.length);

  const rb = $('running-body'); rb.replaceChildren();
  running.forEach(r => { const tr=document.createElement('tr');
    tr.append(cell(r.issue_identifier), cell(r.state), cell(r.session_id), cell(r.started_at), cell(r.last_event), cell((r.tokens||{}).total_tokens));
    rb.append(tr); });
  toggle('running', running.length);

  const tb = $('retrying-body'); tb.replaceChildren();
  retrying.forEach(r => { const tr=document.createElement('tr');
    tr.append(cell(r.issue_identifier), cell(r.attempt), cell(r.due_at), cell(r.error));
    tb.append(tr); });
  toggle('retrying', retrying.length);

  const t = d.codex_totals || {};
  $('t-in').textContent = (t.input_tokens != null ? t.input_tokens : 0);
  $('t-out').textContent = (t.output_tokens != null ? t.output_tokens : 0);
  $('t-total').textContent = (t.total_tokens != null ? t.total_tokens : 0);
  $('t-runtime').textContent = Number(t.seconds_running || 0).toFixed(1);

  const rl = d.rate_limits;
  $('rate-limits').textContent = rl
    ? ('primary ' + rl.primary_used_percent + '% · secondary ' + rl.secondary_used_percent + '%')
    : 'none';
}
async function load(){
  try {
    const r = await fetch('/api/v1/state', {headers:{'Accept':'application/json'}});
    if(!r.ok) throw new Error('HTTP ' + r.status);
    render(await r.json());
  } catch(e){ $('err').textContent = 'fetch error: ' + e.message; }
}
async function refresh(){
  try { await fetch('/api/v1/refresh', {method:'POST'}); } catch(e){ /* surfaced on next load */ }
  load();
}
let timer = null;
function setAuto(on){ if(timer){ clearInterval(timer); timer=null; } if(on){ timer=setInterval(load, 5000); } }
document.addEventListener('DOMContentLoaded', () => {
  $('refresh').addEventListener('click', refresh);
  $('auto').addEventListener('change', e => setAuto(e.target.checked));
  setAuto(true);
  load();
});
</script>
</body>
</html>`

// renderDashboard serves the static dashboard shell. It carries no scheduler
// state — the page consumes the JSON REST API client-side — so this handler
// does not read from the scheduler.
func renderDashboard(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(dashboardHTML))
}
