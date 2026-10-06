import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { loadEnv, type Plugin } from 'vite';

// Embed public web configuration so a restarted worker needs no open page.
export function firebaseWorker(): Plugin {
  let source = '';
  return {
    name: 'firebase-worker-config',
    configResolved(config) {
      const env = { ...loadEnv(config.mode, config.envDir, 'VITE_'), ...process.env };
      const firebaseConfig = {
        apiKey: env.VITE_FIREBASE_API_KEY,
        projectId: env.VITE_FIREBASE_PROJECT_ID,
        messagingSenderId: env.VITE_FIREBASE_MESSAGING_SENDER_ID,
        appId: env.VITE_FIREBASE_APP_ID,
      };
      source = readFileSync(resolve(config.root, 'public/firebase-messaging-sw.js'), 'utf8')
        .replace('/* FIREBASE_CONFIG */ {}', JSON.stringify(firebaseConfig));
    },
    configureServer(server) {
      server.middlewares.use('/firebase-messaging-sw.js', (_req, res) => {
        res.setHeader('Content-Type', 'application/javascript');
        res.setHeader('Cache-Control', 'no-cache');
        res.end(source);
      });
    },
    generateBundle() {
      this.emitFile({ type: 'asset', fileName: 'firebase-messaging-sw.js', source });
    },
  };
}
