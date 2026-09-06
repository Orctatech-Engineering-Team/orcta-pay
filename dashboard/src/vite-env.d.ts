/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_ORCTA_PAY_URL?: string;
  readonly VITE_ORCTA_PAY_PRODUCT?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
