import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { BookPlus, Search } from 'lucide-react'
import { getErrorMessage, getFieldErrors } from '../../api/client'
import { classesApi, classSubjectsApi, subjectsApi } from '../../api/services'
import { loadYearOptions, yearOptionLabel } from '../../lib/classes'
import { loadAllSubjects } from '../../lib/subjects'
import { cn } from '../../lib/utils'
import { Button } from '../ui/Button'
import { Alert, Skeleton, StatusBadge } from '../ui/Display'
import { Select } from '../ui/Field'
import { Modal } from '../ui/Modal'
import { useToast } from '../ui/Toast'
import { CompulsoryToggle, SubjectTypeBadge } from './SubjectTypeBadge'

const isCompulsoryType = (s) => s.subject_type === 'compulsory'

/**
 * Assign subjects to a class for one academic year.
 *
 * Without `subject`: pick a year and class, then any number of subjects, each
 * compulsory or optional. With `subject`: assign that one subject to a class.
 * `defaults` may pre-select `academic_year_id` / `class_id`. Subjects already
 * assigned, inactive subjects, inactive classes and completed years are shown
 * but can't be chosen; the API enforces the same rules.
 */
export function AssignSubjectsModal({ open, onClose, onSaved, defaults, subject }) {
  const toast = useToast()
  const fixed = !!subject
  const defaultYearId = defaults?.academic_year_id ?? ''
  const defaultClassId = defaults?.class_id ?? ''

  const [options, setOptions] = useState({ loading: true, error: null, years: [], classes: [], subjects: [] })
  const [yearId, setYearId] = useState('')
  const [classId, setClassId] = useState('')
  const [assigned, setAssigned] = useState({ loading: false, error: null, ids: new Set() })
  const [selection, setSelection] = useState({}) // subject id -> is_compulsory
  const [fixedCompulsory, setFixedCompulsory] = useState(true)
  const [query, setQuery] = useState('')
  const [errors, setErrors] = useState({})
  const [submitting, setSubmitting] = useState(false)
  const [refreshKey, setRefreshKey] = useState(0)

  // Load the choices whenever the dialog opens.
  useEffect(() => {
    if (!open) return
    let cancelled = false
    setOptions((o) => ({ ...o, loading: true, error: null }))
    setSelection({})
    setQuery('')
    setErrors({})
    setFixedCompulsory(fixed ? isCompulsoryType(subject) : true)
    Promise.all([loadYearOptions(), classesApi.list({ page_size: 100, sort: 'display_order' }), fixed ? Promise.resolve([]) : loadAllSubjects()])
      .then(([{ years, defaultYear }, { classes }, subjects]) => {
        if (cancelled) return
        setOptions({ loading: false, error: null, years, classes, subjects })
        const preset = years.find((y) => y.id === defaultYearId)
        setYearId(preset?.id ?? (defaultYear && defaultYear.status !== 'completed' ? defaultYear.id : ''))
        setClassId(classes.some((c) => c.id === defaultClassId) ? defaultClassId : '')
      })
      .catch((err) => !cancelled && setOptions({ loading: false, error: getErrorMessage(err), years: [], classes: [], subjects: [] }))
    return () => {
      cancelled = true
    }
  }, [open, fixed, subject, defaultYearId, defaultClassId])

  // What is already assigned: subjects of the class (or classes of the subject) in the year.
  useEffect(() => {
    if (!open || !yearId || (!fixed && !classId)) {
      setAssigned({ loading: false, error: null, ids: new Set() })
      return
    }
    let cancelled = false
    setAssigned((a) => ({ ...a, loading: true, error: null }))
    const request = fixed ? subjectsApi.classes(subject.id, { academic_year_id: yearId }) : classesApi.subjects(classId, { academic_year_id: yearId })
    request
      .then((items) => {
        if (cancelled) return
        const ids = new Set(items.map((a) => (fixed ? a.class.id : a.subject.id)))
        setAssigned({ loading: false, error: null, ids })
        if (!fixed) setSelection((sel) => Object.fromEntries(Object.entries(sel).filter(([id]) => !ids.has(id))))
      })
      .catch((err) => !cancelled && setAssigned({ loading: false, error: getErrorMessage(err), ids: new Set() }))
    return () => {
      cancelled = true
    }
  }, [open, fixed, subject, yearId, classId, refreshKey])

  const year = options.years.find((y) => y.id === yearId)
  const cls = options.classes.find((c) => c.id === classId)
  const noYears = !options.loading && !options.error && options.years.length === 0
  const noClasses = !options.loading && !options.error && options.classes.length === 0
  const noSubjects = !fixed && !options.loading && !options.error && options.subjects.length === 0

  const visibleSubjects = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return options.subjects
    return options.subjects.filter((s) => s.name.toLowerCase().includes(q) || s.code.toLowerCase().includes(q))
  }, [options.subjects, query])
  const isAvailable = (s) => s.status === 'active' && !assigned.ids.has(s.id)
  const availableVisible = visibleSubjects.filter(isAvailable)
  const selectedIds = Object.keys(selection)
  const allVisibleSelected = availableVisible.length > 0 && availableVisible.every((s) => s.id in selection)

  const toggle = (s) =>
    setSelection((sel) => {
      const next = { ...sel }
      if (s.id in next) delete next[s.id]
      else next[s.id] = isCompulsoryType(s)
      return next
    })
  const toggleAllVisible = () =>
    setSelection((sel) => {
      const next = { ...sel }
      if (allVisibleSelected) availableVisible.forEach((s) => delete next[s.id])
      else availableVisible.forEach((s) => (next[s.id] = next[s.id] ?? isCompulsoryType(s)))
      return next
    })

  const fixedAlreadyAssigned = fixed && classId && assigned.ids.has(classId)
  const ready =
    !options.loading &&
    !assigned.loading &&
    year &&
    year.status !== 'completed' &&
    cls &&
    cls.status === 'active' &&
    (fixed ? !fixedAlreadyAssigned && subject.status === 'active' : selectedIds.length > 0)

  const onSubmit = async (e) => {
    e.preventDefault()
    const next = {}
    if (!yearId) next.academic_year_id = 'Choose an academic year.'
    if (!classId) next.class_id = 'Choose a class.'
    if (!fixed && selectedIds.length === 0) next.subjects = 'Choose at least one subject.'
    setErrors(next)
    if (Object.keys(next).length) return
    const subjects = fixed
      ? [{ subject_id: subject.id, is_compulsory: fixedCompulsory }]
      : selectedIds.map((id) => ({ subject_id: id, is_compulsory: selection[id] }))
    setSubmitting(true)
    try {
      const created = await classSubjectsApi.assign({ class_id: classId, academic_year_id: yearId, subjects })
      const names = created.map((a) => a.subject.name)
      toast.success(
        created.length === 1 ? 'Subject assigned' : `${created.length} subjects assigned`,
        `${names.length > 3 ? `${names.slice(0, 3).join(', ')} and ${names.length - 3} more` : names.join(', ')} → ${cls.name} (${year.name}).`,
      )
      onSaved?.(created)
      onClose()
    } catch (err) {
      const fields = getFieldErrors(err)
      setErrors({ ...fields, form: fields.subjects || fields.class_id || fields.academic_year_id ? null : getErrorMessage(err) })
      // Refresh what is assigned in case someone else just changed it.
      if (err?.response?.data?.error?.code === 'CLASS_SUBJECT_EXISTS') setRefreshKey((k) => k + 1)
    } finally {
      setSubmitting(false)
    }
  }

  const title = fixed ? `Assign ${subject.name} to a class` : 'Assign subjects to a class'

  return (
    <Modal
      open={open}
      onClose={submitting ? undefined : onClose}
      dismissible={!submitting}
      icon={BookPlus}
      size="lg"
      title={title}
      description="Subjects are assigned to a class for one academic year. Each assignment is compulsory or optional for that class."
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={submitting}>
            Cancel
          </Button>
          <Button type="submit" form="assign-subjects-form" loading={submitting} disabled={!ready}>
            {fixed ? 'Assign subject' : selectedIds.length > 1 ? `Assign ${selectedIds.length} subjects` : 'Assign subject'}
          </Button>
        </>
      }
    >
      <form id="assign-subjects-form" onSubmit={onSubmit} noValidate className="space-y-5">
        {errors.form && (
          <Alert tone="danger" title="Couldn't assign">
            {errors.form}
          </Alert>
        )}
        {errors.subjects && (
          <Alert tone="danger" title="Check the chosen subjects">
            {errors.subjects}
          </Alert>
        )}
        {options.error && <Alert tone="danger" title="Couldn't load academic years, classes and subjects">{options.error}</Alert>}
        {noYears && (
          <Alert tone="warning" title="No academic years yet">
            Subjects are assigned per academic year. <Link to="/academic-years" className="font-semibold underline">Add an academic year</Link> first.
          </Alert>
        )}
        {noClasses && (
          <Alert tone="warning" title="No classes yet">
            <Link to="/classes" className="font-semibold underline">Add a class</Link> before assigning subjects.
          </Alert>
        )}
        {noSubjects && (
          <Alert tone="warning" title="No subjects yet">
            <Link to="/subjects" className="font-semibold underline">Add subjects</Link> to the catalogue first.
          </Alert>
        )}
        {fixed && subject.status !== 'active' && (
          <Alert tone="warning" title={`${subject.name} is inactive`}>
            Activate the subject before assigning it to classes.
          </Alert>
        )}

        {options.loading ? (
          <div className="grid gap-5 sm:grid-cols-2">
            <Skeleton className="h-16" />
            <Skeleton className="h-16" />
          </div>
        ) : (
          <div className="grid gap-5 sm:grid-cols-2">
            <Select
              label="Academic year"
              required
              value={yearId}
              error={errors.academic_year_id}
              onChange={(e) => {
                setYearId(e.target.value)
                setErrors({})
              }}
            >
              <option value="">Choose a year</option>
              {options.years.map((y) => (
                <option key={y.id} value={y.id} disabled={y.status === 'completed'}>
                  {yearOptionLabel(y)}
                </option>
              ))}
            </Select>
            <Select
              label="Class"
              required
              value={classId}
              error={errors.class_id}
              onChange={(e) => {
                setClassId(e.target.value)
                setErrors({})
              }}
            >
              <option value="">Choose a class</option>
              {options.classes.map((c) => {
                const taken = fixed && assigned.ids.has(c.id)
                return (
                  <option key={c.id} value={c.id} disabled={c.status !== 'active' || taken}>
                    {c.name}
                    {c.status !== 'active' ? ' (inactive)' : taken ? ' (already assigned)' : ''}
                  </option>
                )
              })}
            </Select>
          </div>
        )}
        {year?.status === 'completed' && (
          <Alert tone="info" title={`${year.name} is completed`}>
            Subjects can only be assigned for the current or an upcoming academic year.
          </Alert>
        )}
        {assigned.error && <Alert tone="danger" title="Couldn't load existing assignments">{assigned.error}</Alert>}

        {fixed ? (
          <div className="flex flex-col gap-3 rounded-xl border border-slate-200/80 p-4 sm:flex-row sm:items-center">
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium text-slate-900">
                {subject.name} <span className="font-mono text-[13px] text-slate-500">{subject.code}</span>
              </p>
              <p className="mt-0.5 text-[13px] text-slate-500">
                {fixedAlreadyAssigned ? `Already assigned to ${cls?.name} for ${year?.name}.` : 'Compulsory or optional for the chosen class.'}
              </p>
            </div>
            <CompulsoryToggle value={fixedCompulsory} onChange={setFixedCompulsory} disabled={!!fixedAlreadyAssigned} label={`${subject.name} type`} />
          </div>
        ) : (
          !options.loading &&
          !noSubjects && (
            <fieldset className="space-y-3">
              <div className="flex flex-wrap items-center gap-3">
                <legend className="flex-1 text-sm font-medium text-slate-700">
                  Subjects
                  {cls && year && (
                    <span className="ml-1.5 font-normal text-slate-500">
                      · {assigned.ids.size} already assigned to {cls.name}
                    </span>
                  )}
                </legend>
                {classId && yearId && availableVisible.length > 0 && (
                  <Button variant="ghost" size="sm" onClick={toggleAllVisible}>
                    {allVisibleSelected ? 'Clear selection' : 'Select all available'}
                  </Button>
                )}
              </div>
              {options.subjects.length > 8 && (
                <div className="relative">
                  <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
                  <input
                    type="search"
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                    placeholder="Filter subjects by name or code"
                    aria-label="Filter subjects"
                    className="w-full rounded-lg border border-slate-300 bg-white py-2 pr-3 pl-9 text-sm shadow-sm placeholder:text-slate-400 focus:border-brand-500 focus:ring-4 focus:ring-brand-500/15 focus:outline-none"
                  />
                </div>
              )}
              {!classId || !yearId ? (
                <p className="rounded-xl border border-dashed border-slate-200 px-4 py-6 text-center text-sm text-slate-500">
                  Choose an academic year and a class to see which subjects are available.
                </p>
              ) : assigned.loading ? (
                <div className="space-y-2">
                  {[0, 1, 2].map((i) => (
                    <Skeleton key={i} className="h-12" />
                  ))}
                </div>
              ) : (
                <>
                  {availableVisible.length === 0 && !query && (
                    <Alert tone="info" title="No subjects available">
                      Every active subject is already assigned to {cls?.name} for {year?.name}.{' '}
                      <Link to="/subjects" className="font-semibold underline">Add another subject</Link> to assign more.
                    </Alert>
                  )}
                  <ul className="max-h-[22rem] divide-y divide-slate-100 overflow-y-auto rounded-xl border border-slate-200/80" aria-label="Subjects">
                    {visibleSubjects.length === 0 && <li className="px-4 py-6 text-center text-sm text-slate-500">No subjects match “{query}”.</li>}
                    {visibleSubjects.map((s) => {
                      const taken = assigned.ids.has(s.id)
                      const inactive = s.status !== 'active'
                      const selected = s.id in selection
                      const disabled = taken || inactive
                      return (
                        <li key={s.id} className={cn('flex flex-wrap items-center gap-3 px-4 py-2.5 sm:flex-nowrap', selected && 'bg-brand-50/40', disabled && 'bg-slate-50/60')}>
                          <label className={cn('flex min-w-0 flex-1 items-center gap-3', disabled ? 'cursor-not-allowed' : 'cursor-pointer')}>
                            <input
                              type="checkbox"
                              checked={selected}
                              disabled={disabled}
                              onChange={() => toggle(s)}
                              className="size-4 shrink-0 rounded border-slate-300 text-brand-600 accent-brand-600 focus-visible:ring-2 focus-visible:ring-brand-500/40 disabled:opacity-50"
                            />
                            <span className="min-w-0 flex-1">
                              <span className={cn('block truncate text-sm font-medium', disabled ? 'text-slate-400' : 'text-slate-900')}>{s.name}</span>
                              <span className="block truncate font-mono text-xs text-slate-500">{s.code}</span>
                            </span>
                          </label>
                          {taken ? (
                            <span className="text-xs font-medium text-slate-500">Already assigned</span>
                          ) : inactive ? (
                            <StatusBadge status="inactive" />
                          ) : selected ? (
                            <CompulsoryToggle
                              size="sm"
                              value={selection[s.id]}
                              label={`${s.name} type`}
                              onChange={(v) => setSelection((sel) => ({ ...sel, [s.id]: v }))}
                            />
                          ) : (
                            <SubjectTypeBadge type={s.subject_type} />
                          )}
                        </li>
                      )
                    })}
                  </ul>
                </>
              )}
            </fieldset>
          )
        )}
      </form>
    </Modal>
  )
}
