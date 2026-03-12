/// <reference types="vite/client" />

declare const __APP_UI_LANGUAGE__: string

// Allow importing .vue files in TS
declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const component: DefineComponent<Record<string, unknown>, Record<string, unknown>, any>
  export default component
}

interface Window {
  __CCX_RUNTIME_CONFIG__?: {
    uiLanguage?: string
  }
}
