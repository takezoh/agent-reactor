// Web client logic for agent-reactor's server. Loaded as a same-origin script so
// a strict Content-Security-Policy (script-src 'self') applies.
//
// The bearer token is read from the URL *fragment* (#token=...), not the query
// string: fragments are never sent to the server (no access-log entry) and are
// stripped from the Referer header, so the token does not leak. The token
// authenticates the REST API via an Authorization header. WebSocket connections
// — which cannot carry headers from a browser — use a short-lived, single-use
// ticket fetched over that authenticated API, so the token never appears in any
// URL.
const token = new URLSearchParams(location.hash.slice(1)).get('token') || '';
const authHeaders = { 'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json' };
const term = new Terminal({ fontSize: 13, cursorBlink: true, theme: { background: '#111' } });
const fit = new FitAddon.FitAddon();
term.loadAddon(fit);
term.open(document.getElementById('term'));
fit.fit();

let ws = null, activeID = null;
const status = (s) => document.getElementById('status').textContent = s;

function logEv(k, text) {
  const d = document.createElement('div');
  d.className = 'ev';
  // Build with textContent, never innerHTML: k/text derive from server control
  // events whose fields can be influenced by OSC sequences a process emits into
  // the terminal, so treating them as HTML would be an XSS sink.
  const tag = document.createElement('span');
  tag.className = 'k';
  tag.textContent = '[' + k + ']';
  d.appendChild(tag);
  d.appendChild(document.createTextNode(' ' + text));
  document.getElementById('evlist').prepend(d);
}

async function api(method, path, body) {
  const r = await fetch(path, { method, headers: authHeaders, body: body ? JSON.stringify(body) : undefined });
  if (!r.ok) throw new Error(await r.text());
  return r.status === 204 ? null : r.json();
}

async function refresh() {
  const sessions = await api('GET', '/api/sessions');
  const list = document.getElementById('list');
  list.innerHTML = '';
  for (const s of sessions) {
    const el = document.createElement('div');
    el.className = 'sess' + (s.id === activeID ? ' active' : '');
    el.textContent = s.id + ' · ' + s.command;
    const stop = document.createElement('button');
    stop.textContent = '✕';
    stop.onclick = async (e) => { e.stopPropagation(); await api('DELETE', '/api/sessions/' + s.id); refresh(); };
    el.onclick = () => attach(s.id);
    el.appendChild(document.createTextNode(' '));
    el.appendChild(stop);
    list.appendChild(el);
  }
}

function send(o) { if (ws && ws.readyState === 1) ws.send(JSON.stringify(o)); }
function sendResize() { send({ k: 'r', cols: term.cols, rows: term.rows }); }

async function attach(id) {
  if (ws) { ws.close(); ws = null; }
  activeID = id;
  term.reset();
  // Mint a single-use ticket over the authenticated API, then carry it (not the
  // bearer token) in the WebSocket URL.
  let ticket;
  try {
    ({ ticket } = await api('POST', '/api/ws-ticket'));
  } catch (err) {
    status('error: ' + err.message);
    return;
  }
  // A newer attach() may have superseded us while we awaited the ticket; bail so
  // we don't wire the terminal to a stale session or leave two live sockets.
  if (activeID !== id) return;
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  const sock = new WebSocket(`${proto}://${location.host}/ws?session=${encodeURIComponent(id)}&ticket=${encodeURIComponent(ticket)}`);
  // A concurrent attach() (e.g. a double-click on the same session) may have
  // opened a socket while we awaited the ticket; close it before replacing so we
  // never orphan a socket and leak its server-side subscriber/goroutines.
  if (ws) ws.close();
  ws = sock;
  // Guard every handler on `ws === sock`: a superseded socket (closed when the
  // user switched sessions) fires its callbacks asynchronously and would
  // otherwise clobber the live socket's status/terminal.
  sock.onopen = () => { if (ws !== sock) return; status('attached ' + id); sendResize(); refresh(); };
  sock.onclose = () => { if (ws !== sock) return; status('detached'); };
  sock.onmessage = (e) => {
    if (ws !== sock) return;
    if (typeof e.data !== 'string') return; // server only sends text frames
    let msg;
    try { msg = JSON.parse(e.data); } catch { return; } // skip one bad frame, don't kill the stream
    if (Array.isArray(msg)) { if (msg[1] === 'o') term.write(msg[2]); return; }
    logEv(msg.k + (msg.code ? ' ' + msg.code : ''), msg.data || '');
  };
}

term.onData((d) => send({ k: 'i', d }));
term.onResize(() => sendResize());
addEventListener('resize', () => fit.fit());
document.getElementById('create').onclick = async () => {
  try {
    const info = await api('POST', '/api/sessions', {
      command: document.getElementById('cmd').value,
      project: document.getElementById('project').value,
      cols: term.cols, rows: term.rows,
    });
    await refresh();
    attach(info.id);
  } catch (err) { status('error: ' + err.message); }
};
// Surface the first load failure (e.g. a bad/expired #token= → 401) in the
// status line instead of dying as an unhandled rejection with a blank page.
refresh().catch((err) => status('error: ' + err.message));
