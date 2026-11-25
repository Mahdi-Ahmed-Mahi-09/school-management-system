import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { ArrowLeft, CheckCircle2, CircleMinus, Eye, Layers, RefreshCw, TriangleAlert, XCircle } from 'lucide-react'
import { getErrorMessage, getFieldErrors } from '../../api/client'
import { classesApi, teacherAssignmentsApi } from '../../api/services'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Alert, Badge, Card, PageHeader, Skeleton } from '../../components/ui/Display'
import { Select } from '../../components/ui/Field'
import { ConfirmDialog } from '../../components/ui/Modal'
import { useToast } from '../../components/ui/Toast'
import { loadYearOptions, yearOptionLabel } from '../../lib/classes'
import { loadClasses, loadSections } from '../../lib/enrollments'
import { loadActiveTeachers, teacherLabel } from '../../lib/teacherAssignments'
import { cn } from '../../lib/utils'

const STATUS_META = {
  ready: { label: 'Will be assigned', icon: CheckCircle2, className: 'text-emerald-700' },
  replace: { label: 'Will replace', icon: RefreshCw, className: 'text-amber-700' },
  assigned: { label: 'Assigned', icon: CheckCircle2, className: 'text-emerald-700' },
  replaced: { label: 'Replaced', icon: RefreshCw, className: 'text-amber-700' },
  unchanged: { label: 'Already assigned', icon: CircleMinus, className: 'text-slate-500' },
  conflict: { label: 'Another teacher', icon: TriangleAlert, className: 'text-amber-700' },
  failed: { label: 'Not possible', icon: XCircle, className: 'text-rose-700' },
}

/** One row per section with its outcome; conflicts can be marked for replacement in a preview. */
function SectionResults({ report, replace, onToggleReplace }) {
  return (
    <ul className="divide-y divide-slate-100 rounded-xl border border-slate-200/80" aria-label="Sections">
      {report.items.map((it) => {
        const meta = STATUS_META[it.status] ?? STATUS_META.failed
        const Icon = meta.icon
        const choosable = report.dry_run && (it.status === 'conflict' || it.status === 'replace')
        return (
          <li key={it.section_id} className="flex flex-wrap items-center gap-3 px-4 py-3">
            <Icon className={cn('size-5 shrink-0', meta.className)} aria-hidden="true" />
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium text-slate-900">
                {report.class.name} – Section {it.section?.name ?? '?'} <span className={cn('ml-1 text-xs font-medium', meta.className)}>{meta.label}</span>
              </p>
              {it.message && <p className="text-xs text-slate-500">{it.message}</p>}
            </div>
            {choosable && (
              <label className="flex items-center gap-2 text-[13px] text-slate-700">
                <input type="checkbox" className="size-4 accent-brand-600" checked={replace.has(it.section_id)} onChange={() => onToggleReplace(it.section_id)} />
                Replace {it.current_teacher?.full_name}
              </label>
            )}
          </li>
        )
      })}
    </ul>
  )
}

export default function BulkAssignPage() {
  const toast = useToast()
  const [params] = useSearchParams()
  const [options, setOptions] = useState({ loading: true, error: null, years: [], classes: [], teachers: [] })
  const [form, setForm] = useState({ yearId: '', classId: '', subjectId: '', teacherId: '' })
  const [sections, setSections] = useState({ loading: false, items: [] })
  const [subjects, setSubjects] = useState({ loading: false, items: [] })
  const [chosen, setChosen] = useState(() => new Set())
  const [replace, setReplace] = useState(() => new Set())
  const [preview, setPreview] = useState(null)
  const [stale, setStale] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState(null)
  const [confirming, setConfirming] = useState(false)
  const [result, setResult] = useState(null)

  useEffect(() => {
    Promise.all([loadYearOptions(), loadClasses(), loadActiveTeachers()])
      .then(([{ years, defaultYear }, classes, teachers]) => {
        setOptions({ loading: false, error: null, years, classes, teachers })
        const year = years.find((y) => y.id === params.get('year')) ?? defaultYear
        setForm((f) => ({ ...f, yearId: year && year.status !== 'completed' ? year.id : '' }))
      })
      .catch((err) => setOptions({ loading: false, error: getErrorMessage(err), years: [], classes: [], teachers: [] }))
  }, [params])

  useEffect(() => {
    if (!form.yearId || !form.classId) {
      setSections({ loading: false, items: [] })
      setSubjects({ loading: false, items: [] })
      return
    }
    let cancelled = false
    setSections({ loading: true, items: [] })
    setSubjects({ loading: true, items: [] })
    loadSections(form.yearId, form.classId)
      .then((items) => {
        if (cancelled) return
        setSections({ loading: false, items })
        setChosen(new Set(items.filter((s) => s.status === 'active').map((s) => s.id)))
      })
      .catch(() => !cancelled && setSections({ loading: false, items: [] }))
    classesApi
      .subjects(form.classId, { academic_year_id: form.yearId })
      .then((items) => !cancelled && setSubjects({ loading: false, items }))
      .catch(() => !cancelled && setSubjects({ loading: false, items: [] }))
    return () => {
      cancelled = true
    }
  }, [form.yearId, form.classId])

  const set = (changes) => {
    setForm((f) => ({ ...f, ...changes }))
    setPreview(null)
    setReplace(new Set())
    setError(null)
  }
  const toggle = (setter, id) => {
    setter((s) => {
      const next = new Set(s)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
    setStale(true)
  }

  const payload = (dryRun) => ({
    academic_year_id: form.yearId,
    class_id: form.classId,
    subject_id: form.subjectId,
    teacher_id: form.teacherId,
    section_ids: [...chosen],
    replace_section_ids: [...replace].filter((id) => chosen.has(id)),
    dry_run: dryRun,
  })

  const runPreview = async () => {
    setBusy(true)
    setError(null)
    try {
      setPreview(await teacherAssignmentsApi.bulk(payload(true)))
      setStale(false)
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
      const report = await teacherAssignmentsApi.bulk(payload(false))
      setResult(report)
      setConfirming(false)
      if (report.succeeded) toast.success(`${report.succeeded} section${report.succeeded === 1 ? '' : 's'} assigned`, `${report.teacher.full_name} → ${report.subject.name}.`)
    } catch (err) {
      setConfirming(false)
      setError(getErrorMessage(err))
      setStale(true)
    } finally {
      setBusy(false)
    }
  }

  const ready = form.yearId && form.classId && form.subjectId && form.teacherId && chosen.size > 0
  const teacher = options.teachers.find((t) => t.id === form.teacherId)

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
        <Breadcrumbs items={[{ label: 'Teacher assignments', to: '/teacher-assignments' }, { label: 'Bulk assignment report' }]} />
        <PageHeader title="Bulk assignment report" description={`${result.teacher.full_name} → ${result.subject.name}, ${result.class.name} (${result.academic_year.name})`} />
        <div className="flex flex-wrap gap-2">
          <Badge className="bg-emerald-50 text-emerald-700 ring-emerald-600/15">{result.succeeded} assigned</Badge>
          {result.unchanged > 0 && <Badge className="bg-slate-100 text-slate-600 ring-slate-500/15">{result.unchanged} already assigned</Badge>}
          {result.conflicts > 0 && <Badge className="bg-amber-50 text-amber-700 ring-amber-600/20">{result.conflicts} left with another teacher</Badge>}
          {result.failed > 0 && <Badge className="bg-rose-50 text-rose-700 ring-rose-600/15">{result.failed} not possible</Badge>}
        </div>
        <SectionResults report={result} replace={replace} onToggleReplace={() => {}} />
        <div className="flex flex-wrap gap-2">
          <Link to={`/teacher-assignments?year=${result.academic_year.id}&class=${result.class.id}&subject=${result.subject.id}`} className={buttonClasses('primary')}>
            View these assignments
          </Link>
          <Button
            variant="secondary"
            onClick={() => {
              setResult(null)
              setPreview(null)
              setReplace(new Set())
            }}
          >
            Assign more
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6 animate-fade-in">
      <Breadcrumbs items={[{ label: 'Teacher assignments', to: '/teacher-assignments' }, { label: 'Bulk assignment' }]} />
      <PageHeader
        title="Bulk assignment"
        description="Assign one teacher to a subject in several sections of a class at once. Existing teachers are never replaced unless you choose to."
        actions={
          <Link to="/teacher-assignments" className={buttonClasses('secondary')}>
            <ArrowLeft className="size-4" aria-hidden="true" />
            Back
          </Link>
        }
      />
      {options.error && <Alert tone="danger" title="Couldn't load the options">{options.error}</Alert>}
      <Card className="p-5 sm:p-6">
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Select label="Academic year" required value={form.yearId} onChange={(e) => set({ yearId: e.target.value, subjectId: '' })}>
            <option value="">Choose a year</option>
            {options.years.map((y) => (
              <option key={y.id} value={y.id} disabled={y.status === 'completed'}>
                {yearOptionLabel(y)}
              </option>
            ))}
          </Select>
          <Select label="Class" required value={form.classId} onChange={(e) => set({ classId: e.target.value, subjectId: '' })}>
            <option value="">Choose a class</option>
            {options.classes.map((c) => (
              <option key={c.id} value={c.id} disabled={c.status !== 'active'}>
                {c.name}
              </option>
            ))}
          </Select>
          <Select label="Subject" required value={form.subjectId} disabled={!form.classId || subjects.loading} onChange={(e) => set({ subjectId: e.target.value })}>
            <option value="">{form.classId ? 'Choose a subject' : 'Choose a class first'}</option>
            {subjects.items.map((a) => (
              <option key={a.subject.id} value={a.subject.id} disabled={a.subject.status !== 'active'}>
                {a.subject.name} ({a.subject.code})
              </option>
            ))}
          </Select>
          <Select label="Teacher" required value={form.teacherId} onChange={(e) => set({ teacherId: e.target.value })}>
            <option value="">Choose a teacher</option>
            {options.teachers.map((t) => (
              <option key={t.id} value={t.id}>
                {teacherLabel(t)}
              </option>
            ))}
          </Select>
        </div>
        {form.classId && !subjects.loading && subjects.items.length === 0 && (
          <Alert tone="warning" title="This class has no subjects in the chosen year" className="mt-4">
            <Link to={`/class-subjects?year=${form.yearId}&class=${form.classId}`} className="font-semibold underline">Assign subjects to the class</Link> first.
          </Alert>
        )}
        {form.classId && (
          <fieldset className="mt-5">
            <legend className="mb-2 text-sm font-medium text-slate-700">Sections</legend>
            {sections.loading ? (
              <Skeleton className="h-10" />
            ) : sections.items.length === 0 ? (
              <p className="text-sm text-slate-500">No sections for this class in the chosen year.</p>
            ) : (
              <div className="flex flex-wrap gap-2">
                {sections.items.map((s) => (
                  <label
                    key={s.id}
                    className={cn(
                      'flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-sm',
                      chosen.has(s.id) ? 'border-brand-300 bg-brand-50/60 text-brand-800' : 'border-slate-200 text-slate-700',
                    )}
                  >
                    <input type="checkbox" className="size-4 accent-brand-600" checked={chosen.has(s.id)} onChange={() => toggle(setChosen, s.id)} />
                    Section {s.name}
                    {s.status !== 'active' ? ' (inactive)' : ''}
                  </label>
                ))}
              </div>
            )}
          </fieldset>
        )}
        <div className="mt-5 flex justify-end">
          <Button icon={preview ? RefreshCw : Eye} variant={preview && !stale ? 'secondary' : 'primary'} loading={busy && !confirming} disabled={!ready} onClick={runPreview}>
            {preview ? 'Refresh preview' : 'Preview'}
          </Button>
        </div>
      </Card>

      {error && (
        <Alert tone="danger" title="Can't assign">
          {error}
        </Alert>
      )}
      {preview && (
        <Card className="p-5 sm:p-6">
          <h2 className="text-sm font-semibold text-slate-900">
            {teacher?.full_name} → {preview.subject.name}, {preview.class.name} ({preview.academic_year.name})
          </h2>
          <p className="mt-1 mb-4 text-[13px] text-slate-500">
            {preview.conflicts > 0
              ? 'Some sections already have another teacher. Tick “Replace” to hand them over (the current teacher is kept in the history), or leave them as they are.'
              : 'Nothing has been saved yet.'}
          </p>
          {stale && (
            <Alert tone="warning" title="Choices changed" className="mb-4">
              Refresh the preview before confirming.
            </Alert>
          )}
          <SectionResults report={preview} replace={replace} onToggleReplace={(id) => toggle(setReplace, id)} />
          <div className="mt-5 flex justify-end">
            <Button icon={Layers} disabled={stale || busy || preview.succeeded === 0} onClick={() => setConfirming(true)}>
              Confirm assignment
            </Button>
          </div>
        </Card>
      )}

      {confirming && preview && (
        <ConfirmDialog
          open
          tone="brand"
          icon={Layers}
          onClose={() => !busy && setConfirming(false)}
          onConfirm={execute}
          loading={busy}
          title={`Assign ${teacher?.full_name}?`}
          description={`${preview.subject.name} in ${preview.succeeded} section${preview.succeeded === 1 ? '' : 's'} of ${preview.class.name}. Sections with another teacher you didn't choose to replace stay unchanged. Everything is saved together.`}
          confirmLabel="Assign"
        />
      )}
    </div>
  )
}
