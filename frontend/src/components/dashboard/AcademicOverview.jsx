import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { BookOpenCheck, CalendarRange, ChartNoAxesColumn, School, UserRoundPlus } from 'lucide-react'
import { Link } from 'react-router-dom'
import { useAuth } from '../../hooks/useAuth'
import { YEAR_STATUS_META } from '../../lib/academicYears'
import { PERMISSIONS } from '../../lib/roles'
import { formatDateOnly } from '../../lib/students'
import { cn, formatNumber } from '../../lib/utils'
import { ChartTooltip } from './ChartParts'
import { ModuleUnavailable, NoData, Panel } from './Panel'
import { BRAND, axisProps, gridProps } from './chartTheme'

function AcademicYearCard({ section }) {
  const { can } = useAuth()
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
  if (!year) {
    // The module exists but no year is current yet: prompt to set one up.
    return (
      <div className="flex h-full flex-col items-start rounded-xl border border-dashed border-slate-200 bg-slate-50/50 p-4">
        <span className="flex size-8 items-center justify-center rounded-lg bg-white text-slate-400 shadow-card ring-1 ring-slate-200/70">
          <CalendarRange className="size-4" aria-hidden="true" />
        </span>
        <p className="mt-3 text-sm font-semibold text-slate-800">No current academic year</p>
        <p className="mt-1 text-[13px] leading-relaxed text-slate-500">Set the year your school is running now to track its progress here.</p>
        {can(PERMISSIONS.ACADEMIC_YEARS_READ) && (
          <Link to="/academic-years" className="mt-3 text-[13px] font-medium text-brand-600 hover:text-brand-700">
            Set up academic years
          </Link>
        )}
      </div>
    )
  }
  const progress = year.days_total ? Math.min(100, Math.max(0, Math.round((year.days_elapsed / year.days_total) * 100))) : 0
  const ended = year.timeline === 'ended'
  return (
    <div className="rounded-xl border border-slate-200/80 p-4">
      <div className="flex items-center justify-between gap-2">
        <p className="text-[13px] text-slate-500">Current academic year</p>
        <span className={cn('rounded-md px-2 py-0.5 text-xs font-medium ring-1 ring-inset', YEAR_STATUS_META[year.status]?.badge ?? YEAR_STATUS_META.completed.badge)}>
          {YEAR_STATUS_META[year.status]?.label ?? year.status}
        </span>
      </div>
      {can(PERMISSIONS.ACADEMIC_YEARS_READ) ? (
        <Link to={`/academic-years/${year.id}`} className="mt-1 block text-lg font-semibold text-slate-900 hover:text-brand-700">
          {year.name}
        </Link>
      ) : (
        <p className="mt-1 text-lg font-semibold text-slate-900">{year.name}</p>
      )}
      <p className="text-xs text-slate-500">
        {formatDateOnly(year.start_date)} – {formatDateOnly(year.end_date)}
      </p>
      <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-slate-100" role="progressbar" aria-valuenow={progress} aria-valuemin={0} aria-valuemax={100} aria-label="Academic year progress">
        <div className="h-full rounded-full bg-brand-500" style={{ width: `${progress}%` }} />
      </div>
      <p className={cn('mt-1.5 text-xs', ended ? 'font-medium text-amber-700' : 'text-slate-500')}>
        {ended ? 'Ended — set the next year as current' : year.timeline === 'not_started' ? 'Not started yet' : `${progress}% complete`}
      </p>
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

export function AcademicOverview({ academic, className }) {
  return (
    <Panel icon={BookOpenCheck} title="Academic overview" description="Academic year, enrollment and students by class." className={className}>
      <div className="grid gap-4 lg:grid-cols-[minmax(0,15rem)_minmax(0,1fr)]">
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-1">
          <AcademicYearCard section={academic.academic_year} />
          <EnrollmentCard section={academic.enrollment} />
        </div>
        <ClassDistribution section={academic.class_distribution} />
      </div>
    </Panel>
  )
}
