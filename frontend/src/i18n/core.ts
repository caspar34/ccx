import { messages, type MessageKey, type SupportedLocale } from './messages'

const SUPPORTED_LOCALE_MAP: Record<string, SupportedLocale> = {
  en: 'en',
  'en-us': 'en',
  id: 'id',
  'id-id': 'id',
  zh: 'zh-CN',
  'zh-cn': 'zh-CN',
}

export const DEFAULT_LOCALE: SupportedLocale = 'en'

export function normalizeLocale(locale?: string | null): SupportedLocale {
  if (!locale) return DEFAULT_LOCALE
  return SUPPORTED_LOCALE_MAP[locale.trim().toLowerCase()] || DEFAULT_LOCALE
}

export function isSupportedLocale(locale?: string | null): boolean {
  if (!locale) return false
  return !!SUPPORTED_LOCALE_MAP[locale.trim().toLowerCase()]
}

export function resolveInitialLocale(
  persistedLocale?: string | null,
  runtimeLocale?: string | null,
): SupportedLocale {
  if (isSupportedLocale(persistedLocale)) return normalizeLocale(persistedLocale)
  if (isSupportedLocale(runtimeLocale)) return normalizeLocale(runtimeLocale)
  return DEFAULT_LOCALE
}

export function getRuntimeLocale(): SupportedLocale {
  if (typeof window !== 'undefined' && window.__CCX_RUNTIME_CONFIG__?.uiLanguage) {
    return normalizeLocale(window.__CCX_RUNTIME_CONFIG__.uiLanguage)
  }

  if (typeof __APP_UI_LANGUAGE__ !== 'undefined') {
    return normalizeLocale(__APP_UI_LANGUAGE__)
  }

  return DEFAULT_LOCALE
}

export function getDocumentLanguage(locale: SupportedLocale): string {
  return locale === 'zh-CN' ? 'zh-CN' : locale
}

export function applyDocumentLanguage(locale: SupportedLocale) {
  if (typeof document === 'undefined') return
  document.documentElement.lang = getDocumentLanguage(locale)
}

export function translate(
  locale: SupportedLocale,
  key: MessageKey,
  params?: Record<string, string | number>,
): string {
  const template = messages[locale][key] ?? messages.en[key] ?? key

  if (!params) return template

  return template.replace(/\{(\w+)\}/g, (_, paramKey: string) => {
    return String(params[paramKey] ?? `{${paramKey}}`)
  })
}

export function createTranslator(locale: SupportedLocale) {
  return (key: MessageKey, params?: Record<string, string | number>) => translate(locale, key, params)
}
