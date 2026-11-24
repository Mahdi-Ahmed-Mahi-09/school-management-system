import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { AlertTriangle, ArrowLeft, ArrowRight, Eye, RefreshCw, Sparkles } from 'lucide-react'
import { getErrorMessage, getFieldErrors } from '../../api/client'
import { enrollmentsApi, promotionsApi } from '../../api/services'
import { DecisionBadge, OutcomeBadge } from '../../components/enrollments/EnrollmentBadges'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Alert, Badge, Card, EmptyState, PageHeader, Skeleton } from '../../components/ui/Display'
import { Input, Select } from '../../components/ui/Field'
import { ConfirmDialog } from '../../components/ui/Modal'
import { TextArea } from '../../components/ui/Sections'
import { useToast } from '../../components/ui/Toast'
import { loadYearOptions, yearOptionLabel } from '../../lib/classes'
import { DECISION_META, loadAllPages, loadClasses, loadYearSections, newRequestId } from '../../lib/enrollments'
import { cn, formatNumber } from '../../lib/utils'

const COMPLETE = '__complete__'

function Step({ n, title, description, children, action }) {
  return (
    <Card>
      <div className="flex flex-wrap items-start gap-3 border-b border-slate-100 px-5 py-3.5 sm:px-6">
        <span className="mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-full bg-brand-600 text-xs font-semibold text-white">{n}</span>
        <div className="min-w-0 flex-1">
          <h2 className="text-sm font-semibold text-slate-900">{title}</h2>
          {description && <p className="mt-0.5 text-[13px] text-slate-500">{description}</p>}
        </div>
        {action}
      </div>
      <div className="p-5 sm:p-6">{children}</div>
    </Card>
  )
}

function Tile({ label, value, tone = 'text-slate-900' }) {
  return (
    <div className="rounded-xl border border-slate-200/80 px-4 py-3">
      <p className="text-xs font-medium text-slate-500">{label}</p>
      <p className={cn('mt-1 text-xl font-semibold tabular-nums', value ? tone : 'text-slate-400')}>{formatNumber(value)}</p>
    </div>
  )
}

/** Suggests the next class by display order (never by comparing names). */
function suggestNext(cls, classes) {
  const later = classes.filter((c) => c.status === 'active' && c.display_order > cls.display_order).sort((a, b) => a.display_order - b.display_order)
  return later[0]?.id ?? ''
}

export default function PromotionWizardPage() {
  const toast = useToast()
  const navigate = useNavigate()
  const [options, setOptions] = useState({ loading: true, error: null, years: [], classes: [] })
  const [sourceId, setSourceId] = useState('')
  const [targetId, setTargetId] = useState('')
  const [pool, setPool] = useState({ loading: false, error: null, enrollments: [] })
  const [targetSections, setTargetSections] = useState([])
  const [selected, setSelected] = useState(() => new Set())
  const [classFilter, setClassFilter] = useState('')
  const [mappings, setMappings] = useState({}) // source class id -> { target, section }
  const [overrides, setOverrides] = useState({}) // enrollment id -> { decision, section, roll }
  const [rollStrategy, setRollStrategy] = useState('keep')
  const [enrollmentDate, setEnrollmentDate] = useState('')
  const [remarks, setRemarks] = useState('')
  const [preview, setPreview] = useState(null)
  const [requestId, setRequestId] = useState(null)
  const [stale, setStale] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState(null)
  const [confirming, setConfirming] = useState(false)
  const [outcomeFilter, setOutcomeFilter] = useState('')

  useEffect(() => {
    Promise.all([loadYearOptions(), loadClasses()])
      .then(([{ years, current }, classes]) => {
        setOptions({ loading: false, error: null, years, classes })
        const source = current ?? [...years].reverse().find((y) => y.status !== 'upcoming') ?? years[years.length - 1]
        setSourceId(source?.id ?? '')
        const next = [...years].reverse().find((y) => source && y.start_date > source.start_date && y.status !== 'completed')
        setTargetId(next?.id ?? '')
      })
      .catch((err) => setOptions({ loading: false, error: getErrorMessage(err), years: [], classes: [] }))
  }, [])

  const source = options.years.find((y) => y.id === sourceId)
  const target = options.years.find((y) => y.id === targetId)
  const invalidate = () => {
    setStale(true)
    setError(null)
  }

  // Students currently enrolled in the source year (the only eligible ones).
  useEffect(() => {
    if (!sourceId) return
    let cancelled = false
    setPool({ loading: true, error: null, enrollments: [] })
    loadAllPages((page) => enrollmentsApi.list({ academic_year_id: sourceId, status: 'enrolled', sort: 'roll', page, page_size: 100 }), 'enrollments')
      .then((enrollments) => {
        if (cancelled) return
        setPool({ loading: false, error: null, enrollments })
        setSelected(new Set(enrollments.map((e) => e.id)))
        setOverrides({})
        setPreview(null)
      })
      .catch((err) => !cancelled && setPool({ loading: false, error: getErrorMessage(err), enrollments: [] }))
    return () => {
      cancelled = true
    }
  }, [sourceId])

  useEffect(() => {
    let cancelled = false
    loadYearSections(targetId)
      .then((items) => !cancelled && setTargetSections(items))
      .catch(() => !cancelled && setTargetSections([]))
    return () => {
      cancelled = true
    }
  }, [targetId])

  // Source classes among the students, in display order, with default mappings.
  const sourceClasses = useMemo(() => {
    const byId = new Map()
    pool.enrollments.forEach((e) => {
      const row = byId.get(e.class.id) ?? { cls: e.class, total: 0, selected: 0 }
      row.total += 1
      if (selected.has(e.id)) row.selected += 1
      byId.set(e.class.id, row)
    })
    return [...byId.values()].sort((a, b) => a.cls.display_order - b.cls.display_order)
  }, [pool.enrollments, selected])

  useEffect(() => {
    setMappings((m) => {
      const next = { ...m }
      sourceClasses.forEach(({ cls }) => {
        if (!next[cls.id]) next[cls.id] = { target: suggestNext(cls, options.classes) || COMPLETE, section: '' }
      })
      return next
    })
  }, [sourceClasses, options.classes])

  const sectionsOf = useCallback((classId) => targetSections.filter((s) => s.class.id === classId), [targetSections])
  const visible = pool.enrollments.filter((e) => !classFilter || e.class.id === classFilter)
  const allVisibleSelected = visible.length > 0 && visible.every((e) => selected.has(e.id))

  const toggle = (id) => {
    setSelected((s) => {
      const next = new Set(s)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
    invalidate()
  }
  const toggleVisible = () => {
    setSelected((s) => {
      const next = new Set(s)
      visible.forEach((e) => (allVisibleSelected ? next.delete(e.id) : next.add(e.id)))
      return next
    })
    invalidate()
  }
  const setOverride = (id, changes) => {
    setOverrides((o) => ({ ...o, [id]: { ...o[id], ...changes } }))
    invalidate()
  }
  const setMapping = (classId, changes) => {
    setMappings((m) => ({ ...m, [classId]: { ...m[classId], ...changes } }))
    invalidate()
  }

  const effectiveDecision = (e) => overrides[e.id]?.decision || (mappings[e.class.id]?.target === COMPLETE ? 'complete' : 'promote')

  const buildPayload = () => {
    const chosen = pool.enrollments.filter((e) => selected.has(e.id))
    return {
      source_academic_year_id: sourceId,
      target_academic_year_id: targetId,
      enrollment_ids: chosen.map((e) => e.id),
      class_mappings: Object.entries(mappings)
        .filter(([classId, m]) => m.target && m.target !== COMPLETE && sourceClasses.some((r) => r.cls.id === classId))
        .map(([classId, m]) => ({ source_class_id: classId, target_class_id: m.target, target_section_id: m.section || '' })),
      overrides: chosen
        .map((e) => {
          const o = overrides[e.id] ?? {}
          const decision = effectiveDecision(e)
          if (decision === 'promote' && !o.section && !o.roll) return null
          return { enrollment_id: e.id, decision, target_section_id: o.section || '', roll_number: o.roll ? Number(o.roll) : null }
        })
        .filter(Boolean),
      roll_strategy: rollStrategy,
      enrollment_date: enrollmentDate,
      remarks: remarks.trim(),
    }
  }

  const runPreview = async () => {
    setBusy(true)
    setError(null)
    try {
      const res = await promotionsApi.preview(buildPayload())
      setPreview(res)
      setRequestId(newRequestId())
      setStale(false)
      if (!enrollmentDate) setEnrollmentDate(res.enrollment_date)
    } catch (err) {
      const fields = getFieldErrors(err)
      setError(Object.values(fields)[0] ?? getErrorMessage(err))
      setPreview(null)
    } finally {
      setBusy(false)
    }
  }

  const execute = async () => {
    setBusy(true)
    try {
      const { batch, replayed } = await promotionsApi.execute({ ...buildPayload(), request_id: requestId })
      toast.success(
        replayed ? 'Promotion already recorded' : 'Promotion complete',
        `${batch.promoted_count} promoted, ${batch.repeated_count} repeating, ${batch.completed_count} completed.`,
      )
      navigate(`/promotions/${batch.id}`, { replace: true })
    } catch (err) {
      setConfirming(false)
      setStale(true)
      setError(getErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  const targetChoices = options.years.filter((y) => source && y.start_date > source.start_date)
  const items = (preview?.items ?? []).filter((i) => !outcomeFilter || i.outcome === outcomeFilter)

  if (options.loading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-40 rounded-xl" />
        <Skeleton className="h-64 rounded-xl" />
      </div>
    )
  }

  return (
    <div className="space-y-6 animate-fade-in">
      <Breadcrumbs items={[{ label: 'Promotions', to: '/promotions' }, { label: 'New promotion' }]} />
      <PageHeader
        title="Promote students"
        description="Move students into the next academic year. Review the preview carefully — nothing changes until you confirm."
        actions={
          <Link to="/promotions" className={buttonClasses('secondary')}>
            <ArrowLeft className="size-4" aria-hidden="true" />
            Promotion history
          </Link>
        }
      />
      {options.error && <Alert tone="danger" title="Couldn't load academic years and classes">{options.error}</Alert>}

      <Step n={1} title="Academic years" description="Students move from the source year into the target year.">
        <div className="grid gap-4 sm:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] sm:items-end">
          <Select
            label="Source academic year"
            value={sourceId}
            onChange={(e) => {
              setSourceId(e.target.value)
              setTargetId('')
              invalidate()
            }}
          >
            {options.years.map((y) => (
              <option key={y.id} value={y.id}>
                {yearOptionLabel(y)}
              </option>
            ))}
          </Select>
          <ArrowRight className="mx-auto hidden size-5 text-slate-400 sm:mb-2.5 sm:block" aria-hidden="true" />
          <Select
            label="Target academic year"
            value={targetId}
            onChange={(e) => {
              setTargetId(e.target.value)
              setEnrollmentDate('')
              invalidate()
            }}
          >
            <option value="">Choose a later year</option>
            {targetChoices.map((y) => (
              <option key={y.id} value={y.id} disabled={y.status === 'completed'}>
                {yearOptionLabel(y)}
              </option>
            ))}
          </Select>
        </div>
        {source && targetChoices.filter((y) => y.status !== 'completed').length === 0 && (
          <Alert tone="warning" title="No later academic year to promote into" className="mt-4">
            <Link to="/academic-years" className="font-semibold underline">Create the next academic year</Link> and its sections first.
          </Alert>
        )}
        {target && targetSections.length === 0 && (
          <Alert tone="warning" title={`${target.name} has no sections yet`} className="mt-4">
            Promoted students need sections to go into. <Link to={`/sections?year=${target.id}`} className="font-semibold underline">Add sections for {target.name}</Link>.
          </Alert>
        )}
      </Step>

      <Step
        n={2}
        title="Students"
        description={`Students currently enrolled in ${source?.name ?? 'the source year'}. Withdrawn, transferred and already-promoted students aren't listed.`}
        action={<Badge className="bg-brand-50 text-brand-700 ring-brand-600/15">{selected.size} of {pool.enrollments.length} selected</Badge>}
      >
        {pool.error ? (
          <Alert tone="danger" title="Couldn't load students">
            {pool.error}
          </Alert>
        ) : pool.loading ? (
          <Skeleton className="h-40" />
        ) : pool.enrollments.length === 0 ? (
          <EmptyState icon={Sparkles} title={`No students are enrolled in ${source?.name ?? 'this year'}`} description="There is nobody to promote from this year." />
        ) : (
          <>
            <div className="mb-3 flex flex-wrap items-center gap-2">
              <Select aria-label="Show class" value={classFilter} onChange={(e) => setClassFilter(e.target.value)} className="w-48">
                <option value="">All classes</option>
                {sourceClasses.map(({ cls, total }) => (
                  <option key={cls.id} value={cls.id}>
                    {cls.name} ({total})
                  </option>
                ))}
              </Select>
              <Button variant="ghost" size="sm" onClick={toggleVisible}>
                {allVisibleSelected ? 'Unselect' : 'Select'} {classFilter ? 'this class' : 'all eligible'}
              </Button>
            </div>
            <div className="max-h-[26rem] overflow-auto rounded-xl border border-slate-200/80">
              <table className="min-w-full text-sm">
                <thead className="sticky top-0 bg-slate-50 text-xs font-medium text-slate-500">
                  <tr>
                    <th className="w-10 px-3 py-2">
                      <span className="sr-only">Select</span>
                    </th>
                    <th className="px-3 py-2 text-left">Student</th>
                    <th className="px-3 py-2 text-left">Current class</th>
                    <th className="px-3 py-2 text-right">Roll</th>
                    <th className="px-3 py-2 text-left">Decision</th>
                    <th className="px-3 py-2 text-left">Destination section</th>
                    <th className="px-3 py-2 text-left">New roll</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {visible.map((e) => {
                    const on = selected.has(e.id)
                    const decision = effectiveDecision(e)
                    const o = overrides[e.id] ?? {}
                    const mapped = mappings[e.class.id]?.target
                    const sectionChoices =
                      decision === 'repeat' ? sectionsOf(e.class.id) : decision === 'promote' ? (mapped && mapped !== COMPLETE ? sectionsOf(mapped) : targetSections) : []
                    return (
                      <tr key={e.id} className={cn(!on && 'opacity-50')}>
                        <td className="px-3 py-2">
                          <input type="checkbox" checked={on} onChange={() => toggle(e.id)} className="size-4 accent-brand-600" aria-label={`Select ${e.student.full_name}`} />
                        </td>
                        <td className="px-3 py-2">
                          <span className="block font-medium text-slate-900">{e.student.full_name}</span>
                          <span className="block font-mono text-xs text-slate-500">{e.student.admission_number}</span>
                        </td>
                        <td className="px-3 py-2 whitespace-nowrap text-slate-700">
                          {e.class.name} – {e.section.name}
                        </td>
                        <td className="px-3 py-2 text-right tabular-nums">{e.roll_number}</td>
                        <td className="px-3 py-2">
                          <select
                            aria-label={`Decision for ${e.student.full_name}`}
                            disabled={!on}
                            value={o.decision ?? ''}
                            onChange={(ev) => setOverride(e.id, { decision: ev.target.value, section: '' })}
                            className="h-8 rounded-lg border border-slate-300 bg-white px-2 text-[13px]"
                          >
                            <option value="">Class default ({DECISION_META[mappings[e.class.id]?.target === COMPLETE ? 'complete' : 'promote'].label.toLowerCase()})</option>
                            {Object.entries(DECISION_META).map(([k, m]) => (
                              <option key={k} value={k}>
                                {m.label}
                              </option>
                            ))}
                          </select>
                        </td>
                        <td className="px-3 py-2">
                          {sectionChoices.length > 0 || decision === 'promote' || decision === 'repeat' ? (
                            <select
                              aria-label={`Destination section for ${e.student.full_name}`}
                              disabled={!on}
                              value={o.section ?? ''}
                              onChange={(ev) => setOverride(e.id, { section: ev.target.value })}
                              className="h-8 max-w-44 rounded-lg border border-slate-300 bg-white px-2 text-[13px]"
                            >
                              <option value="">Automatic</option>
                              {sectionChoices.map((s) => (
                                <option key={s.id} value={s.id}>
                                  {s.class.name} – {s.name}
                                </option>
                              ))}
                            </select>
                          ) : (
                            <span className="text-xs text-slate-400">—</span>
                          )}
                        </td>
                        <td className="px-3 py-2">
                          {decision === 'promote' || decision === 'repeat' ? (
                            <input
                              type="number"
                              min={1}
                              max={9999}
                              disabled={!on}
                              placeholder="Auto"
                              aria-label={`New roll for ${e.student.full_name}`}
                              value={o.roll ?? ''}
                              onChange={(ev) => setOverride(e.id, { roll: ev.target.value })}
                              className="h-8 w-20 rounded-lg border border-slate-300 px-2 text-[13px] tabular-nums"
                            />
                          ) : (
                            <span className="text-xs text-slate-400">—</span>
                          )}
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          </>
        )}
      </Step>

      <Step n={3} title="Class mapping and roll numbers" description="Choose where each class goes. The next class is suggested by display order; change it whenever your school differs.">
        {sourceClasses.length === 0 ? (
          <p className="text-sm text-slate-500">Select students to map their classes.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full text-sm">
              <thead className="text-xs font-medium text-slate-500">
                <tr className="border-b border-slate-100">
                  <th className="py-2 pr-4 text-left">Current class</th>
                  <th className="px-4 py-2 text-left">Students</th>
                  <th className="px-4 py-2 text-left">Destination class in {target?.name ?? 'target year'}</th>
                  <th className="py-2 pl-4 text-left">Default section</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {sourceClasses.map(({ cls, selected: count }) => {
                  const m = mappings[cls.id] ?? { target: '', section: '' }
                  const options2 = m.target && m.target !== COMPLETE ? sectionsOf(m.target) : []
                  return (
                    <tr key={cls.id}>
                      <td className="py-2 pr-4 font-medium text-slate-900">{cls.name}</td>
                      <td className="px-4 py-2 text-slate-600 tabular-nums">{count} selected</td>
                      <td className="px-4 py-2">
                        <select
                          aria-label={`Destination class for ${cls.name}`}
                          value={m.target}
                          onChange={(e) => setMapping(cls.id, { target: e.target.value, section: '' })}
                          className="h-9 w-56 rounded-lg border border-slate-300 bg-white px-2 text-sm"
                        >
                          <option value="">Choose a class</option>
                          {options.classes.map((c) => (
                            <option key={c.id} value={c.id} disabled={c.status !== 'active'}>
                              {c.name}
                              {c.status !== 'active' ? ' (inactive)' : ''}
                            </option>
                          ))}
                          <option value={COMPLETE}>Complete studies (final class)</option>
                        </select>
                      </td>
                      <td className="py-2 pl-4">
                        {m.target === COMPLETE ? (
                          <span className="text-xs text-slate-500">No new enrollment</span>
                        ) : (
                          <select
                            aria-label={`Default section for ${cls.name}`}
                            value={m.section}
                            disabled={!m.target}
                            onChange={(e) => setMapping(cls.id, { section: e.target.value })}
                            className="h-9 w-48 rounded-lg border border-slate-300 bg-white px-2 text-sm"
                          >
                            <option value="">Same section name</option>
                            {options2.map((s) => (
                              <option key={s.id} value={s.id}>
                                Section {s.name}
                              </option>
                            ))}
                          </select>
                        )}
                        {m.target && m.target !== COMPLETE && options2.length === 0 && target && (
                          <p className="mt-1 text-xs text-amber-700">No sections in {target.name}</p>
                        )}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
        <div className="mt-5 grid gap-4 sm:grid-cols-3">
          <Select
            label="Roll numbers"
            value={rollStrategy}
            onChange={(e) => {
              setRollStrategy(e.target.value)
              invalidate()
            }}
            hint={rollStrategy === 'keep' ? 'Keep each current roll; taken ones get the next free number.' : 'Number students 1, 2, 3 … per section in current order.'}
          >
            <option value="keep">Keep current roll numbers</option>
            <option value="sequential">Renumber sequentially</option>
          </Select>
          <Input
            label="Enrollment date"
            type="date"
            value={enrollmentDate}
            placeholder={target?.start_date}
            hint={`Default: the start of ${target?.name ?? 'the target year'}.`}
            onChange={(e) => {
              setEnrollmentDate(e.target.value)
              invalidate()
            }}
          />
          <TextArea
            label="Remarks"
            rows={2}
            maxLength={500}
            placeholder="Optional note for the batch"
            value={remarks}
            onChange={(e) => {
              setRemarks(e.target.value)
              invalidate()
            }}
          />
        </div>
      </Step>

      <Step
        n={4}
        title="Preview"
        description="The server checks eligibility, destinations and roll numbers. Previewing never changes anything."
        action={
          <Button size="sm" icon={preview ? RefreshCw : Eye} variant={preview && !stale ? 'secondary' : 'primary'} loading={busy && !confirming} disabled={!sourceId || !targetId || selected.size === 0} onClick={runPreview}>
            {preview ? 'Refresh preview' : 'Preview promotion'}
          </Button>
        }
      >
        {error && (
          <Alert tone="danger" title="The promotion can't run as configured" className="mb-4">
            {error}
          </Alert>
        )}
        {!preview ? (
          <p className="text-sm text-slate-500">Configure the steps above, then preview.</p>
        ) : (
          <div className="space-y-5">
            {stale && (
              <Alert tone="warning" title="Settings changed since this preview">
                Refresh the preview before confirming.
              </Alert>
            )}
            <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-7">
              <Tile label="Selected" value={preview.summary.selected} />
              <Tile label="Eligible" value={preview.summary.eligible} />
              <Tile label="Promote" value={preview.summary.promote} tone="text-brand-700" />
              <Tile label="Repeat" value={preview.summary.repeat} tone="text-amber-700" />
              <Tile label="Complete" value={preview.summary.complete} tone="text-violet-700" />
              <Tile label="Excluded" value={preview.summary.excluded} tone="text-slate-500" />
              <Tile label="Not eligible" value={preview.summary.skipped} tone="text-rose-700" />
            </div>
            <p className="text-sm text-slate-600">
              {preview.source_academic_year.name} → {preview.target_academic_year.name} · enrollment date {preview.enrollment_date} · roll numbers{' '}
              {preview.roll_strategy === 'keep' ? 'kept where free' : 'renumbered'}
            </p>
            {preview.issues.length > 0 && (
              <Alert tone="danger" icon={AlertTriangle} title={`${preview.issues.length} problem${preview.issues.length === 1 ? '' : 's'} to fix before promoting`}>
                <ul className="mt-1 list-disc space-y-1 pl-5">
                  {preview.issues.map((is, i) => (
                    <li key={i}>{is.message}</li>
                  ))}
                </ul>
              </Alert>
            )}
            {preview.destinations.length > 0 && (
              <div>
                <h3 className="mb-2 text-xs font-semibold tracking-wide text-slate-500 uppercase">Destinations</h3>
                <ul className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
                  {preview.destinations.map((d) => (
                    <li key={d.section.id} className="rounded-lg border border-slate-200/80 px-3 py-2.5 text-sm">
                      <span className="font-medium text-slate-900">
                        {d.class.name} – {d.section.name}
                      </span>
                      <span className="block text-xs text-slate-500">
                        {d.students} student{d.students === 1 ? '' : 's'} · rolls {d.from_roll}–{d.to_roll}
                      </span>
                    </li>
                  ))}
                </ul>
              </div>
            )}
            <div>
              <div className="mb-2 flex items-center justify-between gap-3">
                <h3 className="text-xs font-semibold tracking-wide text-slate-500 uppercase">Students</h3>
                <select aria-label="Filter preview" value={outcomeFilter} onChange={(e) => setOutcomeFilter(e.target.value)} className="h-8 rounded-lg border border-slate-300 bg-white px-2 text-[13px]">
                  <option value="">All outcomes</option>
                  <option value="processed">Will be processed</option>
                  <option value="skipped">Not eligible</option>
                  <option value="excluded">Excluded</option>
                </select>
              </div>
              <div className="max-h-[28rem] overflow-auto rounded-xl border border-slate-200/80">
                <table className="min-w-full text-sm">
                  <thead className="sticky top-0 bg-slate-50 text-xs font-medium text-slate-500">
                    <tr>
                      <th className="px-3 py-2 text-left">Student</th>
                      <th className="px-3 py-2 text-left">From</th>
                      <th className="px-3 py-2 text-left">Decision</th>
                      <th className="px-3 py-2 text-left">To</th>
                      <th className="px-3 py-2 text-left">Outcome</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100">
                    {items.map((it) => (
                      <tr key={it.enrollment_id}>
                        <td className="px-3 py-2">
                          <span className="block font-medium text-slate-900">{it.student.full_name}</span>
                          <span className="block font-mono text-xs text-slate-500">{it.student.admission_number}</span>
                        </td>
                        <td className="px-3 py-2 whitespace-nowrap text-slate-700">
                          {it.current.class.name} – {it.current.section.name} · {it.current.roll_number}
                        </td>
                        <td className="px-3 py-2">
                          <DecisionBadge decision={it.decision} />
                        </td>
                        <td className="px-3 py-2 whitespace-nowrap text-slate-700">
                          {it.target ? (
                            <>
                              {it.target.class.name} – {it.target.section.name} · <span className={cn('tabular-nums', it.target.roll_changed && 'font-semibold text-amber-700')}>{it.target.roll_number || '—'}</span>
                            </>
                          ) : (
                            <span className="text-slate-400">—</span>
                          )}
                        </td>
                        <td className="px-3 py-2">
                          <OutcomeBadge outcome={it.outcome} />
                          {it.message && <span className="mt-0.5 block text-xs text-slate-500">{it.message}</span>}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
            <div className="flex flex-wrap items-center justify-end gap-3 border-t border-slate-100 pt-4">
              {!preview.can_execute && <p className="mr-auto text-[13px] text-rose-700">Fix the problems above and refresh the preview.</p>}
              <Button icon={Sparkles} disabled={!preview.can_execute || stale || busy} onClick={() => setConfirming(true)}>
                Confirm promotion
              </Button>
            </div>
          </div>
        )}
      </Step>

      {confirming && preview && (
        <ConfirmDialog
          open
          tone="brand"
          icon={Sparkles}
          onClose={() => !busy && setConfirming(false)}
          onConfirm={execute}
          loading={busy}
          title={`Promote ${preview.summary.eligible} student${preview.summary.eligible === 1 ? '' : 's'}?`}
          description={`${preview.summary.promote} promoted and ${preview.summary.repeat} repeating into ${preview.target_academic_year.name}; ${preview.summary.complete} marked completed. Their ${preview.source_academic_year.name} enrollments are kept as history. This is saved in one step and recorded as a promotion batch.`}
          confirmLabel="Promote now"
        />
      )}
    </div>
  )
}
