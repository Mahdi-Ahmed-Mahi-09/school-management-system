/** Joins truthy class names. */
export const cn = (...classes) => classes.filter(Boolean).join(' ')

/** Up to two initials for an avatar. */
export function initials(user) {
  const first = user?.first_name?.trim()?.[0] ?? ''
  const last = user?.last_name?.trim()?.[0] ?? ''
  return (first + last).toUpperCase() || user?.username?.[0]?.toUpperCase() || '?'
}

const numberFormat = new Intl.NumberFormat()
const monthShort = new Intl.DateTimeFormat(undefined, { month: 'short' })
const monthLong = new Intl.DateTimeFormat(undefined, { month: 'long', year: 'numeric' })
const longDate = new Intl.DateTimeFormat(undefined, { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' })

/** Locale-formatted integer, or an em dash when the value is missing. */
export const formatNumber = (value) => (value == null ? '—' : numberFormat.format(value))

/** Percentage of part in total, rounded; null when total is zero. */
export const percent = (part, total) => (total > 0 ? Math.round((part / total) * 100) : null)

/** "2026-11" -> "Nov" (short) or "November 2026" (long). */
export function formatMonth(key, style = 'short') {
  const [y, m] = key.split('-').map(Number)
  const date = new Date(y, m - 1, 1)
  return (style === 'long' ? monthLong : monthShort).format(date)
}

/** "Sunday, 22 November 2026". */
export const formatLongDate = (date = new Date()) => longDate.format(date)

/** Formats a decimal string amount in a currency, e.g. ("1250.50", "BDT"). */
export function formatCurrency(amount, currency) {
  const value = Number(amount)
  if (!Number.isFinite(value)) return '—'
  try {
    return new Intl.NumberFormat(undefined, { style: 'currency', currency, maximumFractionDigits: 0 }).format(value)
  } catch {
    return `${numberFormat.format(value)} ${currency ?? ''}`.trim()
  }
}

/** Compact currency for chart axes, e.g. "BDT 140K". */
export function formatCurrencyCompact(amount, currency) {
  const value = Number(amount)
  if (!Number.isFinite(value)) return ''
  try {
    return new Intl.NumberFormat(undefined, { style: 'currency', currency, notation: 'compact', maximumFractionDigits: 1 }).format(value)
  } catch {
    return new Intl.NumberFormat(undefined, { notation: 'compact' }).format(value)
  }
}

const dateFormat = new Intl.DateTimeFormat(undefined, { day: 'numeric', month: 'short', year: 'numeric' })
const dateTimeFormat = new Intl.DateTimeFormat(undefined, {
  day: 'numeric',
  month: 'short',
  year: 'numeric',
  hour: 'numeric',
  minute: '2-digit',
})
const relative = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' })

export const formatDate = (value) => (value ? dateFormat.format(new Date(value)) : '—')
export const formatDateTime = (value) => (value ? dateTimeFormat.format(new Date(value)) : '—')

/** "3 hours ago", "yesterday", falling back to a date after a month. */
export function formatRelative(value) {
  if (!value) return 'Never'
  const seconds = (new Date(value).getTime() - Date.now()) / 1000
  const abs = Math.abs(seconds)
  if (abs < 60) return 'Just now'
  if (abs < 3600) return relative.format(Math.round(seconds / 60), 'minute')
  if (abs < 86400) return relative.format(Math.round(seconds / 3600), 'hour')
  if (abs < 86400 * 30) return relative.format(Math.round(seconds / 86400), 'day')
  return formatDate(value)
}

/** Greeting based on the local time of day. */
export function greeting(date = new Date()) {
  const hour = date.getHours()
  if (hour < 12) return 'Good morning'
  if (hour < 18) return 'Good afternoon'
  return 'Good evening'
}

/** Generates a strong random password that satisfies the API policy. */
export function generatePassword(length = 14) {
  const lower = 'abcdefghjkmnpqrstuvwxyz'
  const upper = 'ABCDEFGHJKMNPQRSTUVWXYZ'
  const digits = '23456789'
  const symbols = '!@#$%&*?'
  const all = lower + upper + digits + symbols
  const random = (max) => {
    const buf = new Uint32Array(1)
    crypto.getRandomValues(buf)
    return buf[0] % max
  }
  const chars = [lower, upper, digits, symbols].map((set) => set[random(set.length)])
  while (chars.length < length) chars.push(all[random(all.length)])
  for (let i = chars.length - 1; i > 0; i--) {
    const j = random(i + 1)
    ;[chars[i], chars[j]] = [chars[j], chars[i]]
  }
  return chars.join('')
}

/** Scores password strength 0–4 with a label, for UI feedback only. */
export function passwordStrength(password = '') {
  if (!password) return { score: 0, label: '' }
  let score = 0
  if (password.length >= 8) score++
  if (password.length >= 12) score++
  if (/[a-z]/.test(password) && /[A-Z]/.test(password)) score++
  if (/\d/.test(password) && /[^A-Za-z0-9]/.test(password)) score++
  const labels = ['Very weak', 'Weak', 'Fair', 'Good', 'Strong']
  return { score, label: labels[score] }
}
