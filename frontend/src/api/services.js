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
  create: (payload) => api.post('/sections', payload).then(unwrap),
  update: (id, payload) => api.put(`/sections/${id}`, payload).then(unwrap),
  setStatus: (id, status) => api.patch(`/sections/${id}/status`, { status }).then(unwrap),
  remove: (id) => api.delete(`/sections/${id}`).then(unwrap),
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
