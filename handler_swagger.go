package resty

import (
	"encoding/json"
	"net/http"
	"strings"
)

// swaggerHTMLEndpoint is one (path, method) for the try-it UI.
type swaggerHTMLEndpoint struct {
	Path        string            `json:"path"`
	Method      string            `json:"method"`
	Summary     string            `json:"summary"`
	Description string            `json:"description"`
	BodyExample string            `json:"bodyExample"`
	Responses   map[int]string    `json:"responses"`
	QueryParams map[string]string `json:"queryParams,omitempty"` // GET: param name (snake_case) -> example value
}

func (r *router) buildSwaggerHTMLEndpoints() []swaggerHTMLEndpoint {
	var out []swaggerHTMLEndpoint
	for _, doc := range r.GetAppDocRoutes() {
		responses := doc.Responses
		if responses == nil {
			responses = map[int]string{200: `{"success":true,"message":""}`}
		}
		bodyExample := doc.BodyExample
		if bodyExample == "" {
			bodyExample = "{}"
		}
		for _, method := range doc.Methods {
			queryParams := doc.QueryParams
			if method != "GET" {
				queryParams = nil
			}
			out = append(out, swaggerHTMLEndpoint{
				Path:        doc.Path,
				Method:      method,
				Summary:     doc.Summary,
				Description: doc.Description,
				BodyExample: bodyExample,
				Responses:   responses,
				QueryParams: queryParams,
			})
		}
	}
	return out
}

func (r *router) serveSwaggerHTML(w http.ResponseWriter, _ *http.Request) {
	endpoints := r.buildSwaggerHTMLEndpoints()
	data, _ := json.Marshal(endpoints)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(swaggerHTMLPage(string(data))))
}

func swaggerHTMLPage(endpointsJSON string) string {
	escaped := strings.ReplaceAll(endpointsJSON, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "`", "\\`")
	escaped = strings.ReplaceAll(escaped, "</", "<\\/")
	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>API — Try endpoints</title>
  <style>
    :root { --bg: #0f0f12; --card: #18181c; --border: #2a2a30; --text: #e4e4e7; --muted: #71717a; --accent: #3b82f6; --accent-hover: #2563eb; --success: #22c55e; --error: #ef4444; }
    * { box-sizing: border-box; }
    body { font-family: 'JetBrains Mono', 'SF Mono', ui-monospace, monospace; background: var(--bg); color: var(--text); margin: 0; padding: 24px; line-height: 1.5; }
    h1 { font-size: 1.5rem; font-weight: 600; margin: 0 0 24px 0; }
    .endpoint { background: var(--card); border: 1px solid var(--border); border-radius: 10px; padding: 20px; margin-bottom: 20px; }
    .endpoint-header { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; flex-wrap: wrap; }
    .method { font-weight: 700; padding: 4px 10px; border-radius: 6px; font-size: 0.85rem; }
    .method-GET { background: #166534; color: #bbf7d0; }
    .method-POST { background: #1e40af; color: #bfdbfe; }
    .method-PUT { background: #854d0e; color: #fef08a; }
    .method-PATCH { background: #713f12; color: #fed7aa; }
    .method-DELETE { background: #991b1b; color: #fecaca; }
    .path { font-size: 1rem; word-break: break-all; }
    .summary { color: var(--muted); font-size: 0.9rem; margin-top: 4px; }
    .row { margin-bottom: 12px; }
    .row label { display: block; font-size: 0.8rem; color: var(--muted); margin-bottom: 4px; }
    .row input, .row textarea { width: 100%; padding: 10px 12px; background: var(--bg); border: 1px solid var(--border); border-radius: 6px; color: var(--text); font-family: inherit; font-size: 0.9rem; }
    .row textarea { min-height: 80px; resize: vertical; }
    .btn { padding: 10px 20px; background: var(--accent); color: #fff; border: none; border-radius: 6px; font-family: inherit; font-weight: 600; cursor: pointer; }
    .btn:hover { background: var(--accent-hover); }
    .responses { margin-top: 12px; font-size: 0.85rem; }
    .responses h4 { margin: 0 0 8px 0; color: var(--muted); font-size: 0.8rem; }
    .responses pre { background: var(--bg); border: 1px solid var(--border); border-radius: 6px; padding: 10px; overflow-x: auto; margin: 4px 0; font-size: 0.8rem; }
    .response-output { margin-top: 12px; padding: 12px; border-radius: 6px; font-size: 0.85rem; white-space: pre-wrap; word-break: break-all; }
    .response-output.success { background: #14532d; border: 1px solid #166534; }
    .response-output.error { background: #450a0a; border: 1px solid #991b1b; }
  </style>
</head>
<body>
  <h1>API — Try endpoints</h1>
  <div id="root"></div>
  <script>
    const endpoints = ` + "`" + escaped + "`" + `;
    const data = JSON.parse(endpoints);
    const root = document.getElementById('root');
    data.forEach(function(ep, idx) {
      const card = document.createElement('div');
      card.className = 'endpoint';
      card.innerHTML =
        '<div class="endpoint-header">' +
          '<span class="method method-' + ep.method + '">' + ep.method + '</span>' +
          '<span class="path">' + escapeHtml(ep.path) + '</span>' +
        '</div>' +
        (ep.summary ? '<div class="summary">' + escapeHtml(ep.summary) + '</div>' : '') +
        '<div class="row"><label>Domain</label><input type="text" placeholder="https://localhost:8080" class="domain" value=""></div>' +
        (ep.queryParams && Object.keys(ep.queryParams).length ? '<div class="query-params row"><label>Query params (snake_case)</label>' + formatQueryParams(ep.queryParams) + '</div>' : '') +
        '<div class="row"><label>Headers (one per line: Key: value)</label><textarea class="headers" placeholder="Content-Type: application/json&#10;Authorization: Bearer token">Content-Type: application/json</textarea></div>' +
        (ep.method !== 'GET' && ep.method !== 'HEAD' ? '<div class="row"><label>Request body (JSON)</label><textarea class="body">' + escapeHtml(ep.bodyExample) + '</textarea></div>' : '') +
        '<div class="responses"><h4>Possible responses</h4>' + formatResponses(ep.responses) + '</div>' +
        '<br><button class="btn send-btn" type="button">Send request</button>' +
        '<div class="response-output" style="display:none" role="status"></div>';
      const out = card.querySelector('.response-output');
      const domainIn = card.querySelector('.domain');
      const headersIn = card.querySelector('.headers');
      const bodyIn = card.querySelector('.body');
      card.querySelector('.send-btn').addEventListener('click', function() {
        const domain = (domainIn.value || '').replace(/\/$/, '');
        let url = domain + ep.path;
        if (ep.queryParams && Object.keys(ep.queryParams).length) {
          const q = card.querySelectorAll('.query-params input[data-param]');
          const params = [];
          q.forEach(function(inp) {
            const key = inp.getAttribute('data-param');
            const val = (inp.value || '').trim();
            if (key && val) params.push(encodeURIComponent(key) + '=' + encodeURIComponent(val));
          });
          if (params.length) url += '?' + params.join('&');
        }
        const rawHeaders = (headersIn.value || '').trim().split('\n').filter(Boolean);
        const headers = {};
        rawHeaders.forEach(function(line) {
          const i = line.indexOf(':');
          if (i > 0) headers[line.slice(0, i).trim()] = line.slice(i + 1).trim();
        });
        const body = (ep.method !== 'GET' && ep.method !== 'HEAD') && bodyIn ? bodyIn.value : undefined;
        out.style.display = 'block';
        out.className = 'response-output';
        out.textContent = 'Sending…';
        fetch(url, { method: ep.method, headers: headers, body: body })
          .then(function(res) {
            return res.text().then(function(text) {
              out.textContent = 'Status: ' + res.status + ' ' + res.statusText + '\n\n' + text;
              out.classList.add(res.ok ? 'success' : 'error');
            });
          })
          .catch(function(err) {
            out.textContent = 'Error: ' + err.message;
            out.classList.add('error');
          });
      });
      root.appendChild(card);
    });
    function escapeHtml(s) {
      if (!s) return '';
      const div = document.createElement('div');
      div.textContent = s;
      return div.innerHTML;
    }
    function formatResponses(responses) {
      if (!responses || typeof responses !== 'object') return '';
      let html = '';
      const codes = Object.keys(responses).sort();
      for (let i = 0; i < codes.length; i++) {
        const code = codes[i];
        const body = responses[code];
        html += '<pre><strong>' + code + '</strong>: ' + escapeHtml(typeof body === 'string' ? body : JSON.stringify(body)) + '</pre>';
      }
      return html || '<pre>200: {"success":true,"message":""}</pre>';
    }
    function formatQueryParams(queryParams) {
      if (!queryParams || typeof queryParams !== 'object') return '';
      const keys = Object.keys(queryParams).sort();
      let html = '<div class="query-params-list">';
      for (let i = 0; i < keys.length; i++) {
        const key = keys[i];
        const val = queryParams[key] || '';
        html += '<div class="row"><label>' + escapeHtml(key) + '</label><input type="text" data-param="' + escapeHtml(key) + '" value="' + escapeHtml(val) + '" placeholder=""></div>';
      }
      html += '</div>';
      return html;
    }
  </script>
</body>
</html>`
}
