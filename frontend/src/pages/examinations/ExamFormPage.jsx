import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { Check, Save, X } from 'lucide-react'
import { getErrorMessage, getFieldErrors } from '../../api/client'
import { examinationsApi, examTypesApi } from '../../api/services'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { Alert, Card, ErrorState, PageHeader } from '../../components/ui/Display'
import { Input, Select } from '../../components/ui/Field'
import { FormSection, FormSkeleton, TextArea } from '../../components/ui/Sections'
import { useToast } from '../../components/ui/Toast'
import { loadYearOptions, yearOptionLabel } from '../../lib/classes'
import { loadClasses } from '../../lib/enrollments'
import { cn } from '../../lib/utils'

const EMPTY = { name: '', exam_type_id: '', academic_year_id: '', start_date: '', end_date: '', description: '', instructions: '', reason: '' }

function ClassPicker({ classes, selected, onToggle, error }) {
  const chosen = classes.filter((c) => selected.includes(c.id))
  return (
    <div className="space-y-3">
      <div className="flex min-h-10 flex-wrap gap-2 rounded-lg border border-dashed border-slate-200 p-2" aria-live="polite">
        {chosen.length === 0 ? (
          <span className="px-1 py-1 text-sm text-slate-400">No classes selected yet.</span>
        ) : (
          chosen.map((c) => (
            <span key={c.id} className="inline-flex items-center gap-1 rounded-md bg-brand-50 py-1 pr-1 pl-2.5 text-sm font-medium text-brand-700">
              {c.name}
              <button type="button" onClick={() => onToggle(c.id)} className="rounded p-0.5 hover:bg-brand-100" aria-label={`Remove ${c.name}`}>
                <X className="size-3.5" />
              </button>
            </span>
          ))
        )}
      </div>
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4" role="group" aria-label="Classes">
        {classes.map((c) => {
          const on = selected.includes(c.id)
          const inactive = c.status !== 'active'
          return (
            <button
              key={c.id}
              type="button"
              disabled={inactive}
              onClick={() => onToggle(c.id)}
              aria-pressed={on}
              className={cn(
                'flex items-center justify-between gap-2 rounded-lg border px-3 py-2 text-left text-sm transition',
                on ? 'border-brand-300 bg-brand-50 text-brand-800' : 'border-slate-200 bg-white text-slate-700 hover:border-slate-300',
                inactive && 'cursor-not-allowed opacity-50',
              )}
            >
              <span className="truncate">
                {c.name}
                {inactive && <span className="block text-xs text-slate-500">Inactive</span>}
              </span>
              {on && <Check className="size-4 shrink-0 text-brand-600" aria-hidden="true" />}
            </button>
          )
        })}
      </div>
      {error && <p className="text-[13px] text-rose-600">{error}</p>}
    </div>
  )
}

/** Create or edit an examination (classes are chosen when creating). */
export default function ExamFormPage() {
  const { id } = useParams()
  const editing = Boolean(id)
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const toast = useToast()
  const [state, setState] = useState({ loading: true, error: null, years: [], types: [], classes: [], exam: null })
  const [values, setValues] = useState(EMPTY)
  const [classIds, setClassIds] = useState([])
  const [errors, setErrors] = useState({})
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    let cancelled = false
    Promise.all([loadYearOptions(), examTypesApi.list(), loadClasses(), editing ? examinationsApi.get(id) : Promise.resolve(null)])
      .then(([y, types, classes, exam]) => {
        if (cancelled) return
        setState({ loading: false, error: null, years: y.years, types, classes, exam })
        if (exam) {
          setValues({
            name: exam.name, exam_type_id: exam.exam_type.id, academic_year_id: exam.academic_year.id, start_date: exam.start_date, end_date: exam.end_date,
            description: exam.description ?? '', instructions: exam.instructions ?? '', reason: '',
          })
        } else {
          const wanted = y.years.find((x) => x.id === params.get('year') && x.status !== 'completed')
          const def = wanted ?? (y.defaultYear?.status !== 'completed' ? y.defaultYear : y.years.find((x) => x.status !== 'completed'))
          setValues((v) => ({ ...v, academic_year_id: def?.id ?? '' }))
        }
      })
      .catch((err) => !cancelled && setState((s) => ({ ...s, loading: false, error: getErrorMessage(err) })))
    return () => {
      cancelled = true
    }
  }, [editing, id, params])

  const exam = state.exam
  const year = state.years.find((y) => y.id === values.academic_year_id)
  const type = state.types.find((t) => t.id === values.exam_type_id)
  const published = exam?.status === 'published'
  const yearLocked = editing && (exam?.status !== 'draft' || exam?.class_count > 0)
  const suggestedName = type && year ? `${type.name} ${year.name}` : ''
  const typeOptions = useMemo(() => state.types.filter((t) => t.status === 'active' || t.id === exam?.exam_type.id), [state.types, exam])
  const set = (k) => (e) => {
    setValues((v) => ({ ...v, [k]: e.target.value }))
    setErrors((er) => ({ ...er, [k]: undefined }))
  }

  const submit = async (e) => {
    e.preventDefault()
    const next = {}
    const name = values.name.trim() || suggestedName
    if (!name) next.name = 'Enter a name for the examination.'
    if (!values.exam_type_id) next.exam_type_id = 'Choose an examination type.'
    if (!values.academic_year_id) next.academic_year_id = 'Choose an academic year.'
    if (!values.start_date) next.start_date = 'Choose the first day.'
    if (!values.end_date) next.end_date = 'Choose the last day.'
    if (values.start_date && values.end_date && values.end_date < values.start_date) next.end_date = 'The end date must be on or after the start date.'
    if (year && values.start_date && (values.start_date < year.start_date || values.end_date > year.end_date)) next.start_date = `The dates must fall within ${year.name}.`
    if (published && !values.reason.trim()) next.reason = 'This examination is published. Give a reason for the change.'
    setErrors(next)
    if (Object.keys(next).length) return
    setSaving(true)
    const payload = { ...values, name }
    try {
      if (editing) {
        await examinationsApi.update(id, payload)
        toast.success('Examination updated', name)
        navigate(`/examinations/${id}`)
      } else {
        delete payload.reason
        const x = await examinationsApi.create({ ...payload, class_ids: classIds })
        toast.success('Examination created', classIds.length ? 'Now build the subject-wise timetable.' : 'Add classes, then build the timetable.')
        navigate(classIds.length ? `/examinations/${x.id}/schedule` : `/examinations/${x.id}`)
      }
    } catch (err) {
      setErrors(getFieldErrors(err))
      toast.error(editing ? "Couldn't save the examination" : "Couldn't create the examination", getErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  const title = editing ? `Edit ${exam?.name ?? 'examination'}` : 'Create examination'
  const crumbs = [{ label: 'Examinations', to: '/examinations' }, ...(editing && exam ? [{ label: exam.name, to: `/examinations/${id}` }] : []), { label: editing ? 'Edit' : 'New' }]

  if (state.error) {
    return (
      <div className="space-y-6">
        <Breadcrumbs items={crumbs} />
        <Card>
          <ErrorState title="Couldn't load the form" message={state.error} />
        </Card>
      </div>
    )
  }
  if (state.loading) return <FormSkeleton sections={2} />
  if (editing && (exam.status === 'completed' || exam.status === 'cancelled')) {
    return (
      <div className="space-y-6">
        <Breadcrumbs items={crumbs} />
        <Alert tone="info" title={`${exam.name} is ${exam.status}`}>
          Completed and cancelled examinations are kept as history and can't be edited.
        </Alert>
      </div>
    )
  }

  return (
    <form onSubmit={submit} noValidate className="space-y-6 animate-fade-in">
      <div className="space-y-3">
        <Breadcrumbs items={crumbs} />
        <PageHeader title={title} description={editing ? 'Change the examination details. Classes and papers are managed on the schedule page.' : 'Name the examination, choose its dates and the classes that sit it.'} />
      </div>
      {published && (
        <Alert tone="warning" title="This timetable is published">
          Changes are recorded in the examination&apos;s history together with the reason you give.
        </Alert>
      )}

      <FormSection title="Examination information" description="Dates must fall within the academic year.">
            <Select label="Examination type" required value={values.exam_type_id} onChange={set('exam_type_id')} error={errors.exam_type_id}>
              <option value="">Choose a type</option>
              {typeOptions.map((t) => (
                <option key={t.id} value={t.id}>
                  {t.name}
                  {t.status !== 'active' ? ' (inactive)' : ''}
                </option>
              ))}
            </Select>
            <Select
              label="Academic year"
              required
              value={values.academic_year_id}
              onChange={set('academic_year_id')}
              disabled={yearLocked}
              error={errors.academic_year_id}
              hint={yearLocked ? 'Fixed once classes are added or the examination is published.' : undefined}
            >
              <option value="">Choose a year</option>
              {state.years
                .filter((y) => y.status !== 'completed' || y.id === values.academic_year_id)
                .map((y) => (
                  <option key={y.id} value={y.id}>
                    {yearOptionLabel(y)}
                  </option>
                ))}
            </Select>
            <Input
              label="Examination name"
              required
              className="sm:col-span-2"
              value={values.name}
              placeholder={suggestedName || 'e.g. Half-Yearly Examination 2027'}
              onChange={set('name')}
              error={errors.name}
              maxLength={120}
              hint={!values.name && suggestedName ? `Leave empty to use “${suggestedName}”.` : 'Unique within the academic year.'}
            />
            <Input label="Start date" type="date" required value={values.start_date} min={year?.start_date} max={values.end_date || year?.end_date} onChange={set('start_date')} error={errors.start_date} />
            <Input label="End date" type="date" required value={values.end_date} min={values.start_date || year?.start_date} max={year?.end_date} onChange={set('end_date')} error={errors.end_date} />
            <TextArea label="Description" className="sm:col-span-2" rows={2} value={values.description} onChange={set('description')} error={errors.description} hint="Optional." />
            <TextArea
              label="Instructions for students"
              className="sm:col-span-2"
              rows={3}
              value={values.instructions}
              onChange={set('instructions')}
              error={errors.instructions}
              hint="Optional. Printed on the timetable (e.g. bring admit cards, arrive 15 minutes early)."
            />
      </FormSection>

      {!editing && (
        <FormSection title="Classes" description="Every section of a selected class sits the same timetable. You can also add classes later while the examination is a draft.">
          <div className="sm:col-span-2">
            <ClassPicker
              classes={state.classes}
              selected={classIds}
              onToggle={(cid) => setClassIds((cur) => (cur.includes(cid) ? cur.filter((x) => x !== cid) : [...cur, cid]))}
              error={errors.class_ids}
            />
          </div>
        </FormSection>
      )}

      {published && (
        <FormSection title="Reason for the change" description="Kept in the examination's history.">
          <TextArea label="Reason" required className="sm:col-span-2" rows={2} value={values.reason} onChange={set('reason')} error={errors.reason} />
        </FormSection>
      )}

      <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <Button variant="secondary" onClick={() => navigate(editing ? `/examinations/${id}` : '/examinations')} disabled={saving}>
          Cancel
        </Button>
        <Button type="submit" icon={Save} loading={saving}>
          {editing ? 'Save changes' : classIds.length ? 'Create and build timetable' : 'Create examination'}
        </Button>
      </div>
    </form>
  )
}
