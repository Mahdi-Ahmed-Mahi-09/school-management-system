import { useCallback, useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { Plus, Save, Settings2, Trash2 } from 'lucide-react'
import { getErrorMessage, getFieldErrors } from '../../api/client'
import { resultsApi } from '../../api/services'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { Alert, Badge, Card, EmptyState, ErrorState, PageHeader, Skeleton } from '../../components/ui/Display'
import { Input } from '../../components/ui/Field'
import { TextArea } from '../../components/ui/Sections'
import { useToast } from '../../components/ui/Toast'
import { formatMarks, fromHundredths, parseMarksInput } from '../../lib/results'
import { formatDateOnly } from '../../lib/students'
import { cn } from '../../lib/utils'

function toForm(scheme) {
  return {
    full_marks: formatMarks(scheme.full_marks),
    pass_marks: formatMarks(scheme.pass_marks),
    instructions: scheme.instructions ?? '',
    components: scheme.components.map((c) => ({ id: c.id, name: c.name, max_marks: formatMarks(c.max_marks) })),
    reason: '',
  }
}

/** Full marks, pass marks and optional components of one paper. */
function SchemeEditor({ item, onSaved }) {
  const toast = useToast()
  const { paper, scheme } = item
  const [form, setForm] = useState(() => toForm(scheme))
  const [errors, setErrors] = useState({})
  const [saving, setSaving] = useState(false)
  const locked = scheme.marks_entered > 0
  const sum = form.components.reduce((n, c) => n + (parseMarksInput(c.max_marks).value ?? 0), 0)
  const full = parseMarksInput(form.full_marks).value
  const mismatch = form.components.length > 0 && full != null && sum !== full
  const set = (k, v) => setForm((f) => ({ ...f, [k]: v }))
  const setComp = (i, patch) => setForm((f) => ({ ...f, components: f.components.map((c, j) => (j === i ? { ...c, ...patch } : c)) }))

  const save = async () => {
    setSaving(true)
    setErrors({})
    try {
      await resultsApi.saveScheme(paper.schedule_id, {
        full_marks: form.full_marks.trim(),
        pass_marks: form.pass_marks.trim(),
        instructions: form.instructions,
        components: form.components.map((c) => ({ id: c.id || undefined, name: c.name, max_marks: c.max_marks.trim() })),
        reason: form.reason,
      })
      toast.success('Marking scheme saved', `${paper.class.name} – ${paper.subject.name}`)
      onSaved()
    } catch (err) {
      setErrors(getFieldErrors(err))
      toast.error("Couldn't save the scheme", getErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-4 border-t border-slate-100 bg-slate-50/40 p-5 sm:px-6">
      {locked && (
        <Alert tone="warning" title={`${scheme.marks_entered} student${scheme.marks_entered === 1 ? ' has' : 's have'} marks for this paper`}>
          Changes need a reason and are recorded. Components can be renamed or resized but not added or removed, and maximums can't drop below marks already given.
        </Alert>
      )}
      <div className="grid gap-3 sm:grid-cols-3">
        <Input label="Full marks" required inputMode="decimal" value={form.full_marks} onChange={(e) => set('full_marks', e.target.value)} error={errors.full_marks} />
        <Input label="Pass marks" required inputMode="decimal" value={form.pass_marks} onChange={(e) => set('pass_marks', e.target.value)} error={errors.pass_marks}
          hint="Students below this fail the subject, even if their percentage would be a D." />
      </div>
      <div>
        <div className="mb-2 flex items-center justify-between">
          <p className="text-sm font-medium text-slate-700">Components</p>
          <Button size="sm" variant="ghost" icon={Plus} disabled={locked || form.components.length >= 10}
            onClick={() => set('components', [...form.components, { id: '', name: '', max_marks: '' }])}>
            Add component
          </Button>
        </div>
        {form.components.length === 0 ? (
          <p className="text-sm text-slate-500">No components: marks are entered as one total out of the full marks.</p>
        ) : (
          <ul className="space-y-2">
            {form.components.map((c, i) => (
              <li key={i} className="flex items-start gap-2">
                <Input label={`Component ${i + 1} name`} className="flex-1" value={c.name} placeholder="e.g. Theory" maxLength={40}
                  onChange={(e) => setComp(i, { name: e.target.value })} error={errors[`components[${i}].name`]} />
                <Input label="Maximum" className="w-32" inputMode="decimal" value={c.max_marks} onChange={(e) => setComp(i, { max_marks: e.target.value })}
                  error={errors[`components[${i}].max_marks`]} />
                <Button variant="ghost" size="sm" icon={Trash2} className="mt-7" disabled={locked} aria-label={`Remove component ${i + 1}`}
                  onClick={() => set('components', form.components.filter((_, j) => j !== i))} />
              </li>
            ))}
          </ul>
        )}
        {form.components.length > 0 && (
          <p className={cn('mt-2 text-sm', mismatch ? 'text-rose-600' : 'text-slate-500')}>
            Components add up to {fromHundredths(sum)}
            {mismatch && ` — they must equal the full marks (${form.full_marks})`}.
          </p>
        )}
        {errors.components && <p className="mt-1 text-sm text-rose-600">{errors.components}</p>}
      </div>
      <TextArea label="Instructions for markers" rows={2} value={form.instructions} onChange={(e) => set('instructions', e.target.value)} error={errors.instructions} />
      {locked && <TextArea label="Reason for the change" required rows={2} value={form.reason} onChange={(e) => set('reason', e.target.value)} error={errors.reason} />}
      <div className="flex justify-end">
        <Button icon={Save} onClick={save} loading={saving} disabled={mismatch}>
          Save scheme
        </Button>
      </div>
    </div>
  )
}

export default function MarkSchemesPage() {
  const { examId } = useParams()
  const [state, setState] = useState({ loading: true, error: null, items: [] })
  const [open, setOpen] = useState(null)
  const [reloadKey, setReloadKey] = useState(0)
  const reload = useCallback(() => setReloadKey((k) => k + 1), [])

  useEffect(() => {
    let cancelled = false
    resultsApi
      .schemes(examId)
      .then((items) => !cancelled && setState({ loading: false, error: null, items }))
      .catch((err) => !cancelled && setState({ loading: false, error: getErrorMessage(err), items: [] }))
    return () => {
      cancelled = true
    }
  }, [examId, reloadKey])

  const examName = state.items[0]?.paper.examination.name ?? 'Examination'
  const groups = []
  for (const it of state.items) {
    let g = groups.find((x) => x.class.id === it.paper.class.id)
    if (!g) groups.push((g = { class: it.paper.class, items: [] }))
    g.items.push(it)
  }

  return (
    <div className="space-y-6 animate-fade-in">
      <div className="space-y-3">
        <Breadcrumbs items={[{ label: 'Marks & results', to: `/results?exam=${examId}` }, { label: 'Marks configuration' }]} />
        <PageHeader
          title={`Marks configuration · ${examName}`}
          description="Each paper starts from its subject's full and pass marks. Override them for this examination and split the marks into components where needed; the subject itself is not changed."
        />
      </div>
      {state.error ? (
        <Card>
          <ErrorState title="Couldn't load the papers" message={state.error} onRetry={reload} />
        </Card>
      ) : state.loading ? (
        <Skeleton className="h-64" />
      ) : groups.length === 0 ? (
        <Card>
          <EmptyState icon={Settings2} title="No papers" description="Schedule papers for this examination first." />
        </Card>
      ) : (
        groups.map((g) => (
          <Card key={g.class.id} className="overflow-hidden">
            <h2 className="border-b border-slate-100 px-5 py-3 text-[15px] font-semibold text-slate-900 sm:px-6">{g.class.name}</h2>
            <ul className="divide-y divide-slate-100">
              {g.items.map((it) => {
                const sid = it.paper.schedule_id
                return (
                  <li key={sid}>
                    <button
                      type="button"
                      onClick={() => setOpen(open === sid ? null : sid)}
                      aria-expanded={open === sid}
                      className="flex w-full flex-wrap items-center gap-3 px-5 py-3 text-left hover:bg-slate-50 sm:px-6"
                    >
                      <span className="min-w-0 flex-1">
                        <span className="block text-sm font-medium text-slate-900">{it.paper.subject.name}</span>
                        <span className="block text-xs text-slate-500">{formatDateOnly(it.paper.exam_date)}</span>
                      </span>
                      <span className="text-sm text-slate-700 tabular-nums">
                        {formatMarks(it.scheme.full_marks)} full · {formatMarks(it.scheme.pass_marks)} pass
                      </span>
                      <span className="text-xs text-slate-500">
                        {it.scheme.components.length ? it.scheme.components.map((c) => `${c.name} ${formatMarks(c.max_marks)}`).join(' + ') : 'Single total'}
                      </span>
                      {it.scheme.saved ? (
                        <Badge className="bg-emerald-50 text-emerald-700 ring-emerald-600/15">Configured</Badge>
                      ) : (
                        <Badge className="bg-slate-50 text-slate-600 ring-slate-500/15">Subject default</Badge>
                      )}
                    </button>
                    {open === sid && <SchemeEditor key={`${sid}-${it.scheme.version}-${it.scheme.saved}`} item={it} onSaved={reload} />}
                  </li>
                )
              })}
            </ul>
          </Card>
        ))
      )}
    </div>
  )
}
