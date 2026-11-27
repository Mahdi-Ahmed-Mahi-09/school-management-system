import { api, unwrap } from './client'

export const authApi = {
  login: (payload) => api.post('/auth/login', payload).then(unwrap),
  logout: () => api.post('/auth/logout').then(unwrap),
  me: () => api.get('/auth/me').then(unwrap),
}

export const rolesApi = {
  /** Resolves to `[{ role, permissions }]`. */
  list: () => api.get('/roles').then(unwrap),
}

export const profileApi = {
  get: () => api.get('/profile').then(unwrap),
  update: (payload) => api.patch('/profile', payload).then(unwrap),
  changePassword: (payload) => api.put('/profile/password', payload).then(unwrap),
}

export const dashboardApi = {
  /** Resolves to the admin dashboard payload (see backend models/dashboard.go). */
  admin: () => api.get('/admin/dashboard').then(unwrap),
}

export const studentsApi = {
  /** Resolves to `{ students, meta }`. */
  list: (params, { signal } = {}) =>
    api.get('/students', { params, signal }).then((res) => ({ students: res.data.data, meta: res.data.meta })),
  stats: () => api.get('/students/stats').then(unwrap),
  get: (id) => api.get(`/students/${id}`).then(unwrap),
  create: (payload) => api.post('/students', payload).then(unwrap),
  update: (id, payload) => api.put(`/students/${id}`, payload).then(unwrap),
  remove: (id) => api.delete(`/students/${id}`).then(unwrap),
  uploadPhoto: (id, blob) => {
    const form = new FormData()
    form.append('photo', blob, 'photo.jpg')
    // Let the browser set the multipart boundary.
    return api.put(`/students/${id}/photo`, form, { headers: { 'Content-Type': undefined } }).then(unwrap)
  },
  removePhoto: (id) => api.delete(`/students/${id}/photo`).then(unwrap),
  /** The student's enrollments, newest academic year first. */
  enrollmentHistory: (id) => api.get(`/students/${id}/enrollment-history`).then(unwrap),
  /** Attendance records of the student; resolves to `{ records, meta }`. */
  attendance: (id, params, { signal } = {}) =>
    api.get(`/students/${id}/attendance`, { params, signal }).then((res) => ({ records: res.data.data, meta: res.data.meta })),
  /** Counts for one academic year (default: current or latest), per month, and recent records. */
  attendanceSummary: (id, params) => api.get(`/students/${id}/attendance/summary`, { params }).then(unwrap),
}

export const teachersApi = {
  /** Resolves to `{ teachers, meta }`. Listings never include salary. */
  list: (params, { signal } = {}) =>
    api.get('/teachers', { params, signal }).then((res) => ({ teachers: res.data.data, meta: res.data.meta })),
  stats: () => api.get('/teachers/stats').then(unwrap),
  linkableAccounts: (params) => api.get('/teachers/linkable-accounts', { params }).then(unwrap),
  get: (id) => api.get(`/teachers/${id}`).then(unwrap),
  create: (payload) => api.post('/teachers', payload).then(unwrap),
  update: (id, payload) => api.put(`/teachers/${id}`, payload).then(unwrap),
  remove: (id) => api.delete(`/teachers/${id}`).then(unwrap),
  uploadPhoto: (id, blob) => {
    const form = new FormData()
    form.append('photo', blob, 'photo.jpg')
    return api.put(`/teachers/${id}/photo`, form, { headers: { 'Content-Type': undefined } }).then(unwrap)
  },
  removePhoto: (id) => api.delete(`/teachers/${id}/photo`).then(unwrap),
  /** The teacher's subject and class-teacher assignments, newest year first. */
  assignments: (id) => api.get(`/teachers/${id}/assignments`).then(unwrap),
}

export const academicYearsApi = {
  /** Resolves to `{ years, meta }`. */
  list: (params, { signal } = {}) =>
    api.get('/academic-years', { params, signal }).then((res) => ({ years: res.data.data, meta: res.data.meta })),
  /** Resolves to `{ total, upcoming, active, completed, current }`. */
  stats: () => api.get('/academic-years/stats').then(unwrap),
  get: (id) => api.get(`/academic-years/${id}`).then(unwrap),
  create: (payload) => api.post('/academic-years', payload).then(unwrap),
  update: (id, payload) => api.put(`/academic-years/${id}`, payload).then(unwrap),
  /** Resolves to the activated year with `previous` (the demoted year, or null). */
  activate: (id) => api.patch(`/academic-years/${id}/activate`).then(unwrap),
  complete: (id) => api.patch(`/academic-years/${id}/complete`).then(unwrap),
  remove: (id) => api.delete(`/academic-years/${id}`).then(unwrap),
}

export const classesApi = {
  /** Resolves to `{ classes, meta }`. */
  list: (params, { signal } = {}) =>
    api.get('/classes', { params, signal }).then((res) => ({ classes: res.data.data, meta: res.data.meta })),
  stats: () => api.get('/classes/stats').then(unwrap),
  get: (id) => api.get(`/classes/${id}`).then(unwrap),
  /** Every section of the class, newest academic year first (optionally one year). */
  sections: (id, params) => api.get(`/classes/${id}/sections`, { params }).then(unwrap),
  /** Class teacher and subject teachers of each section of the class in a year (default: current). */
  teacherAssignments: (id, params) => api.get(`/classes/${id}/teacher-assignments`, { params }).then(unwrap),
  /** Every subject assignment of the class, newest academic year first (optionally one year). */
  subjects: (id, params) => api.get(`/classes/${id}/subjects`, { params }).then(unwrap),
  create: (payload) => api.post('/classes', payload).then(unwrap),
  update: (id, payload) => api.put(`/classes/${id}`, payload).then(unwrap),
  setStatus: (id, status) => api.patch(`/classes/${id}/status`, { status }).then(unwrap),
  /** class_ids: every class id, in the new order. */
  reorder: (classIds) => api.put('/classes/order', { class_ids: classIds }).then(unwrap),
  remove: (id) => api.delete(`/classes/${id}`).then(unwrap),
}

export const sectionsApi = {
  /** Resolves to `{ sections, meta }`. */
  list: (params, { signal } = {}) =>
    api.get('/sections', { params, signal }).then((res) => ({ sections: res.data.data, meta: res.data.meta })),
  get: (id) => api.get(`/sections/${id}`).then(unwrap),
  /** The section's class teacher and each subject's teacher ("assignment: null" = not assigned). */
  teachers: (id) => api.get(`/sections/${id}/teacher-assignments`).then(unwrap),
  create: (payload) => api.post('/sections', payload).then(unwrap),
  update: (id, payload) => api.put(`/sections/${id}`, payload).then(unwrap),
  setStatus: (id, status) => api.patch(`/sections/${id}/status`, { status }).then(unwrap),
  remove: (id) => api.delete(`/sections/${id}`).then(unwrap),
}

export const subjectsApi = {
  /** Resolves to `{ subjects, meta }`. */
  list: (params, { signal } = {}) =>
    api.get('/subjects', { params, signal }).then((res) => ({ subjects: res.data.data, meta: res.data.meta })),
  stats: () => api.get('/subjects/stats').then(unwrap),
  get: (id) => api.get(`/subjects/${id}`).then(unwrap),
  /** Every class assignment of the subject, newest academic year first. */
  classes: (id, params) => api.get(`/subjects/${id}/classes`, { params }).then(unwrap),
  create: (payload) => api.post('/subjects', payload).then(unwrap),
  update: (id, payload) => api.put(`/subjects/${id}`, payload).then(unwrap),
  setStatus: (id, status) => api.patch(`/subjects/${id}/status`, { status }).then(unwrap),
  remove: (id) => api.delete(`/subjects/${id}`).then(unwrap),
}

export const classSubjectsApi = {
  /** Resolves to `{ assignments, meta }`. */
  list: (params, { signal } = {}) =>
    api.get('/class-subjects', { params, signal }).then((res) => ({ assignments: res.data.data, meta: res.data.meta })),
  get: (id) => api.get(`/class-subjects/${id}`).then(unwrap),
  /** payload: `{ class_id, academic_year_id, subjects: [{ subject_id, is_compulsory }] }` — all or nothing. */
  assign: (payload) => api.post('/class-subjects', payload).then(unwrap),
  setCompulsory: (id, isCompulsory) => api.put(`/class-subjects/${id}`, { is_compulsory: isCompulsory }).then(unwrap),
  remove: (id) => api.delete(`/class-subjects/${id}`).then(unwrap),
}

export const enrollmentsApi = {
  /** Resolves to `{ enrollments, meta }`. */
  list: (params, { signal } = {}) =>
    api.get('/enrollments', { params, signal }).then((res) => ({ enrollments: res.data.data, meta: res.data.meta })),
  /** Counts for one academic year (default: the current year). */
  stats: (params) => api.get('/enrollments/stats', { params }).then(unwrap),
  /** Active students not enrolled in `academic_year_id`; resolves to `{ students, meta }`. */
  unenrolled: (params, { signal } = {}) =>
    api.get('/enrollments/unenrolled-students', { params, signal }).then((res) => ({ students: res.data.data, meta: res.data.meta })),
  get: (id) => api.get(`/enrollments/${id}`).then(unwrap),
  create: (payload) => api.post('/enrollments', payload).then(unwrap),
  /** Bulk enrollment report; `dry_run: true` validates and assigns rolls without saving. */
  bulk: (payload) => api.post('/enrollments/bulk', payload).then(unwrap),
  update: (id, payload) => api.put(`/enrollments/${id}`, payload).then(unwrap),
  setStatus: (id, status, reason) => api.patch(`/enrollments/${id}/status`, { status, reason }).then(unwrap),
  remove: (id) => api.delete(`/enrollments/${id}`).then(unwrap),
}

export const promotionsApi = {
  /** Resolves to `{ batches, meta }`. */
  list: (params, { signal } = {}) =>
    api.get('/promotions', { params, signal }).then((res) => ({ batches: res.data.data, meta: res.data.meta })),
  get: (id) => api.get(`/promotions/${id}`).then(unwrap),
  /** Computes the outcome without saving anything. */
  preview: (payload) => api.post('/promotions/preview', payload).then(unwrap),
  /** Resolves to `{ batch, replayed }`; `request_id` makes retries safe. */
  execute: (payload) => api.post('/promotions/execute', payload).then((res) => ({ batch: res.data.data, replayed: !!res.data.meta?.replayed })),
}

export const teacherAssignmentsApi = {
  /** Resolves to `{ assignments, meta }`. */
  list: (params, { signal } = {}) =>
    api.get('/teacher-assignments', { params, signal }).then((res) => ({ assignments: res.data.data, meta: res.data.meta })),
  stats: (params) => api.get('/teacher-assignments/stats', { params }).then(unwrap),
  /** The assignment with `history` (every assignment of the same slot). */
  get: (id) => api.get(`/teacher-assignments/${id}`).then(unwrap),
  /** `assignment_type`: subject_teacher (with subject_id) or class_teacher; `replace: true` confirms replacing the current teacher. */
  create: (payload) => api.post('/teacher-assignments', payload).then(unwrap),
  /** Bulk report; `dry_run: true` previews, `replace_section_ids` resolves conflicts. */
  bulk: (payload) => api.post('/teacher-assignments/bulk', payload).then(unwrap),
  /** A different teacher_id changes the teacher (the old assignment is kept as history). */
  update: (id, payload) => api.put(`/teacher-assignments/${id}`, payload).then(unwrap),
  setStatus: (id, status, reason) => api.patch(`/teacher-assignments/${id}/status`, { status, reason }).then(unwrap),
  remove: (id) => api.delete(`/teacher-assignments/${id}`).then(unwrap),
}

export const attendanceApi = {
  /** Years and sections the caller can take attendance for (teachers: their assigned sections). */
  options: (params) => api.get('/attendance/options', { params }).then(unwrap),
  /** Resolves to `{ records, meta }`. */
  list: (params, { signal } = {}) =>
    api.get('/attendance', { params, signal }).then((res) => ({ records: res.data.data, meta: res.data.meta })),
  /** A record with `history` (audited changes) and what the caller may do. */
  get: (id) => api.get(`/attendance/${id}`).then(unwrap),
  /** A section's roster and attendance for `{ academic_year_id, section_id, date }`. */
  session: (params) => api.get('/attendance/session', { params }).then(unwrap),
  /** A date across sections: totals, per class and per section. */
  summary: (params, { signal } = {}) => api.get('/attendance/session/summary', { params, signal }).then(unwrap),
  /** Saves a whole section for one date; resolves to `{ session, created, updated, unchanged }`. */
  saveSession: (payload) => api.post('/attendance/bulk', payload).then(unwrap),
  /** One student, one day. */
  mark: (payload) => api.post('/attendance', payload).then(unwrap),
  /** `{ status, remarks, reason, version }` — `version` guards against overwriting someone else's change. */
  update: (id, payload) => api.put(`/attendance/${id}`, payload).then(unwrap),
  setStatus: (id, payload) => api.patch(`/attendance/${id}/status`, payload).then(unwrap),
  reports: {
    daily: (params) => api.get('/attendance/reports/daily', { params }).then(unwrap),
    monthly: (params) => api.get('/attendance/reports/monthly', { params }).then(unwrap),
    classWise: (params) => api.get('/attendance/reports/class-wise', { params }).then(unwrap),
    /** Resolves to `{ summary, records, meta }`. */
    studentWise: (params) =>
      api.get('/attendance/reports/student-wise', { params }).then((res) => ({ ...res.data.data, meta: res.data.meta })),
  },
  /** Downloads a CSV export (`report`: daily, monthly, class-wise, student or records). */
  export: (params) => api.get('/attendance/export', { params, responseType: 'blob', timeout: 60000 }),
}

export const examinationsApi = {
  /** Resolves to `{ exams, meta }`. */
  list: (params, { signal } = {}) =>
    api.get('/examinations', { params, signal }).then((res) => ({ exams: res.data.data, meta: res.data.meta })),
  /** Counts for one academic year (default: current). */
  stats: (params) => api.get('/examinations/statistics', { params }).then(unwrap),
  /** The examination with `classes` (subjects the class takes, and its schedule). */
  get: (id) => api.get(`/examinations/${id}`).then(unwrap),
  create: (payload) => api.post('/examinations', payload).then(unwrap),
  /** `reason` is required once the examination is published. */
  update: (id, payload) => api.put(`/examinations/${id}`, payload).then(unwrap),
  remove: (id) => api.delete(`/examinations/${id}`).then(unwrap),
  history: (id) => api.get(`/examinations/${id}/history`).then(unwrap),
  addClasses: (id, classIds) => api.post(`/examinations/${id}/classes`, { class_ids: classIds }).then(unwrap),
  removeClass: (id, classId) => api.delete(`/examinations/${id}/classes/${classId}`).then(unwrap),
  /** Saves a class's whole schedule: `items` with `id`/`version` for existing papers. */
  saveClassSchedule: (id, classId, payload) => api.put(`/examinations/${id}/classes/${classId}/schedules`, payload).then(unwrap),
  updateSchedule: (scheduleId, payload) => api.put(`/examination-schedules/${scheduleId}`, payload).then(unwrap),
  removeSchedule: (scheduleId) => api.delete(`/examination-schedules/${scheduleId}`).then(unwrap),
  /** `{ ready, issues, warnings, classes, papers }` without changing anything. */
  publishCheck: (id) => api.get(`/examinations/${id}/publish-check`).then(unwrap),
  publish: (id) => api.patch(`/examinations/${id}/publish`).then(unwrap),
  complete: (id) => api.patch(`/examinations/${id}/complete`).then(unwrap),
  cancel: (id, reason) => api.patch(`/examinations/${id}/cancel`, { reason }).then(unwrap),
  /** Timetable for admins and (read-only, their classes) teachers. */
  timetable: (id, params) => api.get(`/examinations/${id}/timetable`, { params }).then(unwrap),
  /** Published/past examinations the caller can see; resolves to `{ exams, meta }`. */
  timetables: (params, { signal } = {}) =>
    api.get('/exam-timetables', { params, signal }).then((res) => ({ exams: res.data.data, meta: res.data.meta })),
}

export const examTypesApi = {
  list: () => api.get('/exam-types').then(unwrap),
  create: (payload) => api.post('/exam-types', payload).then(unwrap),
  update: (id, payload) => api.put(`/exam-types/${id}`, payload).then(unwrap),
  remove: (id) => api.delete(`/exam-types/${id}`).then(unwrap),
}

export const usersApi = {
  /** Resolves to `{ users, meta }`. */
  list: (params, { signal } = {}) =>
    api.get('/users', { params, signal }).then((res) => ({ users: res.data.data, meta: res.data.meta })),
  stats: () => api.get('/users/stats').then(unwrap),
  get: (id) => api.get(`/users/${id}`).then(unwrap),
  create: (payload) => api.post('/users', payload).then(unwrap),
  update: (id, payload) => api.patch(`/users/${id}`, payload).then(unwrap),
  changeRole: (id, role) => api.patch(`/users/${id}/role`, { role }).then(unwrap),
  changeStatus: (id, status) => api.patch(`/users/${id}/status`, { status }).then(unwrap),
  resetPassword: (id, payload) => api.put(`/users/${id}/password`, payload).then(unwrap),
}
