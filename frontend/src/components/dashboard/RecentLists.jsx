import { Link } from 'react-router-dom'
import {
  Activity,
  KeyRound,
  Megaphone,
  PencilLine,
  ShieldCheck,
  UserCheck,
  UserPlus,
  UserRoundPlus,
  UserX,
} from 'lucide-react'
import { ROLE_META } from '../../lib/roles'
import { cn, formatDate, formatDateTime, formatRelative } from '../../lib/utils'
import { Avatar, Badge } from '../ui/Display'
import { ModuleUnavailable, NoData, Panel } from './Panel'

const linkClass = 'text-[13px] font-medium text-brand-600 hover:text-brand-700'

/** Newly admitted students (students module). */
export function RecentAdmissions({ section, className }) {
  return (
    <Panel icon={UserRoundPlus} title="Recent admissions" description="Students admitted most recently." className={className}>
      {section?.status !== 'available' ? (
        <ModuleUnavailable
          module={section?.module}
          icon={UserRoundPlus}
          title="Admissions aren't recorded yet"
          description="New students with admission number, class and section will be listed here."
          action={
            <Link to="/users?role=student" className={linkClass}>
              View student accounts
            </Link>
          }
        />
      ) : section.data.length === 0 ? (
        <NoData icon={UserRoundPlus} title="No admissions yet" />
      ) : (
        <ul className="-mx-2 divide-y divide-slate-100">
          {section.data.map((a) => {
            const [first, ...rest] = a.full_name.split(' ')
            return (
              <li key={a.id} className="flex items-center gap-3 px-2 py-3">
                <Avatar user={{ id: a.id, first_name: first, last_name: rest.join(' ') }} size="sm" />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-slate-900">{a.full_name}</p>
                  <p className="truncate text-xs text-slate-500">
                    #{a.admission_number} · {a.class_name}
                    {a.section_name && ` · ${a.section_name}`}
                  </p>
                </div>
                <div className="text-right">
                  <Badge className="bg-slate-50 text-slate-600 ring-slate-500/15 capitalize">{a.status}</Badge>
                  <p className="mt-1 text-xs text-slate-400">{formatDate(a.admitted_on)}</p>
                </div>
              </li>
            )
          })}
        </ul>
      )}
    </Panel>
  )
}

/** Recently published notices (notices module). */
export function RecentNotices({ section, className }) {
  return (
    <Panel icon={Megaphone} title="Recent notices" description="Latest announcements to your school." className={className}>
      {section?.status !== 'available' ? (
        <ModuleUnavailable
          module={section?.module}
          icon={Megaphone}
          title="No notice board yet"
          description="Published notices with their audience and date will be listed here."
        />
      ) : section.data.length === 0 ? (
        <NoData icon={Megaphone} title="No notices published" />
      ) : (
        <ul className="space-y-3">
          {section.data.map((n) => (
            <li key={n.id} className="rounded-lg border border-slate-200/80 p-3">
              <div className="flex items-start justify-between gap-2">
                <p className="line-clamp-1 text-sm font-medium text-slate-900">{n.title}</p>
                <Badge className="bg-brand-50 text-brand-700 ring-brand-600/15 capitalize">{n.audience}</Badge>
              </div>
              <p className="mt-1 line-clamp-2 text-[13px] text-slate-500">{n.preview}</p>
              <p className="mt-2 text-xs text-slate-400">{formatDate(n.published_at)}</p>
            </li>
          ))}
        </ul>
      )}
    </Panel>
  )
}

const ACTIVITY_META = {
  'user.created': { icon: UserPlus, className: 'bg-emerald-50 text-emerald-600 ring-emerald-100' },
  'user.updated': { icon: PencilLine, className: 'bg-sky-50 text-sky-600 ring-sky-100' },
  'user.role_changed': { icon: ShieldCheck, className: 'bg-violet-50 text-violet-600 ring-violet-100' },
  'user.status_changed': { icon: UserCheck, className: 'bg-amber-50 text-amber-600 ring-amber-100' },
  'user.password_reset': { icon: KeyRound, className: 'bg-rose-50 text-rose-600 ring-rose-100' },
}

function activityMeta(item) {
  if (item.action === 'user.status_changed' && item.metadata?.to === 'inactive') {
    return { icon: UserX, className: 'bg-slate-100 text-slate-600 ring-slate-200' }
  }
  return ACTIVITY_META[item.action] ?? { icon: Activity, className: 'bg-slate-50 text-slate-500 ring-slate-200' }
}

/** Administrative activity feed (activity_logs). */
export function RecentActivity({ section, className }) {
  return (
    <Panel
      icon={Activity}
      title="Recent activity"
      description="Latest administrative actions."
      className={className}
      action={
        section?.status === 'available' &&
        section.data.length > 0 && (
          <Link to="/users" className={linkClass}>
            Manage users
          </Link>
        )
      }
    >
      {section?.status !== 'available' ? (
        <ModuleUnavailable module={section?.module} icon={Activity} title="Activity unavailable" description="Administrative actions will appear here." />
      ) : section.data.length === 0 ? (
        <NoData icon={Activity} title="No activity yet" description="Actions such as creating users or changing roles will be listed here." />
      ) : (
        <ol className="relative space-y-4">
          {section.data.map((item, i) => {
            const { icon: Icon, className: tone } = activityMeta(item)
            return (
              <li key={item.id} className="relative flex gap-3">
                {i < section.data.length - 1 && (
                  <span className="absolute top-8 bottom-[-16px] left-[15px] w-px bg-slate-200" aria-hidden="true" />
                )}
                <span className={cn('relative flex size-8 shrink-0 items-center justify-center rounded-full ring-1', tone)}>
                  <Icon className="size-4" aria-hidden="true" />
                </span>
                <div className="min-w-0 flex-1 pt-0.5">
                  <p className="text-sm text-slate-800">{item.summary}</p>
                  <p className="mt-0.5 text-xs text-slate-500">
                    {item.actor ? (
                      <>
                        by <span className="font-medium text-slate-600">{item.actor.name}</span>
                        {ROLE_META[item.actor.role] && <span className="text-slate-400"> · {ROLE_META[item.actor.role].label}</span>}
                      </>
                    ) : (
                      'by System'
                    )}
                    <span className="text-slate-300"> · </span>
                    <time dateTime={item.created_at} title={formatDateTime(item.created_at)}>
                      {formatRelative(item.created_at)}
                    </time>
                  </p>
                </div>
              </li>
            )
          })}
        </ol>
      )}
    </Panel>
  )
}
