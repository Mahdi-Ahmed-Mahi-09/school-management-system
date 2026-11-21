// Shared Recharts styling so every dashboard chart looks consistent.

export const ROLE_COLORS = {
  students: '#10b981', // emerald-500 — matches the Student badge
  teachers: '#0ea5e9', // sky-500 — Teacher
  guardians: '#f59e0b', // amber-500 — Guardian
  admins: '#8b5cf6', // violet-500 — Admin
}

export const ATTENDANCE_COLORS = {
  present: '#10b981',
  late: '#f59e0b',
  leave: '#0ea5e9',
  absent: '#f43f5e',
}

export const BRAND = '#4f46e5'

export const axisProps = {
  tickLine: false,
  axisLine: false,
  tick: { fill: '#64748b', fontSize: 12 },
}

export const gridProps = { stroke: '#e2e8f0', strokeDasharray: '3 3', vertical: false }
