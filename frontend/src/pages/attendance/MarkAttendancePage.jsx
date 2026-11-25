import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { CalendarDays, CheckCheck, ClipboardCheck, Eye, History, RotateCcw, Save, Search, UserX, Users } from 'lucide-react'
import { getErrorCode, getErrorMessage } from '../../api/client'
import { attendanceApi } from '../../api/services'
import { AttendanceStatusBadge, PercentText } from '../../components/attendance/AttendanceBadges'
import { StatusPicker } from '../../components/attendance/StatusPicker'
import { useAttendanceOptions } from '../../components/attendance/useAttendanceOptions'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Alert, Card, EmptyState, ErrorState, PageHeader, Skeleton } from '../../components/ui/Display'
import { Checkbox, Input, Select } from '../../components/ui/Field'
import { Modal } from '../../components/ui/Modal'
import { PersonAvatar } from '../../components/ui/PersonAvatar'
import { TextArea } from '../../components/ui/Sections'
import { useToast } from '../../components/ui/Toast'
import { ATTENDANCE_STATUS_META, liveSummary, sectionLabel } from '../../lib/attendance'
import { yearOptionLabel } from '../../lib/classes'
import { formatDateOnly } from '../../lib/students'
import { cn, formatDateTime } from '../../lib/utils'

/** Loaded statuses and remarks of a session, keyed by enrollment id. */
function initialDraft(session) {
  const statuses = {}
  const remarks = {}
  for (const s of session.students) {
    if (s.attendance) {
      statuses[s.enrollment_id] = s.attendance.status
      remarks[s.enrollment_id] = s.attendance.remarks ?? ''
    }
  }
  return { statuses, remarks }
}

function SummaryStrip({ summary }) {
  const items = [
    { key: 'total', label: 'Students', value: summary.total, tone: 'text-slate-900' },
    { key: 'present', label: 'Present', value: summary.present, dot: ATTENDANCE_STATUS_META.present.dot },
    { key: 'absent', label: 'Absent', value: summary.absent, dot: ATTENDANCE_STATUS_META.absent.dot },
    { key: 'late', label: 'Late', value: summary.late, dot: ATTENDANCE_STATUS_META.late.dot },
    { key: 'leave', label: 'Leave', value: summary.leave, dot: ATTENDANCE_STATUS_META.leave.dot },
    { key: 'unmarked', label: 'Unmarked', value: summary.unmarked, dot: 'bg-slate-400', warn: summary.unmarked > 0 },
  ]
  return (
    <dl className="grid grid-cols-3 gap-px overflow-hidden rounded-xl border border-slate-200/80 bg-slate-100 sm:grid-cols-7" aria-live="polite">
      {items.map((i) => (
        <div key={i.key} className={cn('bg-white px-4 py-3', i.warn && 'bg-amber-50/60')}>
          <dt className="flex items-center gap-1.5 text-xs text-slate-500">
            {i.dot && <span className={cn('size-2 rounded-full', i.dot)} aria-hidden="true" />}
            {i.label}
          </dt>
          <dd className={cn('mt-0.5 text-xl font-semibold tabular-nums', i.warn ? 'text-amber-800' : 'text-slate-900')}>{i.value}</dd>
        </div>
      ))}
      <div className="col-span-3 bg-white px-4 py-3 sm:col-span-1">
        <dt className="text-xs text-slate-500">Attendance</dt>
        <dd className="mt-0.5 text-xl">
          <PercentText value={summary.percentage} />
        </dd>
      </div>
    </dl>
  )
}

export default function MarkAttendancePage() {
  const toast = useToast()
  const [params, setParams] = useSearchParams()
  const yearParam = params.get('year') ?? ''
  const classId = params.get('class') ?? ''
  const sectionId = params.get('section') ?? ''
  const dateParam = params.get('date') ?? ''
  const options = useAttendanceOptions(yearParam)
  const opts = options.data
  const year = opts?.academic_year
  const date = dateParam || opts?.default_date || ''

  const [session, setSession] = useState({ loading: false, error: null, data: null })
  const [reloadKey, setReloadKey] = useState(0)
  const [draft, setDraft] = useState({ statuses: {}, remarks: {} })
  const [selected, setSelected] = useState(() => new Set())
  const [filter, setFilter] = useState('')
  const [reviewing, setReviewing] = useState(false)
  const [reason, setReason] = useState('')
  const [reasonError, setReasonError] = useState('')
  const [saving, setSaving] = useState(false)
  const [conflict, setConflict] = useState(false)

  // A teacher with a single section gets it chosen for them.
  const onlySection = opts && opts.classes.length === 1 && opts.classes[0].sections.length === 1 ? opts.classes[0] : null
  const effectiveClass = classId || onlySection?.class.id || ''
  const effectiveSection = sectionId || onlySection?.sections[0].id || ''
  const classOption = opts?.classes.find((c) => c.class.id === effectiveClass)

  const data = session.data
  const loaded = useMemo(() => (data ? initialDraft(data) : { statuses: {}, remarks: {} }), [data])
  const markable = useMemo(() => (data ? data.students.filter((s) => s.can_mark) : []), [data])
  const changes = useMemo(() => {
    let created = 0
    let updated = 0
    for (const s of markable) {
      const st = draft.statuses[s.enrollment_id]
      if (!st) continue
      if (!s.attendance) created += 1
      else if (st !== s.attendance.status || (draft.remarks[s.enrollment_id] ?? '').trim() !== (s.attendance.remarks ?? '')) updated += 1
    }
    return { created, updated, dirty: created + updated > 0 }
  }, [markable, draft])
  const summary = data ? liveSummary(data.students, draft.statuses) : null
  const unmarkedMarkable = markable.filter((s) => !draft.statuses[s.enrollment_id]).length

  // Warn before leaving the page with unsaved changes.
  useEffect(() => {
    if (!changes.dirty) return
    const onBeforeUnload = (e) => {
      e.preventDefault()
      e.returnValue = ''
    }
    window.addEventListener('beforeunload', onBeforeUnload)
    return () => window.removeEventListener('beforeunload', onBeforeUnload)
  }, [changes.dirty])

  const confirmDiscard = () => !changes.dirty || window.confirm('You have unsaved attendance changes. Discard them?')

  const update = (changesToParams) => {
    if (!confirmDiscard()) return
    const next = new URLSearchParams(params)
    for (const [k, v] of Object.entries(changesToParams)) {
      if (v) next.set(k, v)
      else next.delete(k)
    }
    setParams(next, { replace: true })
  }

  useEffect(() => {
    if (!effectiveSection || !date || !year) {
      setSession({ loading: false, error: null, data: null })
      return
    }
    let cancelled = false
    setSession((s) => ({ ...s, loading: true, error: null }))
    attendanceApi
      .session({ academic_year_id: year.id, section_id: effectiveSection, date })
      .then((d) => {
        if (cancelled) return
        setSession({ loading: false, error: null, data: d })
        setDraft(initialDraft(d))
        setSelected(new Set())
        setConflict(false)
      })
      .catch((err) => !cancelled && setSession({ loading: false, error: getErrorMessage(err), data: null }))
    return () => {
      cancelled = true
    }
  }, [effectiveSection, date, year, reloadKey])

  const reload = useCallback(() => setReloadKey((k) => k + 1), [])
  const setStatus = (id, status) => setDraft((d) => ({ ...d, statuses: { ...d.statuses, [id]: status } }))
  const setRemark = (id, value) => setDraft((d) => ({ ...d, remarks: { ...d.remarks, [id]: value } }))
  const markAll = (status, ids) =>
    setDraft((d) => {
      const statuses = { ...d.statuses }
      for (const id of ids) statuses[id] = status
      return { ...d, statuses }
    })
  const markAllPresent = () => markAll('present', markable.map((s) => s.enrollment_id))
  const markSelectedAbsent = () => {
    markAll('absent', [...selected])
    setSelected(new Set())
  }
  const toggleSelected = (id) =>
    setSelected((cur) => {
      const next = new Set(cur)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  const allSelected = markable.length > 0 && markable.every((s) => selected.has(s.enrollment_id))

  const q = filter.trim().toLowerCase()
  const visible = data
    ? data.students.filter((s) => !q || s.student.full_name.toLowerCase().includes(q) || String(s.roll_number) === q || s.student.admission_number.toLowerCase().includes(q))
    : []

  const needsReason = data?.requires_reason && changes.updated > 0

  const save = async () => {
    if (needsReason && !reason.trim()) {
      setReasonError('Give a reason for correcting an earlier day.')
      return
    }
    setSaving(true)
    try {
      const records = markable
        .filter((s) => draft.statuses[s.enrollment_id])
        .map((s) => ({
          enrollment_id: s.enrollment_id,
          status: draft.statuses[s.enrollment_id],
          remarks: (draft.remarks[s.enrollment_id] ?? '').trim(),
          version: s.attendance?.version ?? 0,
        }))
      const res = await attendanceApi.saveSession({ academic_year_id: year.id, section_id: effectiveSection, date, records, reason: reason.trim() })
      setSession({ loading: false, error: null, data: res.session })
      setDraft(initialDraft(res.session))
      setSelected(new Set())
      setReviewing(false)
      setReason('')
      const parts = [res.created && `${res.created} new`, res.updated && `${res.updated} changed`].filter(Boolean).join(', ')
      toast.success('Attendance saved', `${sectionLabel(res.session.class, res.session.section)} on ${formatDateOnly(res.session.date)}${parts ? ` (${parts})` : ''}.`)
    } catch (err) {
      setReviewing(false)
      if (getErrorCode(err) === 'ATTENDANCE_CHANGED') setConflict(true)
      toast.error("Attendance wasn't saved", getErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  const header = (
    <div className="space-y-3">
      <Breadcrumbs items={[{ label: 'Attendance', to: '/attendance' }, { label: 'Take attendance' }]} />
      <PageHeader
        title="Take attendance"
        description="Choose a section and a date, mark every student, then review and save."
        actions={
          data?.started && (
            <Link to={`/attendance/session?section=${effectiveSection}&date=${date}&year=${year?.id ?? ''}`} className={buttonClasses('secondary')}>
              <Eye className="size-4" aria-hidden="true" />
              Session details
            </Link>
          )
        }
      />
    </div>
  )

  if (options.error) {
    return (
      <div className="space-y-6">
        {header}
        <Card>
          <ErrorState title="Couldn't load your sections" message={options.error} onRetry={options.reload} />
        </Card>
      </div>
    )
  }

  const noSections = opts && opts.classes.length === 0

  return (
    <div className="space-y-6 pb-24 animate-fade-in">
      {header}

      <Card className="p-4 sm:p-5">
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {options.loading && !opts ? (
            [0, 1, 2, 3].map((i) => <Skeleton key={i} className="h-16" />)
          ) : (
            <>
              <Select label="Academic year" value={year?.id ?? ''} onChange={(e) => update({ year: e.target.value, class: '', section: '', date: '' })}>
                {opts?.academic_years.length === 0 && <option value="">No academic years</option>}
                {opts?.academic_years.map((y) => (
                  <option key={y.id} value={y.id}>
                    {yearOptionLabel(y)}
                  </option>
                ))}
              </Select>
              <Select label="Class" value={effectiveClass} onChange={(e) => update({ class: e.target.value, section: '' })} disabled={noSections}>
                <option value="">Choose a class</option>
                {opts?.classes.map((c) => (
                  <option key={c.class.id} value={c.class.id}>
                    {c.class.name}
                  </option>
                ))}
              </Select>
              <Select label="Section" value={effectiveSection} onChange={(e) => update({ class: effectiveClass, section: e.target.value })} disabled={!effectiveClass}>
                <option value="">{effectiveClass ? 'Choose a section' : 'Choose a class first'}</option>
                {(classOption?.sections ?? []).map((s) => (
                  <option key={s.id} value={s.id}>
                    Section {s.name}
                    {s.status === 'inactive' ? ' (inactive)' : ''}
                  </option>
                ))}
              </Select>
              <Input
                label="Date"
                type="date"
                icon={CalendarDays}
                value={date}
                min={year?.start_date}
                max={year ? (opts.today < year.end_date ? opts.today : year.end_date) : undefined}
                onChange={(e) => e.target.value && update({ date: e.target.value })}
                hint={!opts?.can_correct ? 'Teachers take attendance for today.' : undefined}
              />
            </>
          )}
        </div>
      </Card>

      {noSections ? (
        <Card>
          <EmptyState
            icon={Users}
            title="No sections to take attendance for"
            description={opts.scope.note || (opts.academic_years.length === 0 ? 'Add an academic year and sections first.' : 'This academic year has no sections yet.')}
          />
        </Card>
      ) : !effectiveSection ? (
        <Card>
          <EmptyState icon={ClipboardCheck} title="Choose a section" description="Pick the class and section you are taking attendance for." />
        </Card>
      ) : session.error ? (
        <Card>
          <ErrorState title="Couldn't load the class list" message={session.error} onRetry={reload} />
        </Card>
      ) : session.loading || !data ? (
        <Card className="space-y-3 p-5">
          <Skeleton className="h-16" />
          {[0, 1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-12" />
          ))}
        </Card>
      ) : (
        <>
          {conflict && (
            <Alert
              tone="warning"
              title="Someone else saved this attendance meanwhile"
              action={
                <Button size="sm" variant="secondary" icon={RotateCcw} onClick={reload}>
                  Reload
                </Button>
              }
            >
              Reload to see their changes. Your unsaved changes will be discarded.
            </Alert>
          )}
          {!data.editable && data.lock_reason && (
            <Alert tone="info" title="View only">
              {data.lock_reason}
            </Alert>
          )}
          {data.editable && !data.can_create && data.lock_reason && (
            <Alert tone="info" title="Only corrections are possible">
              {data.lock_reason}
            </Alert>
          )}
          {data.editable && data.requires_reason && (
            <Alert tone="warning" icon={History} title={`Correcting ${formatDateOnly(data.date)}`}>
              This is an earlier day. Changes to saved records need a reason and are kept in each record&apos;s history.
            </Alert>
          )}

          <SummaryStrip summary={summary} />

          <Card className="overflow-hidden">
            <div className="flex flex-col gap-3 border-b border-slate-100 p-4 sm:px-6 lg:flex-row lg:items-center">
              <div className="min-w-0 flex-1">
                <h2 className="text-[15px] font-semibold text-slate-900">
                  {sectionLabel(data.class, data.section)} · {formatDateOnly(data.date)}
                </h2>
                <p className="text-[13px] text-slate-500">
                  {data.started
                    ? `Last saved ${formatDateTime(data.last_saved_at)}${data.last_saved_by ? ` by ${data.last_saved_by.name}` : ''}`
                    : 'Not taken yet. Students without a status are unmarked, not absent.'}
                </p>
              </div>
              {data.editable && markable.length > 0 && (
                <div className="flex flex-wrap gap-2">
                  <Button variant="secondary" size="sm" icon={CheckCheck} onClick={markAllPresent}>
                    Mark all present
                  </Button>
                  <Button variant="secondary" size="sm" icon={UserX} onClick={markSelectedAbsent} disabled={selected.size === 0}>
                    Mark selected absent{selected.size ? ` (${selected.size})` : ''}
                  </Button>
                  <Button variant="ghost" size="sm" icon={RotateCcw} onClick={() => setDraft(loaded)} disabled={!changes.dirty}>
                    Reset
                  </Button>
                </div>
              )}
            </div>

            {data.students.length === 0 ? (
              <EmptyState icon={Users} title="No students in this section" description="Only students enrolled in this section on this date appear here." />
            ) : (
              <>
                <div className="flex items-center gap-3 border-b border-slate-100 px-4 py-2.5 sm:px-6">
                  {data.editable && markable.length > 0 && (
                    <Checkbox
                      label="Select all"
                      checked={allSelected}
                      onChange={() => setSelected(allSelected ? new Set() : new Set(markable.map((s) => s.enrollment_id)))}
                    />
                  )}
                  <div className="relative ml-auto w-full max-w-xs">
                    <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
                    <input
                      type="search"
                      value={filter}
                      onChange={(e) => setFilter(e.target.value)}
                      placeholder="Find by name or roll"
                      aria-label="Find a student"
                      className="h-9 w-full rounded-lg bg-white pr-3 pl-9 text-sm ring-1 ring-slate-200 ring-inset placeholder:text-slate-400 focus:ring-2 focus:ring-brand-500 focus:outline-none"
                    />
                  </div>
                </div>
                <ul className="divide-y divide-slate-100">
                  {visible.map((s) => {
                    const status = draft.statuses[s.enrollment_id]
                    const changed = s.attendance && status && (status !== s.attendance.status || (draft.remarks[s.enrollment_id] ?? '').trim() !== (s.attendance.remarks ?? ''))
                    return (
                      <li
                        key={s.enrollment_id}
                        className={cn(
                          'flex flex-col gap-3 px-4 py-3 sm:px-6 md:flex-row md:items-center',
                          !status && s.can_mark && 'bg-amber-50/30',
                          changed && 'bg-brand-50/40',
                        )}
                        data-testid="roster-row"
                      >
                        <div className="flex min-w-0 flex-1 items-center gap-3">
                          {data.editable && s.can_mark ? (
                            <input
                              type="checkbox"
                              checked={selected.has(s.enrollment_id)}
                              onChange={() => toggleSelected(s.enrollment_id)}
                              aria-label={`Select ${s.student.full_name}`}
                              className="size-4 rounded border-slate-300 text-brand-600 focus:ring-brand-500"
                            />
                          ) : (
                            <span className="size-4" aria-hidden="true" />
                          )}
                          <span className="w-9 shrink-0 text-right font-mono text-sm text-slate-500 tabular-nums">{s.roll_number}</span>
                          <PersonAvatar person={s.student} size="sm" />
                          <span className="min-w-0">
                            <span className="block truncate text-sm font-medium text-slate-900">{s.student.full_name}</span>
                            <span className="block truncate text-xs text-slate-500">
                              <span className="font-mono">{s.student.admission_number}</span>
                              {s.note && <span className="text-amber-700"> · {s.note}</span>}
                              {changed && <span className="text-brand-700"> · changed</span>}
                            </span>
                          </span>
                        </div>
                        {s.can_mark && data.editable ? (
                          <div className="flex flex-col gap-2 sm:flex-row sm:items-center md:shrink-0">
                            <StatusPicker value={status} onChange={(v) => setStatus(s.enrollment_id, v)} label={`Attendance for ${s.student.full_name}`} />
                            <input
                              type="text"
                              value={draft.remarks[s.enrollment_id] ?? ''}
                              onChange={(e) => setRemark(s.enrollment_id, e.target.value)}
                              maxLength={300}
                              placeholder="Remarks (optional)"
                              aria-label={`Remarks for ${s.student.full_name}`}
                              className="h-9 w-full rounded-lg bg-white px-3 text-sm ring-1 ring-slate-200 ring-inset placeholder:text-slate-400 focus:ring-2 focus:ring-brand-500 focus:outline-none sm:w-48"
                            />
                          </div>
                        ) : (
                          <div className="flex items-center gap-2 md:shrink-0">
                            <AttendanceStatusBadge status={s.state === 'elsewhere' ? null : s.attendance?.status} />
                            {s.attendance?.remarks && <span className="max-w-48 truncate text-xs text-slate-500">{s.attendance.remarks}</span>}
                          </div>
                        )}
                      </li>
                    )
                  })}
                  {visible.length === 0 && <li className="px-6 py-8 text-center text-sm text-slate-500">No student matches “{filter}”.</li>}
                </ul>
              </>
            )}
          </Card>

          {data.editable && markable.length > 0 && (
            <div className="fixed inset-x-0 bottom-0 z-20 border-t border-slate-200 bg-white/95 backdrop-blur lg:pl-64">
              <div className="mx-auto flex max-w-7xl flex-col gap-2 px-4 py-3 sm:flex-row sm:items-center sm:px-6 lg:px-10">
                <p className="flex-1 text-sm text-slate-600" aria-live="polite">
                  {unmarkedMarkable > 0 ? (
                    <span className="font-medium text-amber-800">
                      {unmarkedMarkable} student{unmarkedMarkable === 1 ? '' : 's'} still unmarked
                    </span>
                  ) : changes.dirty ? (
                    `${changes.created ? `${changes.created} new` : ''}${changes.created && changes.updated ? ', ' : ''}${changes.updated ? `${changes.updated} changed` : ''} — not saved yet`
                  ) : (
                    'Everything is saved.'
                  )}
                </p>
                <Button icon={Save} onClick={() => setReviewing(true)} disabled={!changes.dirty || unmarkedMarkable > 0 || conflict}>
                  Review & save
                </Button>
              </div>
            </div>
          )}
        </>
      )}

      <Modal
        open={reviewing}
        onClose={saving ? undefined : () => setReviewing(false)}
        dismissible={!saving}
        icon={ClipboardCheck}
        title="Save attendance?"
        description={data ? `${sectionLabel(data.class, data.section)} · ${formatDateOnly(data.date)}` : undefined}
      >
        {summary && (
          <div className="space-y-4">
            <dl className="grid grid-cols-2 gap-2 sm:grid-cols-3">
              {[
                ['Students', summary.total],
                ['Present', summary.present],
                ['Absent', summary.absent],
                ['Late', summary.late],
                ['Leave', summary.leave],
                ['Unmarked', summary.unmarked],
              ].map(([label, value]) => (
                <div key={label} className="rounded-lg bg-slate-50 px-3 py-2">
                  <dt className="text-xs text-slate-500">{label}</dt>
                  <dd className="text-lg font-semibold text-slate-900 tabular-nums">{value}</dd>
                </div>
              ))}
            </dl>
            <p className="text-sm text-slate-600">
              Attendance <PercentText value={summary.percentage} /> · {changes.created} new record{changes.created === 1 ? '' : 's'}, {changes.updated} change
              {changes.updated === 1 ? '' : 's'}.
            </p>
            {needsReason && (
              <TextArea
                label="Reason for the correction"
                required
                rows={2}
                value={reason}
                onChange={(e) => {
                  setReason(e.target.value)
                  setReasonError('')
                }}
                error={reasonError}
                hint="Kept in the change history of every corrected record."
              />
            )}
            <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
              <Button variant="secondary" onClick={() => setReviewing(false)} disabled={saving}>
                Keep editing
              </Button>
              <Button icon={Save} onClick={save} loading={saving}>
                Save attendance
              </Button>
            </div>
          </div>
        )}
      </Modal>
    </div>
  )
}
