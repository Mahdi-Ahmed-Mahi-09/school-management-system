// Teacher-specific constants and helpers (mirrors backend/internal/models/teacher.go).
import { todayISO } from './students'

export const EMPLOYMENT_TYPES = [
  { value: 'full_time', label: 'Full-time' },
  { value: 'part_time', label: 'Part-time' },
]

export const employmentLabel = (value) => EMPLOYMENT_TYPES.find((t) => t.value === value)?.label ?? '—'

/** Common designations offered as suggestions; any text is accepted. */
export const DESIGNATION_SUGGESTIONS = [
  'Head Teacher',
  'Assistant Head Teacher',
  'Senior Teacher',
  'Assistant Teacher',
  'Lecturer',
  'Instructor',
  'Coordinator',
]

/** Editable fields, in API order (salary is handled separately). */
export const TEACHER_FIELDS = [
  'employee_id',
  'full_name',
  'gender',
  'date_of_birth',
  'phone',
  'email',
  'address',
  'qualification',
  'specialization',
  'designation',
  'employment_type',
  'joining_date',
  'status',
  'emergency_contact_name',
  'emergency_contact_relation',
  'emergency_contact_phone',
  'notes',
  'user_id',
]

export function emptyTeacherForm() {
  const values = Object.fromEntries(TEACHER_FIELDS.map((f) => [f, '']))
  return { ...values, status: 'active', employment_type: 'full_time', joining_date: todayISO(), salary: '' }
}

/** API teacher -> form values (nulls become empty strings). */
export function teacherToForm(teacher) {
  const values = Object.fromEntries(TEACHER_FIELDS.map((f) => [f, teacher?.[f] ?? '']))
  values.user_id = teacher?.account?.id ?? ''
  values.salary = teacher?.salary ?? ''
  return values
}

/**
 * Form values -> API payload. Salary is only sent when the user may manage
 * it; otherwise it is omitted so the server keeps the current value.
 */
export function teacherFormToPayload(values, { includeSalary }) {
  const payload = Object.fromEntries(TEACHER_FIELDS.map((f) => [f, typeof values[f] === 'string' ? values[f].trim() : values[f]]))
  if (includeSalary) payload.salary = (values.salary ?? '').toString().trim()
  return payload
}

const phonePattern = /^\+?[0-9][0-9 ()-]{5,18}[0-9]$/
const namePattern = /^[^<>{}[\]\\]*$/

const optionalText = (label, max) => ({ validate: (v) => (v?.trim().length ?? 0) <= max || `${label} is too long.` })

/** Client-side rules mirroring the API (which re-validates everything). */
export const teacherRules = {
  employee_id: {
    validate: (v) => {
      const value = v?.trim().toUpperCase() ?? ''
      if (!value) return 'Teacher ID is required.'
      return /^[A-Z0-9][A-Z0-9/_-]{1,31}$/.test(value) || 'Use 2–32 letters, numbers, dashes, slashes or underscores.'
    },
  },
  full_name: {
    validate: (v) => {
      const value = v?.trim() ?? ''
      if (!value) return 'Full name is required.'
      if (value.length > 120) return 'Full name is too long.'
      return namePattern.test(value) || 'Full name contains characters that are not allowed.'
    },
  },
  gender: { required: 'Choose a gender.' },
  date_of_birth: (getJoining) => ({
    validate: (v) => {
      if (!v) return true
      if (v >= todayISO()) return 'Date of birth must be in the past.'
      if (v < '1900-01-01') return 'Enter a realistic date of birth.'
      const joining = getJoining()
      if (joining) {
        const [y, m, d] = v.split('-').map(Number)
        const sixteen = `${y + 16}-${String(m).padStart(2, '0')}-${String(d).padStart(2, '0')}`
        if (sixteen > joining) return 'Teacher must be at least 16 years old on the joining date.'
      }
      return true
    },
  }),
  phone: {
    validate: (v) => (v?.trim() ? phonePattern.test(v.trim()) || 'Enter a valid phone number.' : 'Phone number is required.'),
  },
  email: { validate: (v) => !v?.trim() || /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v.trim()) || 'Enter a valid email address.' },
  address: optionalText('Address', 500),
  qualification: optionalText('Qualification', 200),
  specialization: optionalText('Specialization', 120),
  designation: {
    validate: (v) => {
      const value = v?.trim() ?? ''
      if (!value) return 'Designation is required.'
      return value.length <= 80 || 'Designation is too long.'
    },
  },
  employment_type: { required: 'Choose an employment type.' },
  joining_date: { required: 'Joining date is required.' },
  salary: {
    validate: (v) => {
      const value = (v ?? '').toString().replace(/,/g, '').trim()
      return !value || /^\d{1,10}(\.\d{1,2})?$/.test(value) || 'Enter an amount like 45000 or 45000.50.'
    },
  },
  emergency_contact_name: (getPhone) => ({
    validate: (v) => {
      const value = v?.trim() ?? ''
      if (!value && getPhone()?.trim()) return 'Add a name for the emergency contact.'
      if (value.length > 120) return 'Name is too long.'
      return namePattern.test(value) || 'Name contains characters that are not allowed.'
    },
  }),
  emergency_contact_relation: optionalText('Relationship', 40),
  emergency_contact_phone: (getName) => ({
    validate: (v) => {
      const value = v?.trim() ?? ''
      if (!value) return getName()?.trim() ? 'Add a phone number for the emergency contact.' : true
      return phonePattern.test(value) || 'Enter a valid phone number.'
    },
  }),
  notes: optionalText('Notes', 1000),
}
