// Attendance helpers (mirrors backend/internal/models/attendance.go and
// service/attendance_service.go, which re-validate everything).
import { attendanceApi } from '../api/services'
import { getErrorMessage } from '../api/client'

export const ATTENDANCE_STATUSES = ['present', 'absent', 'late', 'leave']

export const ATTENDANCE_STATUS_META = {
  present: {
    label: 'Present',
    short: 'P',
    badge: 'bg-emerald-50 text-emerald-700 ring-emerald-600/15',
    dot: 'bg-emerald-500',
    active: 'bg-emerald-600 text-white ring-emerald-600',
    hover: 'hover:bg-emerald-50 hover:text-emerald-700',
  },
  absent: {
    label: 'Absent',
    short: 'A',
    badge: 'bg-rose-50 text-rose-700 ring-rose-600/15',
    dot: 'bg-rose-500',
    active: 'bg-rose-600 text-white ring-rose-600',
    hover: 'hover:bg-rose-50 hover:text-rose-700',
  },
  late: {
    label: 'Late',
    short: 'L',
    badge: 'bg-amber-50 text-amber-800 ring-amber-600/20',
    dot: 'bg-amber-500',
    active: 'bg-amber-500 text-white ring-amber-500',
    hover: 'hover:bg-amber-50 hover:text-amber-800',
  },
  leave: {
    label: 'Leave',
    short: 'LV',
    badge: 'bg-sky-50 text-sky-700 ring-sky-600/15',
    dot: 'bg-sky-500',
    active: 'bg-sky-600 text-white ring-sky-600',
    hover: 'hover:bg-sky-50 hover:text-sky-700',
  },
}

export const UNMARKED_META = { label: 'Unmarked', badge: 'bg-slate-100 text-slate-600 ring-slate-500/15', dot: 'bg-slate-400' }

export const DAY_STATE_META = {
  complete: { label: 'Complete', badge: 'bg-emerald-50 text-emerald-700 ring-emerald-600/15', dot: 'bg-emerald-500' },
  partial: { label: 'Partly marked', badge: 'bg-amber-50 text-amber-800 ring-amber-600/20', dot: 'bg-amber-500' },
  not_started: { label: 'Not taken', badge: 'bg-slate-100 text-slate-600 ring-slate-500/15', dot: 'bg-slate-400' },
  no_students: { label: 'No students', badge: 'bg-slate-50 text-slate-500 ring-slate-400/15', dot: 'bg-slate-300' },
}

/** "92.5%" or "N/A" when nothing counts towards the percentage. */
export const formatPercent = (p) => (p == null ? 'N/A' : `${Number.isInteger(p) ? p : p.toFixed(1)}%`)

/** Percentage of (present + late) over (present + absent + late); leave and unmarked are excluded. */
export function attendancePercent({ present = 0, absent = 0, late = 0 }) {
  const den = present + absent + late
  return den === 0 ? null : Math.round(((present + late) * 1000) / den) / 10
}

/** Live counts for a roster being marked: `statuses` maps enrollment id -> status. */
export function liveSummary(students, statuses) {
  const out = { total: 0, present: 0, absent: 0, late: 0, leave: 0, unmarked: 0 }
  for (const s of students) {
    if (s.state === 'elsewhere') continue
    out.total += 1
    const st = statuses[s.enrollment_id]
    if (st) out[st] += 1
    else out.unmarked += 1
  }
  out.percentage = attendancePercent(out)
  return out
}

/** "2026-11" -> "November 2026". */
export function monthLabel(key) {
  const [y, m] = key.split('-').map(Number)
  return new Intl.DateTimeFormat(undefined, { month: 'long', year: 'numeric' }).format(new Date(y, m - 1, 1))
}

/** Months of an academic year up to `today`, newest first, as YYYY-MM. */
export function yearMonths(year, today) {
  if (!year) return []
  const end = today < year.end_date ? today : year.end_date
  const out = []
  let [y, m] = year.start_date.split('-').map(Number)
  const [ey, em] = end.split('-').map(Number)
  while (y < ey || (y === ey && m <= em)) {
    out.push(`${y}-${String(m).padStart(2, '0')}`)
    m += 1
    if (m > 12) {
      m = 1
      y += 1
    }
  }
  return out.reverse()
}

/** "Class 9 – A". */
export const sectionLabel = (cls, section) => (cls && section ? `${cls.name} – ${section.name}` : '—')

/** Finds a section (and its class) in the options payload. */
export function findSection(options, sectionId) {
  for (const c of options?.classes ?? []) {
    const s = c.sections.find((x) => x.id === sectionId)
    if (s) return { class: c.class, section: s }
  }
  return null
}

export const remarksRule = { validate: (v) => (v?.trim().length ?? 0) <= 300 || 'Remarks are too long.' }

/** Downloads a CSV export and saves it with the server's file name. */
export async function downloadAttendanceCsv(params) {
  let res
  try {
    res = await attendanceApi.export(params)
  } catch (err) {
    // Error bodies arrive as a Blob because of responseType: 'blob'.
    const data = err?.response?.data
    if (data instanceof Blob) {
      try {
        const parsed = JSON.parse(await data.text())
        throw new Error(parsed?.error?.message ?? 'The export failed.')
      } catch (inner) {
        throw inner instanceof SyntaxError ? new Error('The export failed.') : inner
      }
    }
    throw new Error(getErrorMessage(err, 'The export failed.'))
  }
  const disposition = res.headers['content-disposition'] ?? ''
  const name = /filename="([^"]+)"/.exec(disposition)?.[1] ?? 'attendance.csv'
  const url = URL.createObjectURL(res.data)
  const a = document.createElement('a')
  a.href = url
  a.download = name
  document.body.appendChild(a)
  a.click()
  a.remove()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
  return name
}
