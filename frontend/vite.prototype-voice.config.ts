// Local-only voice study. All API routes are synthetic; there is no backend proxy.
import { defineConfig, mergeConfig } from 'vite';
import { createReadStream } from 'node:fs';
import { fileURLToPath } from 'node:url';
import e2e from './vite.e2e.config';
const date = '2026-11-20T00:00:00Z';
const user = { id: 'voice-study', auth0_id: 'auth0:browser-test', name: 'Alex Test', nickname: 'Alex', email: 'alex@seed.invalid', role: 'admin', is_player: true, membership_status: 'approved', created_at: date, updated_at: date };
export default mergeConfig(e2e, defineConfig({
  plugins: [{
    name: 'voice-study-fixtures',
    transformIndexHtml(html) {
      return html.replace('<head>', '<head><script>sessionStorage.setItem("e2e:authenticated", "true")</script>');
    },
    configureServer(server) {
      // ONNX imports its runtime dynamically. Serve the generated modules as
      // static files before Vite rejects imports from the public directory.
      server.middlewares.use('/prototype-voice-assets', (req, res, next) => {
        const file = req.url?.split('?')[0].slice(1) ?? '';
        if (!/^(ort-wasm[\w.-]+\.(?:mjs|wasm)|silero_vad_v5\.onnx|vad\.worklet\.bundle\.min\.js)$/.test(file)) return next();
        const path = fileURLToPath(new URL(`./public/prototype-voice-assets/${file}`, import.meta.url));
        res.setHeader('Content-Type', file.endsWith('.wasm') ? 'application/wasm' : /\.(mjs|js)$/.test(file) ? 'text/javascript' : 'application/octet-stream');
        const stream = createReadStream(path);
        stream.on('error', () => { res.statusCode = 404; res.end(); });
        stream.pipe(res);
      });
      server.middlewares.use('/api', (req, res) => {
        const routes: Record<string, unknown> = {
          '/club': { id: 'study-club', name: 'Rally', time_format: '24h' },
          '/auth/callback': { user, is_new_user: false },
          '/users/me': user,
          '/accounts/me': { balance_cents: 5000, state: 'ok' },
          '/accounts/me/spend': { year: 2026, ytd_cents: 0, all_time_cents: 0, months: [] },
          '/users/me/notifications': { push_enabled: false },
          '/users/me/push-devices': [],
          '/push-devices/legacy-revoke': { cleanup_required: false },
          '/notifications': { items: [], unread_count: 0 },
          '/assistant/status': { enabled: false },
        };
        const path = req.url?.split('?')[0] ?? '';
        res.setHeader('Content-Type', 'application/json');
        if (!(path in routes)) { res.statusCode = 404; res.end(JSON.stringify({ message: 'Not connected in the voice study.' })); return; }
        res.end(JSON.stringify(routes[path]));
      });
    },
  }],
  server: { host: '127.0.0.1', port: 4175, strictPort: true, open: false },
}));
