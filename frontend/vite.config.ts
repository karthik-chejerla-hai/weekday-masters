/// <reference types="vitest" />
import { defineConfig } from 'vite'
import { configDefaults } from 'vitest/config'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'

export default defineConfig({
  plugins: [
    react(),
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: ['badminton.svg', 'icons/*.svg'],
      manifest: false, // We use our own manifest.json in public folder
      workbox: {
        globPatterns: ['**/*.{js,css,html,ico,png,svg,woff2}'],
        runtimeCaching: [
          {
            urlPattern: /^https:\/\/.*\.run\.app\/api\/.*/i,
            handler: 'NetworkFirst',
            options: {
              cacheName: 'api-cache',
              expiration: {
                maxEntries: 100,
                maxAgeSeconds: 60 * 60 // 1 hour
              },
              cacheableResponse: {
                statuses: [0, 200]
              }
            }
          }
        ]
      }
    })
  ],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
    // Playwright owns e2e/. Vitest picking those files up throws
    // "Playwright Test did not expect test.describe() to be called here".
    exclude: [...configDefaults.exclude, 'e2e/**'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json-summary', 'json', 'lcov'],
      // Without `all`, v8 only reports on files a test happened to import, so
      // the percentage describes the tested corner of the app rather than the
      // app. Every source file counts, whether or not anything covers it.
      all: true,
      include: ['src/**/*.{ts,tsx}'],
      // A ratchet set just under where the suite currently sits, so coverage
      // cannot slide back without someone deciding to lower it. Raise these as
      // Admin.tsx and App.tsx get covered.
      thresholds: {
        statements: 65,
        branches: 62,
        functions: 60,
        lines: 67,
      },
      exclude: [
        'src/main.tsx',
        'src/vite-env.d.ts',
        'src/types/**',
        'src/test/**',
        // Thin wrappers over the Firebase SDK: exercising them in jsdom would
        // test the mock, not the integration.
        'src/services/firebase.ts',
        'src/services/notifications.ts',
        '**/*.d.ts',
        'vite.config.ts',
        'tailwind.config.js',
        'postcss.config.js'
      ]
    }
  }
})
