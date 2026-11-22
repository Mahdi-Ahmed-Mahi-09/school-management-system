import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  AlertTriangle,
  CalendarCheck2,
  CalendarRange,
  CalendarX2,
  CircleCheckBig,
  History,
  Info,
  PencilLine,
  Settings2,
  Trash2,
} from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { academicYearsApi } from '../../api/services'
import { AcademicYearFormModal } from '../../components/academic-years/AcademicYearFormModal'
import { CurrentYearPill } from '../../components/academic-years/CurrentYearPill'
import { YearStatusBadge } from '../../components/academic-years/YearStatusBadge'
import { useYearActions } from '../../components/academic-years/useYearActions'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Alert, Card, EmptyState, ErrorState, Skeleton } from '../../components/ui/Display'
import { DetailItem, DetailSection } from '../../components/ui/Sections'
import { useAuth } from '../../hooks/useAuth'
import { attentionNote, durationLabel, formatYearRange, progressPercent, YEAR_STATUS_META } from '../../lib/academicYears'
import { PERMISSIONS } from '../../lib/roles'
import { formatDateOnly } from '../../lib/students'
import { cn, formatDateTime, formatNumber } from '../../lib/utils'

function DetailSkeleton() {
  return (
    <div className="space-y-6">
      <Skeleton className="h-4 w-48" />
      <Skeleton className="h-28 rounded-xl" />
      <div className="grid gap-6 lg:grid-cols-3">
        <Skeleton className="h-64 rounded-xl lg:col-span-2" />
        <Skeleton className="h-64 rounded-xl" />
      </div>
    </div>
  )
}

function TimelineCard({ year }) {
  const percent = progressPercent(year)
  const remaining = year.duration_days - year.days_elapsed
  let summary
  if (year.timeline === 'not_started') summary = `Starts on ${formatDateOnly(year.start_date)}.`
  else if (year.timeline === 'ended') summary = `Ended on ${formatDateOnly(year.end_date)}.`
  else summary = `Day ${formatNumber(year.days_elapsed)} of ${formatNumber(year.duration_days)} · ${formatNumber(remaining)} day${remaining === 1 ? '' : 's'} left`

  return (
    <Card className="p-5 sm:p-6">
      <div className="flex items-baseline justify-between gap-3">
        <h2 className="text-sm font-semibold text-slate-900">Timeline</h2>
        <span className="text-sm font-semibold text-slate-900 tabular-nums">{percent}%</span>
      </div>
      <div
        className="mt-3 h-2 overflow-hidden rounded-full bg-slate-100"
        role="progressbar"
        aria-valuenow={percent}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-label="Academic year progress"
      >
        <div className={cn('h-full rounded-full', year.status === 'active' ? 'bg-brand-500' : 'bg-slate-400')} style={{ width: `${percent}%` }} />
      </div>
      <div className="mt-2 flex justify-between text-xs text-slate-500">
        <span>{formatDateOnly(year.start_date)}</span>
        <span>{formatDateOnly(year.end_date)}</span>
      </div>
      <p className="mt-3 text-[13px] text-slate-600">{summary}</p>
    </Card>
  )
}

/** One management action with its availability and, if blocked, the reason. */
function ActionRow({ icon: Icon, title, description, blocked, button }) {
  return (
    <li className="flex items-start gap-3 py-4 first:pt-0 last:pb-0">
      <span className={cn('mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-lg', blocked ? 'bg-slate-50 text-slate-400' : 'bg-brand-50 text-brand-600')}>
        <Icon className="size-4" aria-hidden="true" />
      </span>
      <div className="min-w-0 flex-1">
        <p className="text-sm font-medium text-slate-900">{title}</p>
        <p className="mt-0.5 text-[13px] text-slate-500">{blocked ? blocked.message : description}</p>
        {!blocked && button && <div className="mt-2.5">{button}</div>}
      </div>
    </li>
  )
}

export default function AcademicYearDetailPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { can } = useAuth()
  const [state, setState] = useState({ loading: true, error: null, notFound: false, year: null })
  const [current, setCurrent] = useState(null)
  const [editing, setEditing] = useState(false)

  const load = useCallback(async () => {
    setState((s) => ({ ...s, loading: true, error: null }))
    try {
      const [year, stats] = await Promise.all([academicYearsApi.get(id), academicYearsApi.stats()])
      setCurrent(stats.current)
      setState({ loading: false, error: null, notFound: false, year })
    } catch (err) {
      setState({ loading: false, error: getErrorMessage(err), notFound: err?.response?.status === 404, year: null })
    }
  }, [id])

  useEffect(() => {
    load()
  }, [load])

  const actions = useYearActions({
    current,
    onChanged: (kind) => (kind === 'delete' ? navigate('/academic-years', { replace: true }) : load()),
  })

  if (state.loading && !state.year) return <DetailSkeleton />

  if (state.notFound || state.error) {
    return (
      <div className="space-y-6">
        <Breadcrumbs items={[{ label: 'Academic years', to: '/academic-years' }, { label: state.notFound ? 'Not found' : 'Error' }]} />
        <Card>
          {state.notFound ? (
            <EmptyState
              icon={CalendarX2}
              title="Academic year not found"
              description="It doesn't exist or may have been deleted."
              action={
                <Link to="/academic-years" className={buttonClasses('secondary', 'sm')}>
                  Back to academic years
                </Link>
              }
            />
          ) : (
            <ErrorState title="Couldn't load this academic year" message={state.error} onRetry={load} />
          )}
        </Card>
      </div>
    )
  }

  const y = state.year
  const note = attentionNote(y)
  const canManage = can(PERMISSIONS.ACADEMIC_YEARS_MANAGE_STATUS)
  const canEdit = can(PERMISSIONS.ACADEMIC_YEARS_UPDATE)
  const canDelete = can(PERMISSIONS.ACADEMIC_YEARS_DELETE)

  return (
    <div className="space-y-6 animate-fade-in">
      <Breadcrumbs items={[{ label: 'Academic years', to: '/academic-years' }, { label: y.name }]} />

      <Card className="p-5 sm:p-6">
        <div className="flex flex-col gap-5 sm:flex-row sm:items-center">
          <span className="flex size-14 shrink-0 items-center justify-center rounded-2xl bg-brand-50 text-brand-600 ring-1 ring-brand-100">
            <CalendarRange className="size-6" aria-hidden="true" />
          </span>
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-2xl font-semibold tracking-tight text-slate-900">{y.name}</h1>
              <YearStatusBadge status={y.status} />
              {y.is_current && <CurrentYearPill />}
            </div>
            <p className="mt-1 text-[15px] text-slate-600">{formatYearRange(y)}</p>
          </div>
          <div className="flex flex-wrap gap-2">
            {canEdit && (
              <Button variant="secondary" icon={PencilLine} onClick={() => setEditing(true)}>
                Edit
              </Button>
            )}
            {canManage && !y.actions.activate_blocked && (
              <Button icon={CalendarCheck2} onClick={() => actions.activate(y)}>
                Set as current
              </Button>
            )}
          </div>
        </div>
      </Card>

      {note && (
        <Alert tone="warning" icon={AlertTriangle} title="Action needed">
          {note}
        </Alert>
      )}

      <div className="grid gap-6 lg:grid-cols-3">
        <div className="space-y-6 lg:col-span-2">
          <DetailSection title="Overview" icon={Info}>
            <DetailItem label="Academic year name" value={y.name} />
            <DetailItem label="Status" value={<YearStatusBadge status={y.status} />} />
            <DetailItem label="Start date" value={formatDateOnly(y.start_date)} />
            <DetailItem label="End date" value={formatDateOnly(y.end_date)} />
            <DetailItem label="Duration" value={`${formatNumber(y.duration_days)} days (${durationLabel(y.duration_days)})`} />
            <DetailItem label="Current year" value={y.is_current ? 'Yes — this is the current academic year' : 'No'} />
            <DetailItem label="Description" value={y.description} wide />
          </DetailSection>

          <TimelineCard year={y} />

          <DetailSection title="Record history" icon={History}>
            <DetailItem label="Created" value={`${formatDateTime(y.created_at)}${y.created_by?.name ? ` by ${y.created_by.name}` : ''}`} />
            <DetailItem label="Last updated" value={`${formatDateTime(y.updated_at)}${y.updated_by?.name ? ` by ${y.updated_by.name}` : ''}`} />
            <DetailItem label="Last made current" value={y.activated_at ? formatDateTime(y.activated_at) : 'Never'} />
            <DetailItem label="Completed" value={y.completed_at ? formatDateTime(y.completed_at) : 'Not yet'} />
          </DetailSection>
        </div>

        <Card className="h-fit">
          <div className="flex items-center gap-2.5 border-b border-slate-100 px-5 py-3.5 sm:px-6">
            <Settings2 className="size-4 text-slate-400" aria-hidden="true" />
            <h2 className="text-sm font-semibold text-slate-900">Management actions</h2>
          </div>
          <ul className="divide-y divide-slate-100 p-5 sm:p-6">
            {canManage && (
              <ActionRow
                icon={CalendarCheck2}
                title="Set as current year"
                description={YEAR_STATUS_META.active.description + ' The previous current year is updated automatically.'}
                blocked={y.actions.activate_blocked}
                button={
                  <Button size="sm" onClick={() => actions.activate(y)}>
                    Set as current
                  </Button>
                }
              />
            )}
            {canManage && (
              <ActionRow
                icon={CircleCheckBig}
                title="Mark as completed"
                description="Close this year and keep it for your records."
                blocked={y.actions.complete_blocked}
                button={
                  <Button size="sm" variant="secondary" onClick={() => actions.complete(y)}>
                    Mark completed
                  </Button>
                }
              />
            )}
            {canDelete && (
              <ActionRow
                icon={Trash2}
                title="Delete academic year"
                description="Only possible while no other records use this year."
                blocked={y.actions.delete_blocked}
                button={
                  <Button size="sm" variant="secondary" className="text-rose-600 hover:text-rose-700" onClick={() => actions.remove(y)}>
                    Delete
                  </Button>
                }
              />
            )}
          </ul>
        </Card>
      </div>

      <AcademicYearFormModal open={editing} year={y} onClose={() => setEditing(false)} onSaved={load} />
      {actions.dialogs}
    </div>
  )
}
