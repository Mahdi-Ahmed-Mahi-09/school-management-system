import { useCallback, useEffect, useMemo, useState } from 'react'
import { SESSION_ENDED_EVENT } from '../api/client'
import { authApi } from '../api/services'
import { AuthContext } from './authContextObject'

const SESSION_MESSAGES = {
  SESSION_EXPIRED: 'Your session has expired. Please sign in again.',
  SESSION_INVALID: 'Your session is no longer valid. Please sign in again.',
  ACCOUNT_INACTIVE: 'Your account has been deactivated. Please contact your school administrator.',
}

/**
 * Holds the signed-in user. The session itself is an HttpOnly cookie, so on
 * load we ask the API who we are rather than reading any token in JS.
 */
export function AuthProvider({ children }) {
  const [state, setState] = useState({ status: 'loading', user: null, permissions: [] })
  const [notice, setNotice] = useState(null)

  const applySession = useCallback((payload) => {
    setState({ status: 'authenticated', user: payload.user, permissions: payload.permissions ?? [] })
  }, [])

  const clearSession = useCallback(() => {
    setState({ status: 'unauthenticated', user: null, permissions: [] })
  }, [])

  const refresh = useCallback(async () => {
    try {
      applySession(await authApi.me())
    } catch {
      clearSession()
    }
  }, [applySession, clearSession])

  useEffect(() => {
    refresh()
  }, [refresh])

  // React to any protected request discovering that the session ended.
  useEffect(() => {
    const onEnded = (event) => {
      setNotice(SESSION_MESSAGES[event.detail?.code] ?? null)
      clearSession()
    }
    window.addEventListener(SESSION_ENDED_EVENT, onEnded)
    return () => window.removeEventListener(SESSION_ENDED_EVENT, onEnded)
  }, [clearSession])

  const login = useCallback(
    async ({ identifier, password, rememberMe }) => {
      const payload = await authApi.login({ identifier, password, remember_me: rememberMe })
      setNotice(null)
      applySession(payload)
      return payload.user
    },
    [applySession],
  )

  const logout = useCallback(async () => {
    try {
      await authApi.logout()
    } finally {
      setNotice(null)
      clearSession()
    }
  }, [clearSession])

  const updateUser = useCallback((user) => {
    setState((s) => (s.user ? { ...s, user } : s))
  }, [])

  const can = useCallback((permission) => state.permissions.includes(permission), [state.permissions])
  const hasRole = useCallback((...roles) => !!state.user && roles.includes(state.user.role), [state.user])

  const value = useMemo(
    () => ({
      ...state,
      isAuthenticated: state.status === 'authenticated',
      notice,
      clearNotice: () => setNotice(null),
      login,
      logout,
      refresh,
      applySession,
      updateUser,
      can,
      hasRole,
    }),
    [state, notice, login, logout, refresh, applySession, updateUser, can, hasRole],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
