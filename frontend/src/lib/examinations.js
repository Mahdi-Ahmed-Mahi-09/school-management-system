// Examination helpers (mirrors backend/internal/models/examination.go and
// service/examination_service.go, which re-validate everything).

export const EXAM_STATUSES = ['draft', 'published', 'completed', 'cancelled']

export const EXAM_STATUS_META = {
  draft: { label: 'Draft', badge: 'bg-slate-100 text-slate-700 ring-slate-500/15', dot: 'bg-slate-400' },
  published: { label: 'Published', badge: 'bg-brand-50 text-brand-700 ring-brand-600/15', dot: 'bg-brand-500' },
  completed: { label: 'Completed', badge: 'bg-emerald-50 text-emerald-700 ring-emerald-600/15', dot: 'bg-emerald-500' },
  cancelled: { label: 'Cancelled', badge: 'bg-rose-50 text-rose-700 ring-rose-600/15', dot: 'bg-rose-500' },
}

/** "13:30" -> "1:30 PM". */
export function formatTime(hhmm) {
  if (!hhmm) return '—'
  const [h, m] = hhmm.split(':').map(Number)
  const suffix = h >= 12 ? 'PM' : 'AM'
  return `${h % 12 || 12}:${String(m).padStart(2, '0')} ${suffix}`
}

/** "10:00 AM – 1:00 PM". */
export const timeRange = (s) => `${formatTime(s.start_time)} – ${formatTime(s.end_time)}`

/** Minutes between two "HH:MM" times. */
export function durationMinutes(start, end) {
  if (!start || !end) return 0
  const [h1, m1] = start.split(':').map(Number)
  const [h2, m2] = end.split(':').map(Number)
  return h2 * 60 + m2 - (h1 * 60 + m1)
}

/** "3 h" or "2 h 30 min". */
export function durationLabel(start, end) {
  const m = durationMinutes(start, end)
  if (m <= 0) return ''
  const h = Math.floor(m / 60)
  const r = m % 60
  return [h && `${h} h`, r && `${r} min`].filter(Boolean).join(' ')
}

/** Every date from start to end (YYYY-MM-DD, inclusive). */
export function datesBetween(start, end) {
  const out = []
  const d = new Date(`${start}T00:00:00Z`)
  const last = new Date(`${end}T00:00:00Z`)
  while (d <= last && out.length < 400) {
    out.push(d.toISOString().slice(0, 10))
    d.setUTCDate(d.getUTCDate() + 1)
  }
  return out
}

/** "Wed" for a YYYY-MM-DD date. */
export const weekday = (iso) => new Intl.DateTimeFormat(undefined, { weekday: 'short', timeZone: 'UTC' }).format(new Date(`${iso}T00:00:00Z`))

/** What an administrator may do with an examination in its current status. */
export function examActions(x) {
  const open = x.academic_year?.status !== 'completed'
  return {
    edit: open && (x.status === 'draft' || x.status === 'published'),
    schedule: open && (x.status === 'draft' || x.status === 'published'),
    classes: open && x.status === 'draft',
    publish: x.status === 'draft',
    complete: x.status === 'published',
    cancel: x.status === 'draft' || x.status === 'published',
    remove: x.status === 'draft',
  }
}

/** Escapes one CSV cell (and neutralises spreadsheet formulas). */
function csvCell(v) {
  let s = v == null ? '' : String(v)
  if (/^[=+\-@\t\r]/.test(s)) s = `'${s}`
  return /[",\n\r]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s
}

/** Downloads a timetable as CSV built from the data on screen. */
export function downloadTimetableCsv(exam, classes) {
  const header = ['Examination', 'Type', 'Academic year', 'Class', 'Subject', 'Code', 'Date', 'Start', 'End', 'Room', 'Instructions']
  const rows = [header]
  for (const c of classes) {
    for (const s of c.schedules) {
      rows.push([exam.name, exam.exam_type.name, exam.academic_year.name, c.class.name, s.subject.name, s.subject.code, s.exam_date,
        s.start_time, s.end_time, s.room_number ?? '', s.instructions ?? ''])
    }
  }
  const csv = '﻿' + rows.map((r) => r.map(csvCell).join(',')).join('\r\n')
  const url = URL.createObjectURL(new Blob([csv], { type: 'text/csv;charset=utf-8' }))
  const a = document.createElement('a')
  a.href = url
  a.download = `${exam.name}-${exam.academic_year.name}-timetable.csv`.replace(/[^\w.-]+/g, '-').toLowerCase()
  document.body.appendChild(a)
  a.click()
  a.remove()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
