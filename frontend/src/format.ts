// Formats dépendant de la langue ; la langue courante est fixée par I18nProvider.

const locales = {
  en: {tag: 'en-US', units: ['B', 'KB', 'MB', 'GB', 'TB']},
  fr: {tag: 'fr-FR', units: ['o', 'Ko', 'Mo', 'Go', 'To']},
  es: {tag: 'es-ES', units: ['B', 'KB', 'MB', 'GB', 'TB']},
}

let current = locales.en

export function setFormatLocale(lang: keyof typeof locales) {
  current = locales[lang] ?? locales.en
}

export function localeTag() {
  return current.tag
}

/** Taille lisible (1 GB = 1024³ octets, comme l'Explorateur Windows). */
export function bytes(n: number, digits = 1): string {
  const sign = n < 0 ? '−' : ''
  let v = Math.abs(n)
  let i = 0
  while (v >= 1024 && i < current.units.length - 1) {
    v /= 1024
    i++
  }
  const s = i === 0 ? String(v) : v.toLocaleString(current.tag, {minimumFractionDigits: digits, maximumFractionDigits: digits})
  return `${sign}${s} ${current.units[i]}`
}

/** Variation signée : « −8,5 Go » quand on gagne de la place, « +1,2 Go » sinon. */
export function delta(n: number): string {
  if (n === 0) return '±0'
  return n > 0 ? `+${bytes(n)}` : bytes(n)
}

export function time(iso: string | Date = new Date()): string {
  const d = new Date(iso)
  return isNaN(d.getTime()) ? '' : d.toLocaleTimeString(current.tag, {hour: '2-digit', minute: '2-digit', second: '2-digit'})
}

export function duration(ms: number): string {
  const s = Math.floor(ms / 1000)
  const m = Math.floor(s / 60)
  return m > 0 ? `${m} min ${String(s % 60).padStart(2, '0')} s` : `${s} s`
}

export function isRunning(state: string): boolean {
  return /running|exécution|cours|ejecución/i.test(state)
}

export function errorText(e: unknown): string {
  if (typeof e === 'string') return e
  if (e instanceof Error) return e.message
  return String(e)
}
