// Academic year constants and helpers (mirrors backend/internal/models/academic_year.go).
import { formatDateOnly } from './students'

export const YEAR_STATUSES = ['upcoming', 'active', 'completed']

export const YEAR_STATUS_META = {
  upcoming: {
    label: 'Upcoming',
    badge: 'bg-sky-50 text-sky-700 ring-sky-600/15',
    dot: 'bg-sky-500',
    description: 'Planned, not yet the current year.',
  },
  active: {
    label: 'Active',
    badge: 'bg-emerald-50 text-emerald-700 ring-emerald-600/15',
    dot: 'bg-emerald-500',
    description: 'The school’s current academic year.',
  },
  completed: {
    label: 'Completed',
    badge: 'bg-slate-100 text-slate-600 ring-slate-500/15',
    dot: 'bg-slate-400',
    description: 'Finished and kept for records.',
  },
}

/** "1 Jan 2026 – 31 Dec 2026" */
export const formatYearRange = (year) => `${formatDateOnly(year.start_date)} – ${formatDateOnly(year.end_date)}`

/** "12 months" or "45 days" from an inclusive day count. */
export function durationLabel(days) {
  if (!days) return '—'
  if (days < 60) return `${days} days`
  const months = Math.round(days / 30.44)
  return `${months} months`
}

/** Percentage of the year elapsed, from the server's day counts. */
export const progressPercent = (year) =>
  year?.duration_days ? Math.min(100, Math.max(0, Math.round((year.days_elapsed / year.duration_days) * 100))) : 0

/**
 * A short note when the dates and the status tell different stories, so the
 * admin knows an action is due (status only changes through admin actions).
 */
export function attentionNote(year) {
  if (year.status === 'active' && year.timeline === 'ended') {
    return `This year ended on ${formatDateOnly(year.end_date)}. Mark it completed and set the next year as current.`
  }
  if (year.status === 'upcoming' && year.timeline === 'ended') {
    return 'These dates have passed without the year being made current. Mark it completed to keep it for records.'
  }
  if (year.status === 'upcoming' && year.timeline === 'in_progress') {
    return 'This year has started but is not the current year yet.'
  }
  return null
}

/** Suggests "2026" or "2026–2027" from the dates. */
export function suggestYearName(start, end) {
  if (!start || !end) return ''
  const a = start.slice(0, 4)
  const b = end.slice(0, 4)
  return a === b ? a : `${a}–${b}`
}

const namePattern = /^[^<>{}[\]\\]*$/

/** Client-side rules mirroring the API (which re-validates everything). */
export const yearRules = {
  name: {
    validate: (v) => {
      const value = v?.trim() ?? ''
      if (!value) return 'Academic year name is required.'
      if (value.length > 50) return 'Academic year name is too long.'
      return namePattern.test(value) || 'Name contains characters that are not allowed.'
    },
  },
  start_date: { required: 'Start date is required.' },
  end_date: (getStart) => ({
    validate: (v) => {
      if (!v) return 'End date is required.'
      const start = getStart()
      if (!start) return true
      if (v <= start) return 'End date must be after the start date.'
      const [y, m, d] = start.split('-').map(Number)
      const limit = `${y + 2}-${String(m).padStart(2, '0')}-${String(d).padStart(2, '0')}`
      return v <= limit || 'An academic year can be at most 2 years long.'
    },
  }),
  description: { validate: (v) => (v?.trim().length ?? 0) <= 500 || 'Description is too long.' },
}
