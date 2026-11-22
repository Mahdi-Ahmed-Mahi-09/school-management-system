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
