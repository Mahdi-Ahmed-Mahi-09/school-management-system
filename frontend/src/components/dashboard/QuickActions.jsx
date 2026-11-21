import { Link } from 'react-router-dom'
import { ClipboardCheck, FilePenLine, GraduationCap, Megaphone, Presentation, Receipt, Users, Zap } from 'lucide-react'
import { useAuth } from '../../hooks/useAuth'
import { PERMISSIONS } from '../../lib/roles'
import { cn } from '../../lib/utils'
import { Panel } from './Panel'

/**
 * Every action names the module it needs. Actions for modules that are not
 * implemented are shown disabled — they never link to a placeholder page.
 */
const ACTIONS = [
  { label: 'Add student account', icon: GraduationCap, module: 'users', to: '/users?create=student', permission: PERMISSIONS.USERS_CREATE },
  { label: 'Add teacher account', icon: Presentation, module: 'users', to: '/users?create=teacher', permission: PERMISSIONS.USERS_CREATE },
  { label: 'Manage users', icon: Users, module: 'users', to: '/users', permission: PERMISSIONS.USERS_READ },
  { label: 'Take attendance', icon: ClipboardCheck, module: 'attendance' },
  { label: 'Create examination', icon: FilePenLine, module: 'exams' },
  { label: 'Record payment', icon: Receipt, module: 'fees' },
  { label: 'Publish notice', icon: Megaphone, module: 'notices' },
]

export function QuickActions({ modules, className }) {
  const { can } = useAuth()
  const byKey = Object.fromEntries((modules ?? []).map((m) => [m.key, m]))

  return (
    <Panel icon={Zap} title="Quick actions" description="Common tasks, one click away." className={className}>
      <ul className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-1">
        {ACTIONS.filter((a) => !a.permission || can(a.permission)).map(({ label, icon: Icon, module, to }) => {
          const mod = byKey[module]
          const ready = mod?.available && to
          const content = (
            <>
              <span
                className={cn(
                  'flex size-8 shrink-0 items-center justify-center rounded-lg ring-1',
                  ready ? 'bg-brand-50 text-brand-600 ring-brand-100' : 'bg-slate-50 text-slate-400 ring-slate-200',
                )}
              >
                <Icon className="size-4" aria-hidden="true" />
              </span>
              <span className="min-w-0 flex-1">
                <span className={cn('block truncate text-sm font-medium', ready ? 'text-slate-800' : 'text-slate-400')}>{label}</span>
                {!ready && <span className="block truncate text-xs text-slate-400">Needs {mod?.name ?? 'a future module'}</span>}
              </span>
              {!ready && (
                <span className="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-medium whitespace-nowrap text-slate-500">Soon</span>
              )}
            </>
          )
          return (
            <li key={label}>
              {ready ? (
                <Link
                  to={to}
                  className="flex items-center gap-3 rounded-lg border border-slate-200/80 px-3 py-2.5 transition hover:border-brand-200 hover:bg-brand-50/40 focus-visible:ring-2 focus-visible:ring-brand-500/40"
                >
                  {content}
                </Link>
              ) : (
                <div
                  className="flex cursor-not-allowed items-center gap-3 rounded-lg border border-dashed border-slate-200 px-3 py-2.5"
                  aria-disabled="true"
                  title={`${label} becomes available with ${mod?.name ?? 'a future module'}.`}
                >
                  {content}
                </div>
              )}
            </li>
          )
        })}
      </ul>
    </Panel>
  )
}
