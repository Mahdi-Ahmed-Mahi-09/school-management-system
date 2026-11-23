// Class and section helpers (mirrors backend/internal/service/class_service.go
// and section_service.go, which re-validate everything).
import { academicYearsApi } from '../api/services'

const namePattern = /^[^<>{}[\]\\]*$/
const codePattern = /^[A-Z0-9][A-Z0-9-]{0,19}$/

const requiredName = (label, max) => ({
  validate: (v) => {
    const value = v?.trim() ?? ''
    if (!value) return `${label} is required.`
    if (value.length > max) return `${label} is too long.`
    return namePattern.test(value) || `${label} contains characters that are not allowed.`
  },
})

const optionalCode = {
  validate: (v) => {
    const value = v?.trim().toUpperCase() ?? ''
    return !value || codePattern.test(value) || 'Use up to 20 letters, numbers or dashes.'
  },
}

const description = { validate: (v) => (v?.trim().length ?? 0) <= 500 || 'Description is too long.' }

export const classRules = {
  name: requiredName('Class name', 60),
  code: optionalCode,
  display_order: {
    validate: (v) => {
      if (v === '' || v == null) return true
      const n = Number(v)
      return (Number.isInteger(n) && n >= 1 && n <= 9999) || 'Use a whole number from 1 to 9999.'
    },
  },
  description,
}

export const sectionRules = {
  academic_year_id: { required: 'Choose an academic year.' },
  class_id: { required: 'Choose a class.' },
  name: requiredName('Section name', 30),
  code: optionalCode,
  room_number: { validate: (v) => (v?.trim().length ?? 0) <= 40 || 'Room number is too long.' },
  capacity: {
    validate: (v) => {
      if (v === '' || v == null) return true
      const n = Number(v)
      return (Number.isInteger(n) && n >= 1 && n <= 1000) || 'Capacity must be a whole number from 1 to 1000.'
    },
  },
  description,
}

/** Form value -> integer or null. */
export const toIntOrNull = (v) => (v === '' || v == null ? null : Number(v))

/**
 * Loads academic years for selectors, newest first, and picks a default: the
 * current year, else the nearest upcoming one, else the newest.
 */
export async function loadYearOptions() {
  const { years } = await academicYearsApi.list({ page_size: 100, sort: 'start_date', order: 'desc' })
  const current = years.find((y) => y.is_current) ?? null
  const upcoming = [...years].reverse().find((y) => y.status === 'upcoming') ?? null
  return { years, current, defaultYear: current ?? upcoming ?? years[0] ?? null }
}

/** "2026–2027 (current)" style label for a year option. */
export function yearOptionLabel(y) {
  if (y.is_current) return `${y.name} (current)`
  if (y.status === 'completed') return `${y.name} (completed)`
  return `${y.name} (upcoming)`
}
