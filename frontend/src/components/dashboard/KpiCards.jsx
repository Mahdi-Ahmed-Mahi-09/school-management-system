import { Link } from 'react-router-dom'
import { ArrowUpRight, GraduationCap, HeartHandshake, LayoutGrid, Presentation, Rows3 } from 'lucide-react'
import { cn, formatNumber } from '../../lib/utils'
import { Card } from '../ui/Display'

const tones = {
  emerald: 'bg-emerald-50 text-emerald-600 ring-emerald-100',
  sky: 'bg-sky-50 text-sky-600 ring-sky-100',
  amber: 'bg-amber-50 text-amber-600 ring-amber-100',
  brand: 'bg-brand-50 text-brand-600 ring-brand-100',
  slate: 'bg-slate-50 text-slate-400 ring-slate-200',
}

function KpiCard({ label, icon: Icon, tone, section, to, context }) {
  const available = section?.status === 'available'
  const body = (
    <>
      <div className="flex items-center justify-between gap-3">
        <span className={cn('flex size-9 items-center justify-center rounded-lg ring-1', available ? tones[tone] : tones.slate)}>
          <Icon className="size-[18px]" aria-hidden="true" />
        </span>
        {to && available && (
          <ArrowUpRight className="size-4 text-slate-300 transition-colors group-hover:text-slate-500" aria-hidden="true" />
        )}
      </div>
      <p className="mt-4 text-[13px] font-medium text-slate-500">{label}</p>
      {available ? (
        <>
          <p className="mt-1 text-[28px] leading-8 font-semibold tracking-tight text-slate-900 tabular-nums">
            {formatNumber(section.data.total)}
          </p>
          <p className="mt-2 truncate text-xs text-slate-500">{context?.(section.data)}</p>
        </>
      ) : (
        <>
          <p className="mt-1 text-[28px] leading-8 font-semibold tracking-tight text-slate-300" aria-label="Not available">
            —
          </p>
          <p className="mt-2 truncate text-xs text-slate-400" title={`Requires ${section?.module?.name ?? 'a future module'}`}>
            Not set up yet
          </p>
        </>
      )}
    </>
  )

  const cardClass = 'group h-full p-5 transition duration-200'
  if (to && available) {
    return (
      <Link to={to} className="block rounded-xl focus-visible:ring-2 focus-visible:ring-brand-500/40">
        <Card className={cn(cardClass, 'hover:-translate-y-0.5 hover:border-slate-300 hover:shadow-md')}>{body}</Card>
      </Link>
    )
  }
  return <Card className={cn(cardClass, !available && 'bg-slate-50/40')}>{body}</Card>
}

function roleContext(d) {
  if (d.total === 0) return 'No accounts yet'
  const parts = [`${formatNumber(d.active)} active`]
  if (d.new_this_month > 0) parts.push(`+${formatNumber(d.new_this_month)} this month`)
  else if (d.inactive > 0) parts.push(`${formatNumber(d.inactive)} inactive`)
  return parts.join(' · ')
}

function studentContext(d) {
  if (d.total === 0) return 'No students yet'
  const parts = [`${formatNumber(d.active)} active`]
  if (d.new_this_month > 0) parts.push(`+${formatNumber(d.new_this_month)} this month`)
  else if (d.inactive > 0) parts.push(`${formatNumber(d.inactive)} inactive`)
  return parts.join(' · ')
}

function teacherContext(d) {
  if (d.total === 0) return 'No teachers yet'
  const parts = [`${formatNumber(d.active)} active`]
  if (d.new_this_month > 0) parts.push(`+${formatNumber(d.new_this_month)} joined`)
  else if (d.inactive > 0) parts.push(`${formatNumber(d.inactive)} inactive`)
  return parts.join(' · ')
}

export function KpiCards({ overview }) {
  return (
    <section aria-label="Key figures" className="grid grid-cols-2 gap-3 sm:gap-4 md:grid-cols-3 xl:grid-cols-5">
      <KpiCard label="Total students" icon={GraduationCap} tone="emerald" section={overview.students} to="/students" context={studentContext} />
      <KpiCard label="Total teachers" icon={Presentation} tone="sky" section={overview.teachers} to="/teachers" context={teacherContext} />
      <KpiCard label="Total guardians" icon={HeartHandshake} tone="amber" section={overview.guardians} to="/users?role=guardian" context={roleContext} />
      <KpiCard label="Total classes" icon={LayoutGrid} tone="brand" section={overview.classes} context={(d) => `${formatNumber(d.total)} classes`} />
      <KpiCard label="Total sections" icon={Rows3} tone="brand" section={overview.sections} context={(d) => `${formatNumber(d.total)} sections`} />
    </section>
  )
}
