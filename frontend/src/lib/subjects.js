// Subject helpers (mirrors backend/internal/service/subject_service.go and
// class_subject_service.go, which re-validate everything).
import { subjectsApi } from '../api/services'

const namePattern = /^[^<>{}[\]\\]*$/
const codePattern = /^[A-Z0-9][A-Z0-9-]{0,19}$/

export const SUBJECT_TYPES = ['compulsory', 'optional']

export const SUBJECT_TYPE_META = {
  compulsory: { label: 'Compulsory', badge: 'bg-brand-50 text-brand-700 ring-brand-600/15', dot: 'bg-brand-500' },
  optional: { label: 'Optional', badge: 'bg-amber-50 text-amber-700 ring-amber-600/20', dot: 'bg-amber-500' },
}

/** Form value -> integer or null (blank). */
const toNumberOrNull = (v) => (v === '' || v == null ? null : Number(v))

export const subjectRules = {
  name: {
    validate: (v) => {
      const value = v?.trim() ?? ''
      if (!value) return 'Subject name is required.'
      if (value.length > 100) return 'Subject name is too long.'
      return namePattern.test(value) || 'Subject name contains characters that are not allowed.'
    },
  },
  code: {
    validate: (v) => {
      const value = v?.trim().toUpperCase() ?? ''
      if (!value) return 'Subject code is required.'
      return codePattern.test(value) || 'Use up to 20 letters, numbers or dashes.'
    },
  },
  description: { validate: (v) => (v?.trim().length ?? 0) <= 500 || 'Description is too long.' },
  full_marks: {
    validate: (v) => {
      const n = toNumberOrNull(v)
      if (n == null) return true
      return (Number.isInteger(n) && n >= 1 && n <= 1000) || 'Full marks must be a whole number from 1 to 1000.'
    },
  },
  pass_marks: {
    validate: (v, values) => {
      const n = toNumberOrNull(v)
      if (n == null) return true
      if (!Number.isInteger(n)) return 'Pass marks must be a whole number.'
      if (n < 0) return "Pass marks can't be negative."
      const full = toNumberOrNull(values.full_marks)
      if (full == null) return 'Enter the full marks to set pass marks.'
      return n <= full || "Pass marks can't be more than the full marks."
    },
  },
}

/** "33 / 100", "— / 100" or "—". */
export function formatMarks(subject) {
  if (subject.full_marks == null) return '—'
  return `${subject.pass_marks ?? '—'} / ${subject.full_marks}`
}

/** Groups class-subject assignments (already ordered newest year first) by academic year. */
export function groupAssignmentsByYear(assignments) {
  const groups = []
  for (const a of assignments) {
    const last = groups[groups.length - 1]
    if (last?.year.id === a.academic_year.id) last.items.push(a)
    else groups.push({ year: a.academic_year, items: [a] })
  }
  return groups
}

/** Loads every subject for pickers, sorted by name (pages through the API's 100-row limit). */
export async function loadAllSubjects(params = {}) {
  const all = []
  for (let page = 1; page <= 50; page += 1) {
    const { subjects, meta } = await subjectsApi.list({ ...params, sort: 'name', page, page_size: 100 })
    all.push(...subjects)
    if (page >= meta.total_pages) break
  }
  return all
}
