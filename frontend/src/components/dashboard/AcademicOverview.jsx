import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { BookOpenCheck, CalendarRange, ChartNoAxesColumn, School, UserRoundPlus } from 'lucide-react'
import { cn, formatDate, formatNumber } from '../../lib/utils'
import { ChartTooltip } from './ChartParts'
import { ModuleUnavailable, NoData, Panel } from './Panel'
import { BRAND, axisProps, gridProps } from './chartTheme'

const YEAR_STATUS = {
  active: 'bg-emerald-50 text-emerald-700 ring-emerald-600/15',
  upcoming: 'bg-sky-50 text-sky-700 ring-sky-600/15',
  completed: 'bg-slate-100 text-slate-600 ring-slate-500/15',
}

function AcademicYearCard({ section, now }) {
  if (section?.status !== 'available') {
    return (
      <ModuleUnavailable
        compact
        module={section?.module}
        icon={CalendarRange}
        title="No academic year configured"
        description="The active academic year and its progress will show here."
      />
    )
  }
  const year = section.data
  const start = new Date(year.start_date)
  const end = new Date(year.end_date)
  const progress = Math.min(100, Math.max(0, Math.round(((now - start) / (end - start)) * 100)))
  return (
    <div className="rounded-xl border border-slate-200/80 p-4">
      <div className="flex items-center justify-between gap-2">
        <p className="text-[13px] text-slate-500">Academic year</p>
        <span className={cn('rounded-md px-2 py-0.5 text-xs font-medium capitalize ring-1 ring-inset', YEAR_STATUS[year.status] ?? YEAR_STATUS.completed)}>
          {year.status}
        </span>
      </div>
      <p className="mt-1 text-lg font-semibold text-slate-900">{year.name}</p>
      <p className="text-xs text-slate-500">
        {formatDate(year.start_date)} – {formatDate(year.end_date)}
      </p>
      <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-slate-100" role="progressbar" aria-valuenow={progress} aria-valuemin={0} aria-valuemax={100} aria-label="Academic year progress">
        <div className="h-full rounded-full bg-brand-500" style={{ width: `${progress}%` }} />
      </div>
      <p className="mt-1.5 text-xs text-slate-500">{progress}% complete</p>
    </div>
  )
}

function EnrollmentCard({ section }) {
  if (section?.status !== 'available') {
    return (
      <ModuleUnavailable
        compact
        module={section?.module}
        icon={UserRoundPlus}
        title="Enrollment not tracked yet"
        description="Enrolled students and new admissions will show here."
      />
    )
  }
  const e = section.data
  return (
    <div className="rounded-xl border border-slate-200/80 p-4">
      <p className="text-[13px] text-slate-500">Enrolled students</p>
      <p className="mt-1 text-2xl font-semibold text-slate-900 tabular-nums">{formatNumber(e.total_enrolled)}</p>
      <p className="mt-1 text-xs text-slate-500">+{formatNumber(e.new_this_month)} admitted this month</p>
    </div>
  )
}

function ClassDistribution({ section }) {
  if (section?.status !== 'available') {
    return (
      <ModuleUnavailable
        module={section?.module}
        icon={ChartNoAxesColumn}
        title="Class distribution unavailable"
        description="A chart of students per class will appear once students are placed into classes and sections."
      />
    )
  }
  const rows = section.data
  if (!rows.length) return <NoData icon={School} title="No classes yet" description="Students per class will be charted here." />
  return (
    <div className="h-64" role="img" aria-label="Bar chart of students per class">
      <ResponsiveContainer width="100%" height="100%">
        <BarChart data={rows} margin={{ top: 4, right: 4, bottom: 0, left: -20 }}>
          <CartesianGrid {...gridProps} />
          <XAxis dataKey="class_name" interval={0} {...axisProps} />
          <YAxis allowDecimals={false} {...axisProps} />
          <Tooltip cursor={{ fill: '#f1f5f9' }} content={<ChartTooltip valueFormatter={formatNumber} />} />
          <Bar dataKey="students" name="Students" fill={BRAND} radius={[4, 4, 0, 0]} maxBarSize={36} />
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}

export function AcademicOverview({ academic, now, className }) {
  return (
    <Panel icon={BookOpenCheck} title="Academic overview" description="Academic year, enrollment and students by class." className={className}>
      <div className="grid gap-4 lg:grid-cols-[minmax(0,15rem)_minmax(0,1fr)]">
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-1">
          <AcademicYearCard section={academic.academic_year} now={now} />
          <EnrollmentCard section={academic.enrollment} />
        </div>
        <ClassDistribution section={academic.class_distribution} />
      </div>
    </Panel>
  )
}
