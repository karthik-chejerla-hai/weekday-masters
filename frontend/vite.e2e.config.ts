import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// Explicit test-only entry point. The production config never imports this file.
export default defineConfig({
  plugins: [react()],
  envDir: new URL('./e2e/support', import.meta.url).pathname,
  define: { 'import.meta.env.VITE_API_URL': JSON.stringify('/api') },
  resolve: {
    alias: { '@auth0/auth0-react': new URL('./e2e/support/auth0.tsx', import.meta.url).pathname },
  },
  server: { host: '127.0.0.1', port: 4173, strictPort: true },
});
