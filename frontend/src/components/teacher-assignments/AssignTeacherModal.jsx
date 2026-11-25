import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { BookUser, UserRoundCheck } from 'lucide-react'
import { getErrorMessage, getFieldErrors } from '../../api/client'
import { classesApi, sectionsApi, teacherAssignmentsApi } from '../../api/services'
import { loadYearOptions, yearOptionLabel } from '../../lib/classes'
import { loadClasses, loadSections } from '../../lib/enrollments'
import { loadActiveTeachers, teacherLabel } from '../../lib/teacherAssignments'
import { Button } from '../ui/Button'
import { Alert, Skeleton } from '../ui/Display'
import { Checkbox, Select } from '../ui/Field'
import { Modal } from '../ui/Modal'
import { TextArea } from '../ui/Sections'
import { useToast } from '../ui/Toast'

/**
 * Assign a subject teacher (`type="subject_teacher"`) or class teacher
 * (`type="class_teacher"`) to one section. Every list comes from the API:
 * sections of the chosen class and year, subjects the class takes that year,
 * active teachers. If the slot already has a teacher, replacing them needs
 * explicit confirmation (the old assignment is kept as history).
 */
export function AssignTeacherModal({ open, onClose, onSaved, type = 'subject_teacher', defaults }) {
  const toast = useToast()
  const isClassTeacher = type === 'class_teacher'
  const [options, setOptions] = useState({ loading: true, error: null, years: [], classes: [], teachers: [] })
  const [form, setForm] = useState({ yearId: '', classId: '', sectionId: '', subjectId: '', teacherId: '', remarks: '', replace: false })
  const [sections, setSections] = useState({ loading: false, items: [] })
  const [subjects, setSubjects] = useState({ loading: false, items: [] })
  const [slot, setSlot] = useState({ loading: false, data: null })
  const [errors, setErrors] = useState({})
  const [submitting, setSubmitting] = useState(false)
  const dYear = defaults?.academic_year_id ?? ''
  const dClass = defaults?.class_id ?? ''
  const dSection = defaults?.section_id ?? ''
  const dSubject = defaults?.subject_id ?? ''

  useEffect(() => {
    if (!open) return
    let cancelled = false
    setErrors({})
    setOptions((o) => ({ ...o, loading: true, error: null }))
    Promise.all([loadYearOptions(), loadClasses(), loadActiveTeachers()])
      .then(([{ years, defaultYear }, classes, teachers]) => {
        if (cancelled) return
        setOptions({ loading: false, error: null, years, classes, teachers })
        const year = years.find((y) => y.id === dYear) ?? (defaultYear?.status !== 'completed' ? defaultYear : null)
        setForm({ yearId: year?.id ?? '', classId: dClass, sectionId: dSection, subjectId: dSubject, teacherId: '', remarks: '', replace: false })
      })
      .catch((err) => !cancelled && setOptions({ loading: false, error: getErrorMessage(err), years: [], classes: [], teachers: [] }))
    return () => {
      cancelled = true
    }
  }, [open, dYear, dClass, dSection, dSubject])

  // Sections of the class in the year, and the subjects the class takes that year.
  useEffect(() => {
    if (!open || !form.yearId || !form.classId) {
      setSections({ loading: false, items: [] })
      setSubjects({ loading: false, items: [] })
      return
    }
    let cancelled = false
    setSections({ loading: true, items: [] })
    loadSections(form.yearId, form.classId)
      .then((items) => !cancelled && setSections({ loading: false, items }))
      .catch(() => !cancelled && setSections({ loading: false, items: [] }))
    if (!isClassTeacher) {
      setSubjects({ loading: true, items: [] })
      classesApi
        .subjects(form.classId, { academic_year_id: form.yearId })
        .then((items) => !cancelled && setSubjects({ loading: false, items }))
        .catch(() => !cancelled && setSubjects({ loading: false, items: [] }))
    }
    return () => {
      cancelled = true
    }
  }, [open, form.yearId, form.classId, isClassTeacher])

  // Who holds the slot now.
  useEffect(() => {
    if (!open || !form.sectionId) {
      setSlot({ loading: false, data: null })
      return
    }
    let cancelled = false
    setSlot({ loading: true, data: null })
    sectionsApi
      .teachers(form.sectionId)
      .then((data) => !cancelled && setSlot({ loading: false, data }))
      .catch(() => !cancelled && setSlot({ loading: false, data: null }))
    return () => {
      cancelled = true
    }
  }, [open, form.sectionId])

  const set = (changes) => {
    setForm((f) => ({ ...f, ...changes, replace: false }))
    setErrors({})
  }
  const year = options.years.find((y) => y.id === form.yearId)
  const cls = options.classes.find((c) => c.id === form.classId)
  const current = isClassTeacher
    ? slot.data?.class_teacher
    : slot.data?.subjects.find((s) => s.subject.id === form.subjectId)?.assignment
  const sameTeacher = current && current.teacher.id === form.teacherId
  const noSubjects = !isClassTeacher && form.classId && !subjects.loading && subjects.items.length === 0
  const noSections = form.classId && !sections.loading && sections.items.length === 0
  const ready =
    form.yearId && form.classId && form.sectionId && form.teacherId && (isClassTeacher || form.subjectId) && !sameTeacher && (!current || form.replace)

  const submit = async (e) => {
    e.preventDefault()
    setSubmitting(true)
    setErrors({})
    try {
      const saved = await teacherAssignmentsApi.create({
        assignment_type: type,
        academic_year_id: form.yearId,
        class_id: form.classId,
        section_id: form.sectionId,
        subject_id: isClassTeacher ? '' : form.subjectId,
        teacher_id: form.teacherId,
        replace: form.replace,
        remarks: form.remarks.trim(),
      })
      const what = isClassTeacher ? 'class teacher' : saved.subject.name
      toast.success(
        current ? 'Teacher replaced' : 'Teacher assigned',
        `${saved.teacher.full_name} → ${what}, ${saved.class.name} – ${saved.section.name} (${saved.academic_year.name}).`,
      )
      onSaved?.(saved)
      onClose()
    } catch (err) {
      const fields = getFieldErrors(err)
      setErrors(Object.keys(fields).length ? fields : { form: getErrorMessage(err) })
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Modal
      open={open}
      onClose={submitting ? undefined : onClose}
      dismissible={!submitting}
      icon={isClassTeacher ? UserRoundCheck : BookUser}
      size="lg"
      title={isClassTeacher ? 'Assign a class teacher' : 'Assign a subject teacher'}
      description={
        isClassTeacher
          ? 'The class teacher looks after one section for an academic year. Each section has one.'
          : 'A subject teacher teaches one subject in one section for an academic year.'
      }
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={submitting}>
            Cancel
          </Button>
          <Button type="submit" form="assign-teacher-form" loading={submitting} disabled={!ready}>
            {current ? 'Replace teacher' : 'Assign teacher'}
          </Button>
        </>
      }
    >
      <form id="assign-teacher-form" onSubmit={submit} noValidate className="space-y-5">
        {errors.form && (
          <Alert tone="danger" title="Couldn't assign the teacher">
            {errors.form}
          </Alert>
        )}
        {options.error && <Alert tone="danger" title="Couldn't load the options">{options.error}</Alert>}
        {options.loading ? (
          <div className="grid gap-5 sm:grid-cols-2">
            {[0, 1, 2, 3].map((i) => (
              <Skeleton key={i} className="h-16" />
            ))}
          </div>
        ) : (
          <>
            <div className="grid gap-5 sm:grid-cols-2">
              <Select label="Academic year" required value={form.yearId} error={errors.academic_year_id} onChange={(e) => set({ yearId: e.target.value, sectionId: '', subjectId: '' })}>
                <option value="">Choose a year</option>
                {options.years.map((y) => (
                  <option key={y.id} value={y.id} disabled={y.status === 'completed'}>
                    {yearOptionLabel(y)}
                  </option>
                ))}
              </Select>
              <Select label="Class" required value={form.classId} error={errors.class_id} onChange={(e) => set({ classId: e.target.value, sectionId: '', subjectId: '' })}>
                <option value="">Choose a class</option>
                {options.classes.map((c) => (
                  <option key={c.id} value={c.id} disabled={c.status !== 'active'}>
                    {c.name}
                    {c.status !== 'active' ? ' (inactive)' : ''}
                  </option>
                ))}
              </Select>
              <Select
                label="Section"
                required
                value={form.sectionId}
                disabled={!form.classId || sections.loading}
                error={errors.section_id}
                onChange={(e) => set({ sectionId: e.target.value })}
              >
                <option value="">{sections.loading ? 'Loading…' : form.classId ? 'Choose a section' : 'Choose a class first'}</option>
                {sections.items.map((s) => (
                  <option key={s.id} value={s.id} disabled={s.status !== 'active'}>
                    Section {s.name}
                    {s.status !== 'active' ? ' (inactive)' : ''}
                  </option>
                ))}
              </Select>
              {!isClassTeacher && (
                <Select
                  label="Subject"
                  required
                  value={form.subjectId}
                  disabled={!form.classId || subjects.loading}
                  error={errors.subject_id}
                  onChange={(e) => set({ subjectId: e.target.value })}
                >
                  <option value="">{subjects.loading ? 'Loading…' : form.classId ? 'Choose a subject' : 'Choose a class first'}</option>
                  {subjects.items.map((a) => (
                    <option key={a.subject.id} value={a.subject.id} disabled={a.subject.status !== 'active'}>
                      {a.subject.name} ({a.subject.code}){a.subject.status !== 'active' ? ' (inactive)' : ''}
                    </option>
                  ))}
                </Select>
              )}
            </div>
            {noSections && (
              <Alert tone="warning" title={`${cls?.name} has no sections in ${year?.name}`}>
                <Link to={`/sections?year=${form.yearId}&class=${form.classId}`} className="font-semibold underline">Add sections</Link> first.
              </Alert>
            )}
            {noSubjects && (
              <Alert tone="warning" title={`${cls?.name} has no subjects in ${year?.name}`}>
                <Link to={`/class-subjects?year=${form.yearId}&class=${form.classId}`} className="font-semibold underline">Assign subjects to the class</Link> first.
              </Alert>
            )}
            <Select label="Teacher" required value={form.teacherId} error={errors.teacher_id} onChange={(e) => set({ teacherId: e.target.value })}>
              <option value="">{options.teachers.length ? 'Choose a teacher' : 'No active teachers'}</option>
              {options.teachers.map((t) => (
                <option key={t.id} value={t.id}>
                  {teacherLabel(t)} · {t.designation}
                </option>
              ))}
            </Select>

            {form.sectionId && (isClassTeacher || form.subjectId) && !slot.loading && (
              <div className="rounded-xl border border-slate-200/80 bg-slate-50/60 p-4 text-sm">
                {current ? (
                  <>
                    <p className="text-slate-700">
                      <span className="font-medium text-slate-900">{current.teacher.full_name}</span> is the current{' '}
                      {isClassTeacher ? 'class teacher' : `${current.subject?.name} teacher`} of this section.
                    </p>
                    {sameTeacher ? (
                      <p className="mt-1 text-amber-700">Choose a different teacher — this one is already assigned.</p>
                    ) : (
                      form.teacherId && (
                        <Checkbox
                          className="mt-3"
                          label={`Replace ${current.teacher.full_name}`}
                          description="Their assignment is ended and kept in the history."
                          checked={form.replace}
                          onChange={(e) => setForm((f) => ({ ...f, replace: e.target.checked }))}
                        />
                      )
                    )}
                  </>
                ) : (
                  <p className="text-slate-600">
                    {isClassTeacher ? 'This section has no class teacher yet.' : 'No teacher is assigned to this subject in this section yet.'}
                  </p>
                )}
              </div>
            )}
            <TextArea label="Remarks" rows={2} maxLength={500} placeholder="Optional" value={form.remarks} onChange={(e) => setForm((f) => ({ ...f, remarks: e.target.value }))} />
          </>
        )}
      </form>
    </Modal>
  )
}
