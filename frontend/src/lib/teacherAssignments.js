// Teacher assignment helpers (mirrors backend/internal/models/teacher_assignment.go
// and service/teacher_assignment_service.go, which re-validate everything).
import { teachersApi } from '../api/services'
import { loadAllPages } from './enrollments'

export const ASSIGNMENT_TYPE_META = {
  subject_teacher: { label: 'Subject teacher', badge: 'bg-brand-50 text-brand-700 ring-brand-600/15' },
  class_teacher: { label: 'Class teacher', badge: 'bg-violet-50 text-violet-700 ring-violet-600/15' },
}

/** Every active teacher (for pickers), by name. */
export const loadActiveTeachers = () =>
  loadAllPages((page) => teachersApi.list({ status: 'active', sort: 'name', order: 'asc', page, page_size: 100 }), 'teachers')

/** "Mathematics · Class 9 – A" or "Class teacher · Class 9 – A". */
export function assignmentSlot(a) {
  const where = `${a.class.name} – ${a.section.name}`
  return a.assignment_type === 'class_teacher' ? `Class teacher · ${where}` : `${a.subject?.name ?? 'Subject'} · ${where}`
}

/** Teacher option label: "Ayesha Khan (T-001)". */
export const teacherLabel = (t) => `${t.full_name} (${t.employee_id})`

export const remarksRule = { validate: (v) => (v?.trim().length ?? 0) <= 500 || 'Remarks are too long.' }

/** Removal is offered only for assignments made by mistake (the server re-checks). */
export const removable = (a) => a.status === 'active' && a.academic_year.status !== 'completed' && !a.previous_assignment_id

/** Changing the teacher is possible for active assignments in an open year. */
export const changeable = (a) => a.status === 'active' && a.academic_year.status !== 'completed'
