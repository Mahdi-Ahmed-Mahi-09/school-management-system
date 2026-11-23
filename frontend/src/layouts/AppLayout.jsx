import { Suspense, useEffect, useState } from 'react'
import { Link, NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { BookOpen, CalendarRange, ChevronsUpDown, ListChecks, GraduationCap, LayoutGrid, Rows3, KeyRound, Presentation, LayoutDashboard, LogOut, Menu as MenuIcon, UserRound, Users, X } from 'lucide-react'
import { Wordmark } from '../components/brand/Logo'
import { Alert, Avatar } from '../components/ui/Display'
import { Menu } from '../components/ui/Menu'
import { Spinner } from '../components/ui/Spinner'
import { useToast } from '../components/ui/Toast'
import { useAuth } from '../hooks/useAuth'
import { PERMISSIONS, ROLE_META } from '../lib/roles'
import { cn } from '../lib/utils'

const NAV_SECTIONS = [
  {
    label: 'Overview',
    items: [
      { to: '/', label: 'Dashboard', icon: LayoutDashboard, end: true },
      { to: '/profile', label: 'My profile', icon: UserRound, permission: PERMISSIONS.PROFILE_READ },
    ],
  },
  {
    label: 'School',
    items: [
      { to: '/academic-years', label: 'Academic years', icon: CalendarRange, permission: PERMISSIONS.ACADEMIC_YEARS_READ },
      { to: '/classes', label: 'Classes', icon: LayoutGrid, permission: PERMISSIONS.CLASSES_READ },
      { to: '/sections', label: 'Sections', icon: Rows3, permission: PERMISSIONS.SECTIONS_READ },
      { to: '/subjects', label: 'Subjects', icon: BookOpen, permission: PERMISSIONS.SUBJECTS_READ },
      { to: '/class-subjects', label: 'Class subjects', icon: ListChecks, permission: PERMISSIONS.SUBJECTS_READ },
      { to: '/students', label: 'Students', icon: GraduationCap, permission: PERMISSIONS.STUDENTS_READ },
      { to: '/teachers', label: 'Teachers', icon: Presentation, permission: PERMISSIONS.TEACHERS_READ },
    ],
  },
  {
    label: 'Administration',
    items: [{ to: '/users', label: 'Users', icon: Users, permission: PERMISSIONS.USERS_READ }],
  },
]

function Navigation({ onNavigate }) {
  const { can } = useAuth()
  return (
    <nav className="flex-1 space-y-6 overflow-y-auto px-3 py-5" aria-label="Main">
      {NAV_SECTIONS.map((section) => {
        const items = section.items.filter((i) => !i.permission || can(i.permission))
        if (!items.length) return null
        return (
          <div key={section.label}>
            <p className="px-3 pb-2 text-[11px] font-semibold tracking-wider text-slate-400 uppercase">{section.label}</p>
            <ul className="space-y-0.5">
              {items.map(({ to, label, icon: Icon, end }) => (
                <li key={to}>
                  <NavLink
                    to={to}
                    end={end}
                    onClick={onNavigate}
                    className={({ isActive }) =>
                      cn(
                        'group flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors',
                        isActive ? 'bg-brand-50 text-brand-700' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900',
                      )
                    }
                  >
                    {({ isActive }) => (
                      <>
                        <Icon
                          className={cn('size-[18px]', isActive ? 'text-brand-600' : 'text-slate-400 group-hover:text-slate-500')}
                          aria-hidden="true"
                        />
                        {label}
                      </>
                    )}
                  </NavLink>
                </li>
              ))}
            </ul>
          </div>
        )
      })}
    </nav>
  )
}

function UserMenu() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const toast = useToast()

  const signOut = async () => {
    try {
      await logout()
    } catch {
      // The session is cleared locally regardless.
    }
    toast.success('Signed out', 'See you next time.')
    navigate('/login', { replace: true })
  }

  return (
    <Menu
      align="start"
      label="Account menu"
      trigger={(props) => (
        <button
          {...props}
          type="button"
          className="flex w-full items-center gap-3 rounded-xl p-2 text-left transition-colors hover:bg-slate-100 focus-visible:ring-2 focus-visible:ring-brand-500/40"
        >
          <Avatar user={user} size="sm" />
          <span className="min-w-0 flex-1">
            <span className="block truncate text-sm font-medium text-slate-900">{user.full_name}</span>
            <span className="block truncate text-xs text-slate-500">{ROLE_META[user.role]?.label}</span>
          </span>
          <ChevronsUpDown className="size-4 text-slate-400" aria-hidden="true" />
        </button>
      )}
      items={[
        { label: 'My profile', icon: UserRound, onSelect: () => navigate('/profile') },
        { label: 'Change password', icon: KeyRound, onSelect: () => navigate('/profile?tab=security') },
        { separator: true },
        { label: 'Sign out', icon: LogOut, danger: true, onSelect: signOut },
      ]}
    />
  )
}

function SidebarContent({ onNavigate }) {
  return (
    <div className="flex h-full flex-col">
      <div className="flex h-16 items-center px-5">
        <Link to="/" onClick={onNavigate} className="rounded-lg focus-visible:ring-2 focus-visible:ring-brand-500/40">
          <Wordmark />
        </Link>
      </div>
      <Navigation onNavigate={onNavigate} />
      <div className="border-t border-slate-200/80 p-3">
        <UserMenu />
      </div>
    </div>
  )
}

export default function AppLayout() {
  const { user } = useAuth()
  const [mobileOpen, setMobileOpen] = useState(false)
  const location = useLocation()

  useEffect(() => {
    setMobileOpen(false)
    window.scrollTo(0, 0)
  }, [location.pathname])

  useEffect(() => {
    if (!mobileOpen) return
    const onKey = (e) => e.key === 'Escape' && setMobileOpen(false)
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [mobileOpen])

  const onSecurityTab = location.pathname === '/profile' && location.search.includes('tab=security')

  return (
    <div className="min-h-svh">
      <aside className="fixed inset-y-0 left-0 z-30 hidden w-64 border-r border-slate-200/80 bg-white lg:block">
        <SidebarContent />
      </aside>

      {mobileOpen && (
        <div className="fixed inset-0 z-40 lg:hidden" role="dialog" aria-modal="true" aria-label="Navigation">
          <div className="absolute inset-0 bg-slate-900/40 animate-fade-in" onClick={() => setMobileOpen(false)} aria-hidden="true" />
          <aside className="relative h-full w-72 max-w-[85vw] bg-white shadow-overlay animate-slide-in-left">
            <button
              type="button"
              onClick={() => setMobileOpen(false)}
              className="absolute top-4 right-3 flex size-8 items-center justify-center rounded-lg text-slate-500 hover:bg-slate-100"
              aria-label="Close navigation"
            >
              <X className="size-4" />
            </button>
            <SidebarContent onNavigate={() => setMobileOpen(false)} />
          </aside>
        </div>
      )}

      <div className="lg:pl-64">
        <header className="sticky top-0 z-20 flex h-14 items-center justify-between gap-3 border-b border-slate-200/80 bg-white/85 px-4 backdrop-blur lg:hidden">
          <button
            type="button"
            onClick={() => setMobileOpen(true)}
            className="-ml-1 flex size-9 items-center justify-center rounded-lg text-slate-600 hover:bg-slate-100"
            aria-label="Open navigation"
          >
            <MenuIcon className="size-5" />
          </button>
          <Wordmark />
          <Link to="/profile" aria-label="My profile">
            <Avatar user={user} size="sm" />
          </Link>
        </header>

        <main className="mx-auto w-full max-w-7xl px-4 py-6 sm:px-6 sm:py-8 lg:px-10 lg:py-10">
          {user.must_change_password && !onSecurityTab && (
            <Alert
              tone="warning"
              icon={KeyRound}
              title="Please set a new password"
              className="mb-6"
              action={
                <Link to="/profile?tab=security" className="shrink-0 text-sm font-semibold text-amber-900 underline-offset-4 hover:underline">
                  Change now
                </Link>
              }
            >
              An administrator set a temporary password for your account. Choose your own to keep your account secure.
            </Alert>
          )}
          <Suspense
            fallback={
              <div className="flex min-h-[50vh] items-center justify-center" role="status" aria-label="Loading page">
                <Spinner className="size-5 text-brand-600" />
              </div>
            }
          >
            <Outlet />
          </Suspense>
        </main>
      </div>
    </div>
  )
}
