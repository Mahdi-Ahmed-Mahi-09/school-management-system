// Enrollment & promotion helpers (mirrors backend/internal/models/enrollment.go
// and service/enrollment_service.go, which re-validate everything).
import { classesApi, sectionsApi } from '../api/services'

export const ENROLLMENT_STATUS_META = {
  enrolled: { label: 'Enrolled', badge: 'bg-emerald-50 text-emerald-700 ring-emerald-600/15', dot: 'bg-emerald-500' },
  promoted: { label: 'Promoted', badge: 'bg-brand-50 text-brand-700 ring-brand-600/15', dot: 'bg-brand-500' },
  repeated: { label: 'Repeated', badge: 'bg-amber-50 text-amber-700 ring-amber-600/20', dot: 'bg-amber-500' },
  completed: { label: 'Completed', badge: 'bg-violet-50 text-violet-700 ring-violet-600/15', dot: 'bg-violet-500' },
  withdrawn: { label: 'Withdrawn', badge: 'bg-slate-100 text-slate-600 ring-slate-500/15', dot: 'bg-slate-400' },
  transferred: { label: 'Transferred', badge: 'bg-rose-50 text-rose-700 ring-rose-600/15', dot: 'bg-rose-500' },
}
export const ENROLLMENT_STATUSES = Object.keys(ENROLLMENT_STATUS_META)

export const ADMISSION_TYPE_META = {
  new: { label: 'New' },
  continuing: { label: 'Continuing' },
  readmission: { label: 'Re-admission' },
}
export const ADMISSION_TYPES = Object.keys(ADMISSION_TYPE_META)

export const DECISION_META = {
  promote: { label: 'Promote', badge: 'bg-brand-50 text-brand-700 ring-brand-600/15' },
  repeat: { label: 'Repeat class', badge: 'bg-amber-50 text-amber-700 ring-amber-600/20' },
  complete: { label: 'Complete studies', badge: 'bg-violet-50 text-violet-700 ring-violet-600/15' },
  exclude: { label: 'Exclude', badge: 'bg-slate-100 text-slate-600 ring-slate-500/15' },
}
export const DECISIONS = Object.keys(DECISION_META)

export const OUTCOME_META = {
  processed: { label: 'Will be processed', done: 'Processed', badge: 'bg-emerald-50 text-emerald-700 ring-emerald-600/15' },
  skipped: { label: 'Not eligible', done: 'Skipped', badge: 'bg-rose-50 text-rose-700 ring-rose-600/15' },
  excluded: { label: 'Excluded', done: 'Excluded', badge: 'bg-slate-100 text-slate-600 ring-slate-500/15' },
}

/** "Class 6 – A" */
export const placementLabel = (cls, section) => (cls && section ? `${cls.name} – ${section.name}` : '—')

/** Whether the enrollment still allows changing section, roll, type and date. */
export const placementEditable = (e) =>
  e.academic_year.status !== 'completed' && !['promoted', 'repeated', 'completed'].includes(e.status)

export const rollRule = {
  validate: (v) => {
    if (v === '' || v == null) return true
    const n = Number(v)
    return (Number.isInteger(n) && n >= 1 && n <= 9999) || 'Use a whole number from 1 to 9999.'
  },
}

export const remarksRule = { validate: (v) => (v?.trim().length ?? 0) <= 500 || 'Remarks are too long.' }

/** Today as YYYY-MM-DD in the browser's timezone. */
export function todayISO() {
  const d = new Date()
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

/** A sensible default enrollment date for a year: today if it falls in the year, else its start. */
export function defaultEnrollmentDate(year) {
  if (!year) return todayISO()
  const today = todayISO()
  return today >= year.start_date && today <= year.end_date ? today : year.start_date
}

/** Every class (for selectors), in display order. */
export const loadClasses = () => classesApi.list({ page_size: 100, sort: 'display_order' }).then(({ classes }) => classes)

/** Sections of one class in one academic year. */
export async function loadSections(yearId, classId) {
  if (!yearId || !classId) return []
  const { sections } = await sectionsApi.list({ academic_year_id: yearId, class_id: classId, page_size: 100, sort: 'name' })
  return sections
}

/** Every section of an academic year (all classes), class order then name. */
export async function loadYearSections(yearId) {
  if (!yearId) return []
  const all = []
  for (let page = 1; page <= 20; page += 1) {
    const { sections, meta } = await sectionsApi.list({ academic_year_id: yearId, page_size: 100, page })
    all.push(...sections)
    if (page >= meta.total_pages) break
  }
  return all
}

/** Pages through a list endpoint that resolves `{ [key], meta }`. */
export async function loadAllPages(fetchPage, key, maxPages = 30) {
  const all = []
  for (let page = 1; page <= maxPages; page += 1) {
    const res = await fetchPage(page)
    all.push(...res[key])
    if (page >= res.meta.total_pages) break
  }
  return all
}

/** A fresh idempotency key for one promotion confirmation. */
export const newRequestId = () =>
  globalThis.crypto?.randomUUID?.() ??
  'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0
    return (c === 'x' ? r : (r & 0x3) | 0x8).toString(16)
  })
