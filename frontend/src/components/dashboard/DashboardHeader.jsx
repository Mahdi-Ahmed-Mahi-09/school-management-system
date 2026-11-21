import { useState } from 'react'
import { Link } from 'react-router-dom'
import { CalendarDays, RefreshCw, UserPlus } from 'lucide-react'
import { useAuth } from '../../hooks/useAuth'
import { PERMISSIONS } from '../../lib/roles'
import { cn, formatLongDate, formatRelative, greeting } from '../../lib/utils'
import { Button } from '../ui/Button'
import { buttonClasses } from '../ui/buttonStyles'

export function DashboardHeader({ generatedAt, refreshing, onRefresh, disabled }) {
  const { user, can } = useAuth()
  const [today] = useState(() => new Date())
  return (
    <header className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
      <div className="min-w-0">
        <p className="flex items-center gap-1.5 text-[13px] font-medium text-slate-500">
          <CalendarDays className="size-4 text-brand-500" aria-hidden="true" />
          <time dateTime={today.toLocaleDateString('en-CA')}>{formatLongDate(today)}</time>
        </p>
        <h1 className="mt-2 text-2xl font-semibold tracking-tight text-slate-900 sm:text-[28px]">
          {greeting()}, {user.first_name}
        </h1>
        <p className="mt-1 max-w-xl text-sm text-slate-500">
          Your school at a glance — people, academics, attendance and finances.
        </p>
      </div>
      <div className="flex flex-wrap items-center gap-2 sm:gap-3">
        {generatedAt && (
          <span className="mr-1 text-xs text-slate-400" aria-live="polite">
            Updated {formatRelative(generatedAt).toLowerCase()}
          </span>
        )}
        <Button
          variant="secondary"
          icon={RefreshCw}
          onClick={onRefresh}
          disabled={disabled || refreshing}
          className={cn(refreshing && '[&_svg]:animate-spin')}
        >
          {refreshing ? 'Refreshing…' : 'Refresh'}
        </Button>
        {can(PERMISSIONS.USERS_CREATE) && (
          <Link to="/users?create=1" className={buttonClasses('primary')}>
            <UserPlus className="size-4" aria-hidden="true" />
            Add user
          </Link>
        )}
      </div>
    </header>
  )
}
