package main

import (
	"net/http"
)

func (s *server) handleDashboardPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(dashboardHTML))
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>Nougat — API usage</title>
  <style>
    :root { font-family: system-ui, sans-serif; background: #0f1419; color: #e6edf3; }
    body { max-width: 56rem; margin: 2rem auto; padding: 0 1rem; }
    h1 { font-size: 1.25rem; font-weight: 600; }
    .muted { color: #8b949e; font-size: 0.875rem; }
    label { display: block; margin: 1rem 0 0.35rem; font-size: 0.8rem; text-transform: uppercase; letter-spacing: 0.04em; color: #8b949e; }
    input { width: 100%; max-width: 32rem; padding: 0.5rem 0.65rem; border-radius: 6px; border: 1px solid #30363d; background: #161b22; color: #e6edf3; }
    button { margin-top: 0.75rem; padding: 0.45rem 1rem; border-radius: 6px; border: 1px solid #388bfd; background: #21262d; color: #58a6ff; cursor: pointer; }
    button:hover { background: #30363d; }
    #err { color: #f85149; margin-top: 1rem; white-space: pre-wrap; }
    .grid { display: grid; gap: 1rem; margin-top: 1.5rem; grid-template-columns: repeat(auto-fit, minmax(9rem, 1fr)); }
    .card { background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 1rem; }
    .card b { display: block; font-size: 1.5rem; font-weight: 600; }
    .card span { font-size: 0.75rem; color: #8b949e; }
    table { width: 100%; border-collapse: collapse; margin-top: 1.5rem; font-size: 0.8125rem; }
    th, td { text-align: left; padding: 0.5rem 0.65rem; border-bottom: 1px solid #21262d; vertical-align: top; }
    th { color: #8b949e; font-weight: 500; }
    code { font-size: 0.8em; background: #21262d; padding: 0.1rem 0.35rem; border-radius: 4px; }
  </style>
</head>
<body>
  <h1>Nougat usage dashboard</h1>
  <p class="muted">Paste the same <code>sk-nougat-…</code> API key you use for OpenAI-compatible clients. It is only sent to this server in <code>Authorization</code> headers from your browser (not stored by this page).</p>

  <label for="key">API key</label>
  <input id="key" type="password" autocomplete="off" placeholder="sk-nougat-…"/>

  <div>
    <button type="button" id="load">Load stats</button>
  </div>
  <p id="err"></p>

  <section id="summary" hidden>
    <div class="grid">
      <div class="card"><b id="tot-req">0</b><span>Total requests</span></div>
      <div class="card"><b id="tot-prompt">0</b><span>Prompt tokens (est.)</span></div>
      <div class="card"><b id="tot-comp">0</b><span>Completion tokens (est.)</span></div>
      <div class="card"><b id="key-label">—</b><span>Label</span></div>
    </div>
    <p class="muted" style="margin-top:1rem">Key <code id="key-id"></code> · last request <span id="last-at">—</span></p>
  </section>

  <section id="hist-wrap" hidden>
    <h2 style="font-size:1rem;margin-top:2rem">Recent requests</h2>
    <table>
      <thead><tr><th>When</th><th>Route</th><th>Status</th><th>Model</th><th>Tokens in/out</th><th>ms</th></tr></thead>
      <tbody id="hist-body"></tbody>
    </table>
  </section>

<script>
(async function() {
  const $ = (id) => document.getElementById(id);
  const errEl = $('err');

  function authHdr() {
    const k = $('key').value.trim();
    if (!k) throw new Error('Enter an API key.');
    return { 'Authorization': 'Bearer ' + k };
  }

  $('load').onclick = async () => {
    errEl.textContent = '';
    $('summary').hidden = true;
    $('hist-wrap').hidden = true;
    try {
      const h = authHdr();
      const [sum, hist] = await Promise.all([
        fetch('/dashboard/api/summary', { headers: h }).then(async r => {
          const t = await r.text();
          if (!r.ok) throw new Error(t || r.status);
          return JSON.parse(t);
        }),
        fetch('/dashboard/api/history?limit=100', { headers: h }).then(async r => {
          const t = await r.text();
          if (!r.ok) throw new Error(t || r.status);
          return JSON.parse(t);
        })
      ]);

      $('tot-req').textContent = sum.totals.total_requests ?? 0;
      $('tot-prompt').textContent = sum.totals.total_prompt_tokens ?? 0;
      $('tot-comp').textContent = sum.totals.total_completion_tokens ?? 0;
      $('key-label').textContent = sum.label || '—';
      $('key-id').textContent = sum.key_id || '—';
      const lu = sum.totals.last_request_at;
      $('last-at').textContent = lu ? new Date(lu).toLocaleString() : '—';
      $('summary').hidden = false;

      const tb = $('hist-body');
      tb.innerHTML = '';
      for (const e of (hist.events || [])) {
        const tr = document.createElement('tr');
        const tok = (e.prompt_tokens||0) + ' / ' + (e.completion_tokens||0);
        tr.innerHTML = '<td>' + new Date(e.at).toLocaleString() + '</td>' +
          '<td><code>' + (e.method||'') + ' ' + (e.path||'') + '</code></td>' +
          '<td>' + e.http_status + '</td>' +
          '<td>' + (e.model || '—') + '</td>' +
          '<td>' + tok + '</td>' +
          '<td>' + (e.duration_ms ?? '—') + '</td>';
        tb.appendChild(tr);
      }
      $('hist-wrap').hidden = (hist.events||[]).length === 0;
    } catch (e) {
      errEl.textContent = e.message || String(e);
    }
  };
})();
</script>
</body>
</html>
`
