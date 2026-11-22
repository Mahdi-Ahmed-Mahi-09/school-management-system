// Student-specific constants and helpers (mirrors backend/internal/models/student.go).

export const GENDERS = [
  { value: 'female', label: 'Female' },
  { value: 'male', label: 'Male' },
  { value: 'other', label: 'Other' },
]

export const genderLabel = (value) => GENDERS.find((g) => g.value === value)?.label ?? '—'

export const BLOOD_GROUPS = ['A+', 'A-', 'B+', 'B-', 'AB+', 'AB-', 'O+', 'O-']

/** Today's date as YYYY-MM-DD in the browser's timezone. */
export const todayISO = () => new Date().toLocaleDateString('en-CA')

const dateOnlyFormat = new Intl.DateTimeFormat(undefined, { day: 'numeric', month: 'short', year: 'numeric' })

/** Formats a YYYY-MM-DD calendar date without timezone shifting. */
export function formatDateOnly(value) {
  if (!value) return '—'
  const [y, m, d] = value.split('-').map(Number)
  return dateOnlyFormat.format(new Date(y, m - 1, d))
}

/** Age in whole years on `on` (YYYY-MM-DD, default today). */
export function ageOn(dob, on = todayISO()) {
  if (!dob) return null
  const [by, bm, bd] = dob.split('-').map(Number)
  const [y, m, d] = on.split('-').map(Number)
  let age = y - by
  if (m < bm || (m === bm && d < bd)) age--
  return age
}

/** Editable fields, in API order. */
export const STUDENT_FIELDS = [
  'admission_number',
  'full_name',
  'date_of_birth',
  'gender',
  'blood_group',
  'phone',
  'email',
  'address',
  'father_name',
  'father_phone',
  'mother_name',
  'mother_phone',
  'guardian_name',
  'guardian_relation',
  'guardian_phone',
  'emergency_contact_name',
  'emergency_contact_relation',
  'emergency_contact_phone',
  'admission_date',
  'status',
]

/** Form values for a new student. */
export function emptyStudentForm() {
  const values = Object.fromEntries(STUDENT_FIELDS.map((f) => [f, '']))
  return { ...values, status: 'active', admission_date: todayISO() }
}

/** Converts an API student into form values (nulls become empty strings). */
export function studentToForm(student) {
  return Object.fromEntries(STUDENT_FIELDS.map((f) => [f, student?.[f] ?? '']))
}

/** Picks the API payload out of form values. */
export function formToPayload(values) {
  return Object.fromEntries(STUDENT_FIELDS.map((f) => [f, typeof values[f] === 'string' ? values[f].trim() : values[f]]))
}

const phonePattern = /^\+?[0-9][0-9 ()-]{5,18}[0-9]$/
const namePattern = /^[^<>{}[\]\\]*$/

const optionalPhone = { validate: (v) => !v?.trim() || phonePattern.test(v.trim()) || 'Enter a valid phone number.' }
const requiredPhone = (label) => ({
  validate: (v) => (v?.trim() ? phonePattern.test(v.trim()) || 'Enter a valid phone number.' : `${label} is required.`),
})
const personName = (label, required = true) => ({
  validate: (v) => {
    const value = v?.trim() ?? ''
    if (!value) return required ? `${label} is required.` : true
    if (value.length > 120) return `${label} is too long.`
    return namePattern.test(value) || `${label} contains characters that are not allowed.`
  },
})

/** Client-side rules mirroring the API (which re-validates everything). */
export const studentRules = {
  admission_number: {
    validate: (v) => {
      const value = v?.trim().toUpperCase() ?? ''
      if (!value) return 'Student ID is required.'
      return /^[A-Z0-9][A-Z0-9/_-]{1,31}$/.test(value) || 'Use 2–32 letters, numbers, dashes, slashes or underscores.'
    },
  },
  full_name: personName('Full name'),
  date_of_birth: {
    validate: (v) => {
      if (!v) return 'Date of birth is required.'
      if (v >= todayISO()) return 'Date of birth must be in the past.'
      return v >= '1900-01-01' || 'Enter a realistic date of birth.'
    },
  },
  gender: { required: 'Choose a gender.' },
  phone: optionalPhone,
  email: {
    validate: (v) => !v?.trim() || /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v.trim()) || 'Enter a valid email address.',
  },
  address: {
    validate: (v) => {
      const value = v?.trim() ?? ''
      if (!value) return 'Address is required.'
      return value.length <= 500 || 'Address is too long.'
    },
  },
  father_name: personName("Father's name", false),
  father_phone: optionalPhone,
  mother_name: personName("Mother's name", false),
  mother_phone: optionalPhone,
  guardian_name: personName("Guardian's name"),
  guardian_relation: { maxLength: { value: 40, message: 'Relationship is too long.' } },
  guardian_phone: requiredPhone("Guardian's phone"),
  emergency_contact_name: personName('Emergency contact name'),
  emergency_contact_relation: { maxLength: { value: 40, message: 'Relationship is too long.' } },
  emergency_contact_phone: requiredPhone('Emergency contact phone'),
  admission_date: (getDob) => ({
    validate: (v) => {
      if (!v) return 'Admission date is required.'
      const dob = getDob()
      return !dob || v > dob || 'Admission date must be after the date of birth.'
    },
  }),
}
