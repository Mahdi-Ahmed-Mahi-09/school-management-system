import { useCallback, useEffect, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { CalendarDays, ClipboardPen, Download, LayoutList, Printer, Table2 } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { examinationsApi } from '../../api/services'
import { ExamStatusBadge } from '../../components/examinations/ExamStatusBadge'
import { TimetableAgenda, TimetableTable } from '../../components/examinations/TimetableView'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Alert, Card, EmptyState, ErrorState, PageHeader, Skeleton } from '../../components/ui/Display'
import { Select } from '../../components/ui/Field'
import { useAuth } from '../../hooks/useAuth'
import { datesBetween, downloadTimetableCsv, examActions, weekday } from '../../lib/examinations'
import { PERMISSIONS } from '../../lib/roles'
import { formatDateOnly } from '../../lib/students'
import { cn } from '../../lib/utils'

/**
 * Printable timetable of one examination. Teachers see only the classes they
 * are assigned to (the API enforces it) and their own papers are highlighted.
 */
export default function ExamTimetablePage() {
  const { id } = useParams()
  const { can } = useAuth()
  const [params, setParams] = useSearchParams()
  const classId = params.get('class') ?? ''
  const date = params.get('date') ?? ''
  const view = params.get('view') === 'days' ? 'days' : 'table'
  const [state, setState] = useState({ loading: true, error: null, data: null })
  const [reloadKey, setReloadKey] = useState(0)
  const reload = useCallback(() => setReloadKey((k) => k + 1), [])

  useEffect(() => {
    let cancelled = false
    setState((s) => ({ ...s, loading: true, error: null }))
    examinationsApi
      .timetable(id)
      .then((data) => !cancelled && setState({ loading: false, error: null, data }))
      .catch((err) => !cancelled && setState({ loading: false, error: getErrorMessage(err), data: null }))
    return () => {
      cancelled = true
    }
  }, [id, reloadKey])

  const set = (k, v) => {
    const next = new URLSearchParams(params)
    if (v) next.set(k, v)
    else next.delete(k)
    setParams(next, { replace: true })
  }

  const admin = can(PERMISSIONS.EXAMS_READ)
  const backTo = admin ? '/examinations' : '/exam-timetables'
  if (state.error) {
    return (
      <div className="space-y-6">
        <Breadcrumbs items={[{ label: admin ? 'Examinations' : 'Exam timetables', to: backTo }, { label: 'Timetable' }]} />
        <Card>
          <ErrorState title="Couldn't load the timetable" message={state.error} onRetry={reload} />
        </Card>
      </div>
    )
  }
  if (state.loading || !state.data) return <Skeleton className="h-96" />

  const { examination: x } = state.data
  const classes = state.data.classes
    .filter((c) => !classId || c.class.id === classId)
    .map((c) => ({ ...c, schedules: date ? c.schedules.filter((s) => s.exam_date === date) : c.schedules }))
  const examDates = datesBetween(x.start_date, x.end_date)
  const papers = classes.reduce((n, c) => n + c.schedules.length, 0)

  return (
    <div className="space-y-6 animate-fade-in print:space-y-4">
      <div className="space-y-3 print:hidden">
        <Breadcrumbs
          items={[
            { label: admin ? 'Examinations' : 'Exam timetables', to: backTo },
            ...(admin ? [{ label: x.name, to: `/examinations/${x.id}` }] : []),
            { label: 'Timetable' },
          ]}
        />
        <PageHeader
          eyebrow={<ExamStatusBadge status={x.status} />}
          title={`${x.name} timetable`}
          description={`${x.exam_type.name} · ${x.academic_year.name} · ${formatDateOnly(x.start_date)} – ${formatDateOnly(x.end_date)}`}
          actions={
            <>
              {admin && can(PERMISSIONS.EXAMS_UPDATE) && examActions(x).schedule && (
                <Link to={`/examinations/${x.id}/schedule${classId ? `?class=${classId}` : ''}`} className={buttonClasses('secondary')}>
                  <ClipboardPen className="size-4" aria-hidden="true" />
                  Edit schedule
                </Link>
              )}
              <Button variant="secondary" icon={Download} onClick={() => downloadTimetableCsv(x, classes)} disabled={!papers}>
                CSV
              </Button>
              <Button icon={Printer} onClick={() => window.print()} disabled={!papers}>
                Print
              </Button>
            </>
          }
        />
      </div>

      {x.status === 'draft' && (
        <Alert tone="warning" title="Draft timetable" className="print:hidden">
          This timetable isn&apos;t published yet. Teachers can&apos;t see it.
        </Alert>
      )}
      {x.status === 'cancelled' && (
        <Alert tone="danger" title="Cancelled" className="print:hidden">
          {x.cancel_reason}
        </Alert>
      )}

      <Card className="p-4 sm:p-5 print:hidden">
        <div className="grid gap-3 sm:grid-cols-3">
          <Select label="Class" value={classId} onChange={(e) => set('class', e.target.value)}>
            <option value="">All classes</option>
            {state.data.classes.map((c) => (
              <option key={c.class.id} value={c.class.id}>
                {c.class.name}
              </option>
            ))}
          </Select>
          <Select label="Date" value={date} onChange={(e) => set('date', e.target.value)}>
            <option value="">All dates</option>
            {examDates.map((d) => (
              <option key={d} value={d}>
                {formatDateOnly(d)} ({weekday(d)})
              </option>
            ))}
          </Select>
          <div>
            <p className="mb-1.5 text-sm font-medium text-slate-700">View</p>
            <div role="tablist" aria-label="View" className="inline-flex rounded-lg bg-slate-100 p-0.5">
              {[
                ['table', 'Table', Table2],
                ['days', 'By day', CalendarDays],
              ].map(([k, label, Icon]) => (
                <button
                  key={k}
                  type="button"
                  role="tab"
                  aria-selected={view === k}
                  onClick={() => set('view', k === 'table' ? '' : k)}
                  className={cn('inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium', view === k ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-600')}
                >
                  <Icon className="size-4" aria-hidden="true" />
                  {label}
                </button>
              ))}
            </div>
          </div>
        </div>
      </Card>

      {/* Printed heading: school and examination details. */}
      <div className="hidden print:block">
        <p className="text-xs tracking-wide text-slate-500 uppercase">Classora · Examination timetable</p>
        <h1 className="text-xl font-semibold text-slate-900">{x.name}</h1>
        <p className="text-sm text-slate-700">
          {x.exam_type.name} · Academic year {x.academic_year.name} · {formatDateOnly(x.start_date)} – {formatDateOnly(x.end_date)}
          {date && ` · ${formatDateOnly(date)}`}
        </p>
      </div>

      {x.instructions && (
        <Card className="p-4 sm:p-5 print:border-slate-300 print:shadow-none">
          <p className="mb-1 flex items-center gap-1.5 text-sm font-semibold text-slate-900">
            <LayoutList className="size-4 text-slate-400 print:hidden" aria-hidden="true" />
            Instructions
          </p>
          <p className="text-sm whitespace-pre-line text-slate-700">{x.instructions}</p>
        </Card>
      )}

      {classes.length === 0 ? (
        <Card>
          <EmptyState icon={CalendarDays} title="No classes" description="This examination has no classes yet." />
        </Card>
      ) : view === 'days' ? (
        <Card className="print:border-0 print:shadow-none">
          <TimetableAgenda classes={classes} />
        </Card>
      ) : (
        classes.map((c) => (
          <Card key={c.id} className="overflow-hidden print:break-inside-avoid print:border-slate-300 print:shadow-none">
            <div className="flex items-center justify-between gap-3 border-b border-slate-100 px-5 py-3 sm:px-6 print:border-slate-300">
              <h2 className="text-[15px] font-semibold text-slate-900">{c.class.name}</h2>
              <span className="text-xs text-slate-500">
                {c.schedules.length} paper{c.schedules.length === 1 ? '' : 's'} · {c.sections} section{c.sections === 1 ? '' : 's'}
              </span>
            </div>
            <TimetableTable schedules={c.schedules} />
          </Card>
        ))
      )}
    </div>
  )
}
