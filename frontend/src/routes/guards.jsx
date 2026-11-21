import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'
import { FullScreenLoader } from '../components/ui/Spinner'

/** Renders children only for signed-in users; otherwise redirects to /login. */
export function RequireAuth() {
  const { status } = useAuth()
  const location = useLocation()
  if (status === 'loading') return <FullScreenLoader />
  if (status !== 'authenticated') {
    return <Navigate to="/login" replace state={{ from: location.pathname + location.search }} />
  }
  return <Outlet />
}

/**
 * Restricts a route to users holding a permission and/or role.
 * This is a UX convenience — the API enforces the same rules server-side.
 */
export function RequireAccess({ permission, roles }) {
  const { can, hasRole } = useAuth()
  const allowed = (!permission || can(permission)) && (!roles || hasRole(...roles))
  if (!allowed) return <Navigate to="/forbidden" replace />
  return <Outlet />
}

/** Keeps signed-in users away from the login page. */
export function GuestOnly() {
  const { status } = useAuth()
  const location = useLocation()
  if (status === 'loading') return <FullScreenLoader />
  if (status === 'authenticated') return <Navigate to={location.state?.from || '/'} replace />
  return <Outlet />
}
