/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Override the API base URL (defaults to the deployed Cloud Run service). */
  readonly VITE_API_BASE?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
