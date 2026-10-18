/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_APP_ENVIRONMENT?: 'dev' | 'prod'
  readonly VITE_RELEASE_SHA?: string
  readonly VITE_API_URL: string
  readonly VITE_AUTH0_DOMAIN: string
  readonly VITE_AUTH0_CLIENT_ID: string
  readonly VITE_AUTH0_AUDIENCE: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
