import axios from 'axios'

/**
 * Shared Axios instance. The session lives in an HttpOnly cookie that the
 * browser sends automatically; JavaScript never sees the token.
 */
export const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1',
  withCredentials: true,
  timeout: 15000,
  headers: {
    'Content-Type': 'application/json',
    // Required by the API's CSRF guard on state-changing requests.
    'X-Requested-With': 'XMLHttpRequest',
  },
})

export const SESSION_ENDED_EVENT = 'classora:session-ended'
const SESSION_CODES = new Set(['AUTH_REQUIRED', 'SESSION_EXPIRED', 'SESSION_INVALID', 'ACCOUNT_INACTIVE'])

api.interceptors.response.use(
  (response) => response,
  (error) => {
    const status = error.response?.status
    const code = error.response?.data?.error?.code
    const url = error.config?.url ?? ''
    // A protected call failed because the session ended: let the auth layer react.
    if (status === 401 && SESSION_CODES.has(code) && !url.startsWith('/auth/')) {
      window.dispatchEvent(new CustomEvent(SESSION_ENDED_EVENT, { detail: { code } }))
    }
    return Promise.reject(error)
  },
)

/** Returns the `data` field from the API envelope. */
export const unwrap = (response) => response.data.data

/** Returns a user-facing message for any request error. */
export function getErrorMessage(error, fallback = 'Something went wrong. Please try again.') {
  if (error?.response?.data?.error?.message) return error.response.data.error.message
  if (error?.code === 'ECONNABORTED') return 'The server took too long to respond. Please try again.'
  if (error?.request && !error.response) return 'Unable to reach the server. Check your connection and try again.'
  return fallback
}

/** Returns the API error code, if any. */
export const getErrorCode = (error) => error?.response?.data?.error?.code

/** Returns per-field validation messages, if any. */
export const getFieldErrors = (error) => error?.response?.data?.error?.details ?? {}

/**
 * Copies server-side field errors into a react-hook-form instance.
 * Returns true when at least one field error was applied.
 */
export function applyFieldErrors(error, setError, allowedFields) {
  const details = getFieldErrors(error)
  let applied = false
  for (const [field, message] of Object.entries(details)) {
    if (!allowedFields || allowedFields.includes(field)) {
      setError(field, { type: 'server', message })
      applied = true
    }
  }
  return applied
}
