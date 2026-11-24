import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { ArrowLeft, CheckCircle2, ClipboardCheck, RefreshCw, Search, Users, XCircle } from 'lucide-react'
import { getErrorMessage, getFieldErrors } from '../../api/client'
import { enrollmentsApi } from '../../api/services'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Alert, Badge, Card, EmptyState, PageHeader, Skeleton } from '../../components/ui/Display'
import { Input, Select } from '../../components/ui/Field'
import { ConfirmDialog } from '../../components/ui/Modal'
import { PersonAvatar } from '../../components/ui/PersonAvatar'
import { Pagination } from '../../components/ui/Pagination'
import { TextArea } from '../../components/ui/Sections'
import { useToast } from '../../components/ui/Toast'
import { loadYearOptions, yearOptionLabel } from '../../lib/classes'
import { ADMISSION_TYPE_META, defaultEnrollmentDate, loadClasses, loadSections } from '../../lib/enrollments'
import { cn } from '../../lib/utils'

const PAGE_SIZE = 25

function Step({ n, title, children, action }) {
  return (
    <Card>
      <div className="flex flex-wrap items-center gap-3 border-b border-slate-100 px-5 py-3.5 sm:px-6">
        <span className="flex size-6 items-center justify-center rounded-full bg-brand-600 text-xs font-semibold text-white">{n}</span>
        <h2 className="flex-1 text-sm font-semibold text-slate-900">{title}</h2>
        {action}
      </div>
      <div className="p-5 sm:p-6">{children}</div>
    </Card>
  )
}

export default function BulkEnrollPage() {
  const toast = useToast()
  const [params] = useSearchParams()
  const [options, setOptions] = useState({ loading: true, error: null, years: [], classes: [] })
  const [settings, setSettings] = useState({ yearId: '', classId: '', sectionId: '', admissionType: 'new', date: '', startRoll: '', remarks: '' })
  const [sections, setSections] = useState({ loading: false, items: [] })
  const [query, setQuery] = useState('')
  const [page, setPage] = useState(1)
  const [pool, setPool] = useState({ loading: false, error: null, students: [], meta: null })
  const [selected, setSelected] = useState({}) // id -> student
  const [rolls, setRolls] = useState({}) // id -> roll string (manual)
  const [review, setReview] = useState(null) // dry-run report
  const [stale, setStale] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState(null)
  const [confirming, setConfirming] = useState(false)
  const [result, setResult] = useState(null)

  useEffect(() => {
    Promise.all([loadYearOptions(), loadClasses()])
      .then(([{ years, defaultYear }, classes]) => {
        setOptions({ loading: false, error: null, years, classes })
        const year = years.find((y) => y.id === params.get('year')) ?? defaultYear
        const usable = year && year.status !== 'completed' ? year : years.find((y) => y.status !== 'completed')
        setSettings((s) => ({ ...s, yearId: usable?.id ?? '', date: defaultEnrollmentDate(usable) }))
      })
      .catch((err) => setOptions({ loading: false, error: getErrorMessage(err), years: [], classes: [] }))
  }, [params])

  const year = options.years.find((y) => y.id === settings.yearId)
  const cls = options.classes.find((c) => c.id === settings.classId)
  const section = sections.items.find((s) => s.id === settings.sectionId)

  useEffect(() => {
    let cancelled = false
    setSections({ loading: true, items: [] })
    loadSections(settings.yearId, settings.classId)
      .then((items) => {
        if (cancelled) return
        setSections({ loading: false, items })
        setSettings((s) => ({ ...s, sectionId: items.some((x) => x.id === s.sectionId) ? s.sectionId : items.length === 1 ? items[0].id : '' }))
      })
      .catch(() => !cancelled && setSections({ loading: false, items: [] }))
    return () => {
      cancelled = true
    }
  }, [settings.yearId, settings.classId])

  // Students not yet enrolled in the year.
  const loadPool = useCallback(() => {
    if (!settings.yearId) return undefined
    const controller = new AbortController()
    setPool((p) => ({ ...p, loading: true, error: null }))
    enrollmentsApi
      .unenrolled({ academic_year_id: settings.yearId, search: query.trim() || undefined, page, page_size: PAGE_SIZE }, { signal: controller.signal })
      .then(({ students, meta }) => setPool({ loading: false, error: null, students, meta }))
      .catch((err) => !controller.signal.aborted && setPool({ loading: false, error: getErrorMessage(err), students: [], meta: null }))
    return () => controller.abort()
  }, [settings.yearId, query, page])
  useEffect(() => {
    const t = setTimeout(() => loadPool(), 250)
    return () => clearTimeout(t)
  }, [loadPool])

  // Changing the placement or year invalidates the selection and review.
  const update = (changes) => {
    setSettings((s) => ({ ...s, ...changes }))
    if ('yearId' in changes) {
      setSelected({})
      setRolls({})
      setPage(1)
    }
    setReview(null)
    setError(null)
  }

  const selectedList = Object.values(selected)
  const toggle = (s) => {
    setSelected((sel) => {
      const next = { ...sel }
      if (next[s.id]) delete next[s.id]
      else next[s.id] = s
      return next
    })
    setStale(true)
  }
  const pageAllSelected = pool.students.length > 0 && pool.students.every((s) => selected[s.id])
  const togglePage = () => {
    setSelected((sel) => {
      const next = { ...sel }
      pool.students.forEach((s) => (pageAllSelected ? delete next[s.id] : (next[s.id] = s)))
      return next
    })
    setStale(true)
  }

  const payload = (dryRun, useReviewRolls) => ({
    academic_year_id: settings.yearId,
    class_id: settings.classId,
    section_id: settings.sectionId,
    admission_type: settings.admissionType,
    enrollment_date: settings.date,
    remarks: settings.remarks.trim(),
    start_roll: settings.startRoll === '' ? null : Number(settings.startRoll),
    dry_run: dryRun,
    students: selectedList.map((s) => {
      const manual = rolls[s.id]
      let roll = manual === undefined || manual === '' ? null : Number(manual)
      if (roll == null && useReviewRolls) roll = review?.items.find((i) => i.student_id === s.id && i.status === 'ready')?.roll_number ?? null
      return { student_id: s.id, roll_number: roll }
    }),
  })

  const runReview = async () => {
    setBusy(true)
    setError(null)
    try {
      setReview(await enrollmentsApi.bulk(payload(true, false)))
      setStale(false)
    } catch (err) {
      const fields = getFieldErrors(err)
      setError(Object.values(fields)[0] ?? getErrorMessage(err))
      setReview(null)
    } finally {
      setBusy(false)
    }
  }

  const execute = async () => {
    setBusy(true)
    try {
      // Send the reviewed roll numbers, so what was reviewed is what is saved.
      const report = await enrollmentsApi.bulk(payload(false, true))
      setResult(report)
      setConfirming(false)
      if (report.succeeded) toast.success(`${report.succeeded} student${report.succeeded === 1 ? '' : 's'} enrolled`, `${report.class.name} – ${report.section.name} (${report.academic_year.name}).`)
      if (report.failed) toast.error(`${report.failed} not enrolled`, 'See the report for the reasons.')
    } catch (err) {
      setConfirming(false)
      setError(getErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  const reset = () => {
    setResult(null)
    setReview(null)
    setSelected({})
    setRolls({})
    loadPool()
  }

  const readyCount = review?.items.filter((i) => i.status === 'ready').length ?? 0
  const placementReady = settings.yearId && settings.classId && settings.sectionId && settings.date
  const reviewRows = useMemo(() => review?.items ?? [], [review])

  if (options.loading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-48 rounded-xl" />
      </div>
    )
  }

  if (result) {
    return (
      <div className="space-y-6 animate-fade-in">
        <Breadcrumbs items={[{ label: 'Enrollments', to: '/enrollments' }, { label: 'Bulk enrollment report' }]} />
        <PageHeader title="Bulk enrollment report" description={`${result.class.name} – Section ${result.section.name}, ${result.academic_year.name}`} />
        <div className="grid gap-4 sm:grid-cols-3">
          <Card className="p-5">
            <p className="text-[13px] font-medium text-slate-500">Selected</p>
            <p className="mt-2 text-[28px] font-semibold text-slate-900 tabular-nums">{result.total}</p>
          </Card>
          <Card className="p-5">
            <p className="text-[13px] font-medium text-emerald-700">Enrolled</p>
            <p className="mt-2 text-[28px] font-semibold text-emerald-700 tabular-nums">{result.succeeded}</p>
          </Card>
          <Card className="p-5">
            <p className="text-[13px] font-medium text-rose-700">Not enrolled</p>
            <p className="mt-2 text-[28px] font-semibold text-rose-700 tabular-nums">{result.failed}</p>
          </Card>
        </div>
        <Card className="overflow-hidden">
          <ul className="divide-y divide-slate-100" aria-label="Results">
            {result.items.map((it) => (
              <li key={it.student_id} className="flex items-center gap-3 px-5 py-3 sm:px-6">
                {it.status === 'enrolled' ? <CheckCircle2 className="size-5 text-emerald-600" aria-hidden="true" /> : <XCircle className="size-5 text-rose-600" aria-hidden="true" />}
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-sm font-medium text-slate-900">{it.student?.full_name ?? it.student_id}</span>
                  <span className="block text-xs text-slate-500">{it.status === 'enrolled' ? `Roll ${it.roll_number}` : it.message}</span>
                </span>
                {it.enrollment && (
                  <Link to={`/enrollments/${it.enrollment.id}`} className="text-[13px] font-medium text-brand-700 hover:underline">
                    View
                  </Link>
                )}
              </li>
            ))}
          </ul>
        </Card>
        <div className="flex flex-wrap gap-2">
          <Link to={`/enrollments?year=${result.academic_year.id}&class=${result.class.id}&section=${result.section.id}`} className={buttonClasses('primary')}>
            View section enrollments
          </Link>
          <Button variant="secondary" onClick={reset}>
            Enroll more students
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6 animate-fade-in">
      <Breadcrumbs items={[{ label: 'Enrollments', to: '/enrollments' }, { label: 'Bulk enrollment' }]} />
      <PageHeader
        title="Bulk enrollment"
        description="Enroll several existing students into one section at once. Nothing is saved until you confirm."
        actions={
          <Link to="/enrollments" className={buttonClasses('secondary')}>
            <ArrowLeft className="size-4" aria-hidden="true" />
            Back
          </Link>
        }
      />
      {options.error && <Alert tone="danger" title="Couldn't load academic years and classes">{options.error}</Alert>}
      {!options.years.some((y) => y.status !== 'completed') ? (
        <Card>
          <EmptyState
            icon={Users}
            title="No open academic year"
            description="Students can only be enrolled in the current or an upcoming academic year."
            action={
              <Link to="/academic-years" className={buttonClasses('primary', 'sm')}>
                Go to academic years
              </Link>
            }
          />
        </Card>
      ) : (
        <>
          <Step n={1} title="Where are they enrolling?">
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              <Select label="Academic year" required value={settings.yearId} onChange={(e) => update({ yearId: e.target.value, date: defaultEnrollmentDate(options.years.find((y) => y.id === e.target.value)) })}>
                {options.years.map((y) => (
                  <option key={y.id} value={y.id} disabled={y.status === 'completed'}>
                    {yearOptionLabel(y)}
                  </option>
                ))}
              </Select>
              <Select label="Class" required value={settings.classId} onChange={(e) => update({ classId: e.target.value, sectionId: '' })}>
                <option value="">Choose a class</option>
                {options.classes.map((c) => (
                  <option key={c.id} value={c.id} disabled={c.status !== 'active'}>
                    {c.name}
                    {c.status !== 'active' ? ' (inactive)' : ''}
                  </option>
                ))}
              </Select>
              <Select label="Section" required value={settings.sectionId} disabled={!settings.classId || sections.loading} onChange={(e) => update({ sectionId: e.target.value })}>
                <option value="">{sections.loading ? 'Loading…' : 'Choose a section'}</option>
                {sections.items.map((s) => (
                  <option key={s.id} value={s.id} disabled={s.status !== 'active'}>
                    Section {s.name}
                    {s.status !== 'active' ? ' (inactive)' : ''}
                  </option>
                ))}
              </Select>
              <Select label="Admission type" value={settings.admissionType} onChange={(e) => update({ admissionType: e.target.value })}>
                {Object.entries(ADMISSION_TYPE_META).map(([k, m]) => (
                  <option key={k} value={k}>
                    {m.label}
                  </option>
                ))}
              </Select>
              <Input label="Enrollment date" type="date" required value={settings.date} onChange={(e) => update({ date: e.target.value })} />
              <Input
                label="Starting roll number"
                type="number"
                min={1}
                max={9999}
                placeholder="After the highest used"
                hint="Automatic roll numbers count up from here."
                value={settings.startRoll}
                onChange={(e) => update({ startRoll: e.target.value })}
              />
            </div>
            {settings.classId && !sections.loading && sections.items.length === 0 && (
              <Alert tone="warning" title={`${cls?.name} has no sections in ${year?.name}`} className="mt-4">
                <Link to={`/sections?year=${settings.yearId}&class=${settings.classId}`} className="font-semibold underline">Add a section</Link> first.
              </Alert>
            )}
            <TextArea label="Remarks" rows={2} maxLength={500} placeholder="Optional, saved on every enrollment" className="mt-4" value={settings.remarks} onChange={(e) => update({ remarks: e.target.value })} />
          </Step>

          <Step
            n={2}
            title={`Choose students not yet enrolled in ${year?.name ?? 'the year'}`}
            action={<Badge className="bg-brand-50 text-brand-700 ring-brand-600/15">{selectedList.length} selected</Badge>}
          >
            <div className="mb-3 flex flex-col gap-3 sm:flex-row sm:items-center">
              <div className="relative flex-1">
                <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
                <input
                  type="search"
                  value={query}
                  onChange={(e) => {
                    setQuery(e.target.value)
                    setPage(1)
                  }}
                  placeholder="Search by name or student ID"
                  aria-label="Search students"
                  className="h-10 w-full rounded-lg border border-slate-300 bg-white pr-3 pl-9 text-sm shadow-sm placeholder:text-slate-400 focus:border-brand-500 focus:ring-4 focus:ring-brand-500/15 focus:outline-none"
                />
              </div>
              {pool.students.length > 0 && (
                <Button variant="ghost" size="sm" onClick={togglePage}>
                  {pageAllSelected ? 'Unselect this page' : 'Select this page'}
                </Button>
              )}
            </div>
            {pool.error ? (
              <Alert tone="danger" title="Couldn't load students">
                {pool.error}
              </Alert>
            ) : pool.loading && !pool.students.length ? (
              <div className="space-y-2">
                {[0, 1, 2].map((i) => (
                  <Skeleton key={i} className="h-12" />
                ))}
              </div>
            ) : pool.students.length === 0 ? (
              <p className="rounded-xl border border-dashed border-slate-200 px-4 py-6 text-center text-sm text-slate-500">
                {query ? `No unenrolled students match “${query}”.` : `Every active student is already enrolled in ${year?.name}.`}
              </p>
            ) : (
              <>
                <ul className="divide-y divide-slate-100 rounded-xl border border-slate-200/80" aria-label="Students not yet enrolled">
                  {pool.students.map((s) => (
                    <li key={s.id} className={cn(selected[s.id] && 'bg-brand-50/40')}>
                      <label className="flex cursor-pointer items-center gap-3 px-4 py-2.5">
                        <input type="checkbox" checked={!!selected[s.id]} onChange={() => toggle(s)} className="size-4 accent-brand-600" aria-label={`Select ${s.full_name}`} />
                        <PersonAvatar person={s} size="sm" />
                        <span className="min-w-0 flex-1">
                          <span className="block truncate text-sm font-medium text-slate-900">{s.full_name}</span>
                          <span className="block font-mono text-xs text-slate-500">{s.admission_number}</span>
                        </span>
                      </label>
                    </li>
                  ))}
                </ul>
                {pool.meta && pool.meta.total_pages > 1 && (
                  <div className="-mx-5 -mb-5 mt-3 sm:-mx-6 sm:-mb-6">
                    <Pagination page={pool.meta.page} pageSize={pool.meta.page_size} total={pool.meta.total} totalPages={pool.meta.total_pages} onPageChange={setPage} />
                  </div>
                )}
              </>
            )}
          </Step>

          <Step
            n={3}
            title="Review roll numbers and confirm"
            action={
              <Button size="sm" variant={review ? 'secondary' : 'primary'} icon={review ? RefreshCw : ClipboardCheck} loading={busy && !confirming} disabled={!placementReady || selectedList.length === 0} onClick={runReview}>
                {review ? 'Check again' : 'Review'}
              </Button>
            }
          >
            {error && (
              <Alert tone="danger" title="Can't enroll into this section" className="mb-4">
                {error}
              </Alert>
            )}
            {!review ? (
              <p className="text-sm text-slate-500">
                {selectedList.length === 0
                  ? 'Select students above, then review. The server checks every student and assigns roll numbers before anything is saved.'
                  : `Review ${selectedList.length} selected student${selectedList.length === 1 ? '' : 's'} for ${section ? `${cls?.name} – Section ${section.name}` : 'the chosen section'}.`}
              </p>
            ) : (
              <>
                <div className="mb-4 flex flex-wrap gap-2 text-sm">
                  <Badge className="bg-emerald-50 text-emerald-700 ring-emerald-600/15">{readyCount} ready</Badge>
                  {review.failed > 0 && <Badge className="bg-rose-50 text-rose-700 ring-rose-600/15">{review.failed} can't be enrolled</Badge>}
                  {stale && <span className="text-amber-700">The selection or roll numbers changed — check again before confirming.</span>}
                </div>
                <div className="overflow-x-auto">
                  <table className="min-w-full text-sm">
                    <thead className="text-xs font-medium text-slate-500">
                      <tr className="border-b border-slate-100">
                        <th className="py-2 pr-4 text-left">Student</th>
                        <th className="w-32 px-4 py-2 text-left">Roll number</th>
                        <th className="py-2 pl-4 text-left">Check</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100">
                      {reviewRows.map((it) => (
                        <tr key={it.student_id}>
                          <td className="py-2 pr-4">
                            <span className="block font-medium text-slate-900">{it.student?.full_name ?? 'Unknown student'}</span>
                            <span className="block font-mono text-xs text-slate-500">{it.student?.admission_number}</span>
                          </td>
                          <td className="px-4 py-2">
                            <input
                              type="number"
                              min={1}
                              max={9999}
                              aria-label={`Roll number for ${it.student?.full_name}`}
                              value={rolls[it.student_id] ?? (it.roll_number ?? '')}
                              onChange={(e) => {
                                setRolls((r) => ({ ...r, [it.student_id]: e.target.value }))
                                setStale(true)
                              }}
                              className="h-9 w-24 rounded-lg border border-slate-300 px-2 text-sm tabular-nums focus:border-brand-500 focus:ring-4 focus:ring-brand-500/15 focus:outline-none"
                            />
                          </td>
                          <td className="py-2 pl-4">
                            {it.status === 'ready' ? (
                              <span className="text-emerald-700">{it.roll_assigned && rolls[it.student_id] === undefined ? 'Ready · roll assigned automatically' : 'Ready'}</span>
                            ) : (
                              <span className="text-rose-700">{it.message}</span>
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                <div className="mt-5 flex flex-wrap items-center justify-end gap-3 border-t border-slate-100 pt-4">
                  {review.failed > 0 && <p className="mr-auto text-[13px] text-slate-500">Students that can't be enrolled stay unenrolled and are listed in the final report.</p>}
                  <Button disabled={stale || readyCount === 0 || busy} onClick={() => setConfirming(true)}>
                    Enroll {readyCount} student{readyCount === 1 ? '' : 's'}
                  </Button>
                </div>
              </>
            )}
          </Step>
        </>
      )}

      {confirming && review && (
        <ConfirmDialog
          open
          tone="brand"
          icon={ClipboardCheck}
          onClose={() => !busy && setConfirming(false)}
          onConfirm={execute}
          loading={busy}
          title={`Enroll ${readyCount} student${readyCount === 1 ? '' : 's'}?`}
          description={`Into ${review.class.name} – Section ${review.section.name} for ${review.academic_year.name}, as ${ADMISSION_TYPE_META[settings.admissionType].label.toLowerCase()} admissions. All are saved together.`}
          confirmLabel="Confirm enrollment"
        />
      )}
    </div>
  )
}
