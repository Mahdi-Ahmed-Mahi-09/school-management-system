import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { AlertTriangle, Clock, FileText, Layers, RotateCcw, Save } from 'lucide-react'
import { getErrorCode, getErrorMessage, getFieldErrors } from '../../api/client'
import { examinationsApi } from '../../api/services'
import { ExamStatusBadge } from '../../components/examinations/ExamStatusBadge'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Alert, Badge, Card, EmptyState, ErrorState, PageHeader, Skeleton } from '../../components/ui/Display'
import { TextArea } from '../../components/ui/Sections'
import { useToast } from '../../components/ui/Toast'
import { durationLabel, examActions, weekday } from '../../lib/examinations'
import { formatDateOnly } from '../../lib/students'
import { cn } from '../../lib/utils'

/** One editable row per subject the class takes, prefilled from its paper. */
function rowsFor(ec) {
  const bySubject = new Map(ec.schedules.map((s) => [s.subject.id, s]))
  return ec.subjects.map(({ subject, is_compulsory: compulsory }) => {
    const s = bySubject.get(subject.id)
    return {
      subject,
      compulsory,
      include: Boolean(s),
      id: s?.id ?? '',
      version: s?.version ?? 0,
      exam_date: s?.exam_date ?? '',
      start_time: s?.start_time ?? '',
      end_time: s?.end_time ?? '',
      room_number: s?.room_number ?? '',
      instructions: s?.instructions ?? '',
    }
  })
}

const same = (a, b) => JSON.stringify(a) === JSON.stringify(b)

/** Client-side checks; the server re-validates everything. */
function localErrors(rows, exam) {
  const errs = {}
  const included = rows.filter((r) => r.include)
  for (const r of included) {
    const e = {}
    if (!r.exam_date) e.exam_date = 'Choose a date.'
    else if (r.exam_date < exam.start_date || r.exam_date > exam.end_date) e.exam_date = 'Outside the examination dates.'
    if (!r.start_time) e.start_time = 'Required.'
    if (!r.end_time) e.end_time = 'Required.'
    if (r.start_time && r.end_time && r.start_time >= r.end_time) e.end_time = 'Must be after the start.'
    if (Object.keys(e).length) errs[r.subject.id] = e
  }
  for (let i = 0; i < included.length; i++) {
    for (let j = i + 1; j < included.length; j++) {
      const a = included[i]
      const b = included[j]
      if (a.exam_date && a.exam_date === b.exam_date && a.start_time && b.start_time && a.start_time < b.end_time && b.start_time < a.end_time) {
        errs[a.subject.id] = { ...errs[a.subject.id], exam_date: `Overlaps ${b.subject.name}.` }
        errs[b.subject.id] = { ...errs[b.subject.id], exam_date: `Overlaps ${a.subject.name}.` }
      }
    }
  }
  return errs
}

const cellInput =
  'h-9 w-full rounded-lg bg-white px-2.5 text-sm ring-1 ring-inset focus:ring-2 focus:ring-brand-500 focus:outline-none disabled:bg-slate-50 disabled:text-slate-400'

export default function ScheduleBuilderPage() {
  const { id } = useParams()
  const toast = useToast()
  const [params, setParams] = useSearchParams()
  const [state, setState] = useState({ loading: true, error: null, exam: null })
  const [rows, setRows] = useState([])
  const [serverErrors, setServerErrors] = useState({})
  const [reason, setReason] = useState('')
  const [reasonError, setReasonError] = useState('')
  const [saving, setSaving] = useState(false)
  const [bulkTime, setBulkTime] = useState({ start: '10:00', end: '13:00' })
  const [reloadKey, setReloadKey] = useState(0)
  const reload = useCallback(() => setReloadKey((k) => k + 1), [])

  useEffect(() => {
    let cancelled = false
    examinationsApi
      .get(id)
      .then((exam) => !cancelled && setState({ loading: false, error: null, exam }))
      .catch((err) => !cancelled && setState({ loading: false, error: getErrorMessage(err), exam: null }))
    return () => {
      cancelled = true
    }
  }, [id, reloadKey])

  const exam = state.exam
  const classId = params.get('class') || exam?.classes[0]?.class.id || ''
  const ec = exam?.classes.find((c) => c.class.id === classId)
  const loaded = useMemo(() => (ec ? rowsFor(ec) : []), [ec])
  useEffect(() => {
    setRows(loaded)
    setServerErrors({})
  }, [loaded])

  const dirty = !same(rows, loaded)
  useEffect(() => {
    if (!dirty) return
    const h = (e) => {
      e.preventDefault()
      e.returnValue = ''
    }
    window.addEventListener('beforeunload', h)
    return () => window.removeEventListener('beforeunload', h)
  }, [dirty])

  if (state.error) {
    return (
      <Card>
        <ErrorState title="Couldn't load the examination" message={state.error} onRetry={reload} />
      </Card>
    )
  }
  if (state.loading || !exam) return <Skeleton className="h-96" />

  const a = examActions(exam)
  const published = exam.status === 'published'
  const crumbs = [{ label: 'Examinations', to: '/examinations' }, { label: exam.name, to: `/examinations/${exam.id}` }, { label: 'Schedule' }]
  const errors = localErrors(rows, exam)
  const included = rows.filter((r) => r.include)
  const update = (subjectId, patch) => {
    setRows((rs) => rs.map((r) => (r.subject.id === subjectId ? { ...r, ...patch } : r)))
    setServerErrors((e) => ({ ...e, [subjectId]: undefined }))
  }
  const pickClass = (cid) => {
    if (dirty && !window.confirm('Discard the unsaved changes to this class?')) return
    setParams({ class: cid }, { replace: true })
  }

  const save = async () => {
    if (published && !reason.trim()) {
      setReasonError('This timetable is published. Give a reason for the change.')
      return
    }
    const items = included.map((r) => ({
      id: r.id || undefined,
      version: r.version,
      subject_id: r.subject.id,
      exam_date: r.exam_date,
      start_time: r.start_time,
      end_time: r.end_time,
      room_number: r.room_number.trim(),
      instructions: r.instructions.trim(),
    }))
    setSaving(true)
    try {
      await examinationsApi.saveClassSchedule(exam.id, classId, { items, reason: reason.trim() })
      toast.success('Schedule saved', `${ec.class.name}: ${items.length} paper${items.length === 1 ? '' : 's'}.`)
      setReason('')
      reload()
    } catch (err) {
      // Map "items[i].field" back to the subject's row.
      const details = getFieldErrors(err)
      const byRow = {}
      for (const [key, msg] of Object.entries(details)) {
        const m = /^items\[(\d+)\]\.(\w+)$/.exec(key)
        if (m && included[Number(m[1])]) {
          const sid = included[Number(m[1])].subject.id
          byRow[sid] = { ...byRow[sid], [m[2]]: msg }
        }
      }
      setServerErrors(byRow)
      if (details.reason) setReasonError(details.reason)
      toast.error(getErrorCode(err) === 'SCHEDULE_CONFLICT' ? 'Schedule conflict' : "Couldn't save the schedule", getErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  const hasLocalErrors = Object.keys(errors).length > 0

  return (
    <div className="space-y-6 pb-24 animate-fade-in">
      <div className="space-y-3">
        <Breadcrumbs items={crumbs} />
        <PageHeader
          eyebrow={<ExamStatusBadge status={exam.status} />}
          title={`Schedule · ${exam.name}`}
          description={`${exam.academic_year.name} · ${formatDateOnly(exam.start_date)} – ${formatDateOnly(exam.end_date)}. Tick the subjects included and set each paper's date and time.`}
          actions={
            <Link to={`/examinations/${exam.id}/timetable`} className={buttonClasses('secondary')}>
              <FileText className="size-4" aria-hidden="true" />
              View timetable
            </Link>
          }
        />
      </div>

      {!a.schedule && (
        <Alert tone="info" title={`${exam.name} is ${exam.status}`}>
          Its timetable can no longer be changed.
        </Alert>
      )}
      {published && (
        <Alert tone="warning" icon={AlertTriangle} title="Changing a published timetable">
          You can move papers or add subjects; removing papers is no longer possible. Every change is recorded with your reason.
        </Alert>
      )}

      {exam.classes.length === 0 ? (
        <Card>
          <EmptyState
            icon={Layers}
            title="No classes yet"
            description="Add classes on the examination page first."
            action={
              <Link to={`/examinations/${exam.id}`} className={buttonClasses('primary', 'sm')}>
                Go to the examination
              </Link>
            }
          />
        </Card>
      ) : (
        <>
          <div role="tablist" aria-label="Classes" className="flex gap-1 overflow-x-auto rounded-xl bg-slate-100 p-1">
            {exam.classes.map((c) => (
              <button
                key={c.class.id}
                type="button"
                role="tab"
                aria-selected={c.class.id === classId}
                onClick={() => pickClass(c.class.id)}
                className={cn(
                  'flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm font-medium whitespace-nowrap',
                  c.class.id === classId ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-600 hover:text-slate-900',
                )}
              >
                {c.class.name}
                <span className={cn('rounded-full px-1.5 text-xs tabular-nums', c.schedules.length ? 'bg-emerald-50 text-emerald-700' : 'bg-amber-50 text-amber-700')}>
                  {c.schedules.length}/{c.subjects.length}
                </span>
              </button>
            ))}
          </div>

          {ec && (
            <Card className="overflow-hidden">
              <div className="flex flex-col gap-3 border-b border-slate-100 p-4 sm:px-6 lg:flex-row lg:items-end">
                <div className="min-w-0 flex-1">
                  <h2 className="text-[15px] font-semibold text-slate-900">{ec.class.name}</h2>
                  <p className="text-[13px] text-slate-500">
                    {ec.sections} section{ec.sections === 1 ? '' : 's'} · {ec.students} enrolled · {included.length} of {ec.subjects.length} subjects included
                  </p>
                </div>
                {a.schedule && ec.subjects.length > 0 && (
                  <div className="flex flex-wrap items-end gap-2">
                    <label className="text-xs text-slate-500">
                      Start
                      <input type="time" value={bulkTime.start} onChange={(e) => setBulkTime((b) => ({ ...b, start: e.target.value }))} className={cn(cellInput, 'mt-1 w-28 ring-slate-200')} aria-label="Start time for all" />
                    </label>
                    <label className="text-xs text-slate-500">
                      End
                      <input type="time" value={bulkTime.end} onChange={(e) => setBulkTime((b) => ({ ...b, end: e.target.value }))} className={cn(cellInput, 'mt-1 w-28 ring-slate-200')} aria-label="End time for all" />
                    </label>
                    <Button
                      variant="secondary"
                      size="sm"
                      icon={Clock}
                      onClick={() => setRows((rs) => rs.map((r) => (r.include ? { ...r, start_time: bulkTime.start, end_time: bulkTime.end } : r)))}
                      disabled={!included.length}
                    >
                      Apply to included
                    </Button>
                    <Button variant="ghost" size="sm" icon={RotateCcw} onClick={() => setRows(loaded)} disabled={!dirty}>
                      Reset
                    </Button>
                  </div>
                )}
              </div>
              {ec.subjects.length === 0 ? (
                <EmptyState
                  icon={Layers}
                  title={`${ec.class.name} has no subjects in ${exam.academic_year.name}`}
                  description="Assign subjects to the class under Class subjects first."
                  action={
                    <Link to="/class-subjects" className={buttonClasses('secondary', 'sm')}>
                      Class subjects
                    </Link>
                  }
                />
              ) : (
                <div className="overflow-x-auto">
                  <table className="min-w-[56rem] text-sm">
                    <thead className="bg-slate-50/70 text-xs font-medium text-slate-500">
                      <tr className="border-b border-slate-100">
                        <th className="w-10 px-4 py-2.5 sm:pl-6">
                          <span className="sr-only">Include</span>
                        </th>
                        <th className="px-3 py-2.5 text-left">Subject</th>
                        <th className="px-3 py-2.5 text-left">Date</th>
                        <th className="px-3 py-2.5 text-left">Start</th>
                        <th className="px-3 py-2.5 text-left">End</th>
                        <th className="px-3 py-2.5 text-left">Room</th>
                        <th className="px-3 py-2.5 text-left sm:pr-6">Instructions</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100">
                      {rows.map((r) => {
                        const e = { ...errors[r.subject.id], ...serverErrors[r.subject.id] }
                        const disabled = !a.schedule || !r.include
                        const lockInclude = !a.schedule || (published && Boolean(r.id)) || (r.subject.status !== 'active' && !r.id)
                        const ring = (k) => (e[k] ? 'ring-rose-300' : 'ring-slate-200')
                        return (
                          <tr key={r.subject.id} className={cn(!r.include && 'bg-slate-50/40')} data-testid="schedule-row">
                            <td className="px-4 py-2 align-top sm:pl-6">
                              <input
                                type="checkbox"
                                checked={r.include}
                                disabled={lockInclude}
                                onChange={() => update(r.subject.id, { include: !r.include, ...(r.include ? {} : { start_time: r.start_time || bulkTime.start, end_time: r.end_time || bulkTime.end }) })}
                                aria-label={`Include ${r.subject.name}`}
                                className="mt-2.5 size-4 rounded border-slate-300 text-brand-600"
                              />
                            </td>
                            <td className="px-3 py-2 align-top">
                              <span className="block pt-1.5 font-medium text-slate-900">{r.subject.name}</span>
                              <span className="flex flex-wrap items-center gap-1.5 text-xs text-slate-500">
                                <span className="font-mono">{r.subject.code}</span>
                                {r.compulsory ? <Badge className="bg-slate-50 text-slate-600 ring-slate-500/15">Compulsory</Badge> : <Badge className="bg-slate-50 text-slate-500 ring-slate-400/15">Optional</Badge>}
                                {r.subject.status !== 'active' && <Badge className="bg-amber-50 text-amber-800 ring-amber-600/20">Inactive</Badge>}
                              </span>
                            </td>
                            <td className="px-3 py-2 align-top">
                              <input
                                type="date"
                                value={r.exam_date}
                                min={exam.start_date}
                                max={exam.end_date}
                                disabled={disabled}
                                onChange={(ev) => update(r.subject.id, { exam_date: ev.target.value })}
                                aria-label={`Date for ${r.subject.name}`}
                                aria-invalid={!!e.exam_date || undefined}
                                className={cn(cellInput, ring('exam_date'), 'w-40')}
                              />
                              {r.include && r.exam_date && !e.exam_date && <span className="mt-0.5 block text-xs text-slate-400">{weekday(r.exam_date)}</span>}
                              {r.include && e.exam_date && <span className="mt-0.5 block max-w-60 text-xs text-rose-600">{e.exam_date}</span>}
                              {r.include && e.subject_id && <span className="mt-0.5 block max-w-60 text-xs text-rose-600">{e.subject_id}</span>}
                            </td>
                            <td className="px-3 py-2 align-top">
                              <input type="time" value={r.start_time} disabled={disabled} onChange={(ev) => update(r.subject.id, { start_time: ev.target.value })} aria-label={`Start time for ${r.subject.name}`} className={cn(cellInput, ring('start_time'), 'w-28')} />
                              {r.include && e.start_time && <span className="mt-0.5 block text-xs text-rose-600">{e.start_time}</span>}
                            </td>
                            <td className="px-3 py-2 align-top">
                              <input type="time" value={r.end_time} disabled={disabled} onChange={(ev) => update(r.subject.id, { end_time: ev.target.value })} aria-label={`End time for ${r.subject.name}`} className={cn(cellInput, ring('end_time'), 'w-28')} />
                              {r.include && (e.end_time ? <span className="mt-0.5 block text-xs text-rose-600">{e.end_time}</span> : <span className="mt-0.5 block text-xs text-slate-400">{durationLabel(r.start_time, r.end_time)}</span>)}
                            </td>
                            <td className="px-3 py-2 align-top">
                              <input type="text" value={r.room_number} maxLength={40} disabled={disabled} placeholder="Optional" onChange={(ev) => update(r.subject.id, { room_number: ev.target.value })} aria-label={`Room for ${r.subject.name}`} className={cn(cellInput, ring('room_number'), 'w-28')} />
                            </td>
                            <td className="px-3 py-2 align-top sm:pr-6">
                              <input type="text" value={r.instructions} maxLength={500} disabled={disabled} placeholder="Optional" onChange={(ev) => update(r.subject.id, { instructions: ev.target.value })} aria-label={`Instructions for ${r.subject.name}`} className={cn(cellInput, 'w-56 ring-slate-200')} />
                            </td>
                          </tr>
                        )
                      })}
                    </tbody>
                  </table>
                </div>
              )}
              {published && a.schedule && (
                <div className="border-t border-slate-100 p-4 sm:px-6">
                  <TextArea label="Reason for the change" required rows={2} value={reason} onChange={(e) => { setReason(e.target.value); setReasonError('') }} error={reasonError} />
                </div>
              )}
            </Card>
          )}

          {a.schedule && ec && ec.subjects.length > 0 && (
            <div className="fixed inset-x-0 bottom-0 z-20 border-t border-slate-200 bg-white/95 backdrop-blur lg:pl-64 print:hidden">
              <div className="mx-auto flex max-w-7xl flex-col gap-2 px-4 py-3 sm:flex-row sm:items-center sm:px-6 lg:px-10">
                <p className="flex-1 text-sm text-slate-600" aria-live="polite">
                  {hasLocalErrors ? (
                    <span className="font-medium text-rose-700">Fix the highlighted papers before saving.</span>
                  ) : dirty ? (
                    `${ec.class.name}: ${included.length} paper${included.length === 1 ? '' : 's'} — not saved yet`
                  ) : (
                    'All changes saved.'
                  )}
                </p>
                <Button icon={Save} onClick={save} loading={saving} disabled={!dirty || hasLocalErrors}>
                  Save {ec.class.name} schedule
                </Button>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  )
}
