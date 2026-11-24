import { Link } from 'react-router-dom'
import {
  Activity,
  BookOpen,
  BookPlus,
  ClipboardList,
  Sparkles,
  Unlink,
  CalendarCheck2,
  CalendarPlus,
  Camera,
  CircleCheckBig,
  LayoutGrid,
  Power,
  Rows3,
  ArrowUpDown,
  GraduationCap,
  KeyRound,
  Megaphone,
  PencilLine,
  Presentation,
  ShieldCheck,
  Trash2,
  UserCheck,
  UserPlus,
  UserRoundPlus,
  UserX,
} from 'lucide-react'
import { ROLE_META } from '../../lib/roles'
import { cn, formatDate, formatDateTime, formatRelative } from '../../lib/utils'
import { formatDateOnly } from '../../lib/students'
import { PersonAvatar } from '../ui/PersonAvatar'
import { Badge, StatusBadge } from '../ui/Display'
import { ModuleUnavailable, NoData, Panel } from './Panel'

const linkClass = 'text-[13px] font-medium text-brand-600 hover:text-brand-700'

/** Newly admitted students (students module). */
export function RecentAdmissions({ section, className }) {
  const available = section?.status === 'available'
  return (
    <Panel
      icon={UserRoundPlus}
      title="Recent admissions"
      description="Students admitted most recently."
      className={className}
      action={
        available && (
          <Link to="/students" className={linkClass}>
            View all
          </Link>
        )
      }
    >
      {!available ? (
        <ModuleUnavailable
          module={section?.module}
          icon={UserRoundPlus}
          title="Admissions aren't recorded yet"
          description="Newly admitted students will be listed here."
        />
      ) : section.data.length === 0 ? (
        <NoData
          icon={UserRoundPlus}
          title="No admissions yet"
          description="Students appear here once they're added with an admission date."
        />
      ) : (
        <ul className="-mx-2 divide-y divide-slate-100">
          {section.data.map((a) => (
            <li key={a.id}>
              <Link to={`/students/${a.id}`} className="flex items-center gap-3 rounded-lg px-2 py-3 transition-colors hover:bg-slate-50">
                <PersonAvatar person={a} size="sm" />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-slate-900">{a.full_name}</p>
                  <p className="truncate text-xs text-slate-500">
                    <span className="font-mono">{a.admission_number}</span>
                    {a.class_name && ` · ${a.class_name}`}
                    {a.section_name && ` · ${a.section_name}`}
                  </p>
                </div>
                <div className="shrink-0 text-right">
                  <StatusBadge status={a.status} />
                  <p className="mt-1 text-xs text-slate-400">{formatDateOnly(a.admitted_on)}</p>
                </div>
              </Link>
            </li>
          ))}
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
  'student.created': { icon: GraduationCap, className: 'bg-emerald-50 text-emerald-600 ring-emerald-100' },
  'student.updated': { icon: PencilLine, className: 'bg-sky-50 text-sky-600 ring-sky-100' },
  'student.deleted': { icon: Trash2, className: 'bg-rose-50 text-rose-600 ring-rose-100' },
  'student.photo_updated': { icon: Camera, className: 'bg-violet-50 text-violet-600 ring-violet-100' },
  'student.photo_removed': { icon: Camera, className: 'bg-slate-100 text-slate-600 ring-slate-200' },
  'teacher.created': { icon: Presentation, className: 'bg-emerald-50 text-emerald-600 ring-emerald-100' },
  'teacher.updated': { icon: PencilLine, className: 'bg-sky-50 text-sky-600 ring-sky-100' },
  'teacher.deleted': { icon: Trash2, className: 'bg-rose-50 text-rose-600 ring-rose-100' },
  'teacher.photo_updated': { icon: Camera, className: 'bg-violet-50 text-violet-600 ring-violet-100' },
  'teacher.photo_removed': { icon: Camera, className: 'bg-slate-100 text-slate-600 ring-slate-200' },
  'academic_year.created': { icon: CalendarPlus, className: 'bg-emerald-50 text-emerald-600 ring-emerald-100' },
  'academic_year.updated': { icon: PencilLine, className: 'bg-sky-50 text-sky-600 ring-sky-100' },
  'academic_year.activated': { icon: CalendarCheck2, className: 'bg-brand-50 text-brand-600 ring-brand-100' },
  'academic_year.completed': { icon: CircleCheckBig, className: 'bg-slate-100 text-slate-600 ring-slate-200' },
  'academic_year.deleted': { icon: Trash2, className: 'bg-rose-50 text-rose-600 ring-rose-100' },
  'class.created': { icon: LayoutGrid, className: 'bg-emerald-50 text-emerald-600 ring-emerald-100' },
  'class.updated': { icon: PencilLine, className: 'bg-sky-50 text-sky-600 ring-sky-100' },
  'class.status_changed': { icon: Power, className: 'bg-amber-50 text-amber-600 ring-amber-100' },
  'class.reordered': { icon: ArrowUpDown, className: 'bg-slate-100 text-slate-600 ring-slate-200' },
  'class.deleted': { icon: Trash2, className: 'bg-rose-50 text-rose-600 ring-rose-100' },
  'section.created': { icon: Rows3, className: 'bg-emerald-50 text-emerald-600 ring-emerald-100' },
  'section.updated': { icon: PencilLine, className: 'bg-sky-50 text-sky-600 ring-sky-100' },
  'section.status_changed': { icon: Power, className: 'bg-amber-50 text-amber-600 ring-amber-100' },
  'section.deleted': { icon: Trash2, className: 'bg-rose-50 text-rose-600 ring-rose-100' },
  'subject.created': { icon: BookOpen, className: 'bg-emerald-50 text-emerald-600 ring-emerald-100' },
  'subject.updated': { icon: PencilLine, className: 'bg-sky-50 text-sky-600 ring-sky-100' },
  'subject.status_changed': { icon: Power, className: 'bg-amber-50 text-amber-600 ring-amber-100' },
  'subject.deleted': { icon: Trash2, className: 'bg-rose-50 text-rose-600 ring-rose-100' },
  'class_subject.assigned': { icon: BookPlus, className: 'bg-brand-50 text-brand-600 ring-brand-100' },
  'class_subject.updated': { icon: PencilLine, className: 'bg-sky-50 text-sky-600 ring-sky-100' },
  'enrollment.created': { icon: ClipboardList, className: 'bg-emerald-50 text-emerald-600 ring-emerald-100' },
  'enrollment.bulk_created': { icon: ClipboardList, className: 'bg-emerald-50 text-emerald-600 ring-emerald-100' },
  'enrollment.updated': { icon: PencilLine, className: 'bg-sky-50 text-sky-600 ring-sky-100' },
  'enrollment.status_changed': { icon: Power, className: 'bg-amber-50 text-amber-600 ring-amber-100' },
  'enrollment.deleted': { icon: Trash2, className: 'bg-rose-50 text-rose-600 ring-rose-100' },
  'promotion.executed': { icon: Sparkles, className: 'bg-brand-50 text-brand-600 ring-brand-100' },
  'class_subject.removed': { icon: Unlink, className: 'bg-slate-100 text-slate-600 ring-slate-200' },
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
