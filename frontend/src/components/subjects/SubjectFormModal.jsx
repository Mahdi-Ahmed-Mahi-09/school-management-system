import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { BookOpen, PencilLine } from 'lucide-react'
import { applyFieldErrors, getErrorMessage } from '../../api/client'
import { subjectsApi } from '../../api/services'
import { toIntOrNull } from '../../lib/classes'
import { subjectRules } from '../../lib/subjects'
import { Button } from '../ui/Button'
import { Alert } from '../ui/Display'
import { Input, Select } from '../ui/Field'
import { Modal } from '../ui/Modal'
import { TextArea } from '../ui/Sections'
import { useToast } from '../ui/Toast'

const FIELDS = ['name', 'code', 'description', 'subject_type', 'full_marks', 'pass_marks', 'status']
const empty = { name: '', code: '', description: '', subject_type: 'compulsory', full_marks: '', pass_marks: '', status: 'active' }

/** Create a subject (no `subject`) or edit one. */
export function SubjectFormModal({ open, onClose, subject, onSaved }) {
  const toast = useToast()
  const isEdit = !!subject
  const {
    register,
    handleSubmit,
    reset,
    setError,
    trigger,
    formState: { errors, isSubmitting, isSubmitted },
  } = useForm({ defaultValues: empty })
  const formError = errors.root?.server?.message

  useEffect(() => {
    if (!open) return
    reset(
      isEdit
        ? {
            name: subject.name,
            code: subject.code,
            description: subject.description ?? '',
            subject_type: subject.subject_type,
            full_marks: subject.full_marks == null ? '' : String(subject.full_marks),
            pass_marks: subject.pass_marks == null ? '' : String(subject.pass_marks),
            status: subject.status,
          }
        : empty,
    )
  }, [open, isEdit, subject, reset])

  const onSubmit = async (values) => {
    const payload = {
      name: values.name.trim(),
      code: values.code.trim(),
      description: values.description.trim(),
      subject_type: values.subject_type,
      full_marks: toIntOrNull(values.full_marks),
      pass_marks: toIntOrNull(values.pass_marks),
      status: values.status,
    }
    try {
      const saved = isEdit ? await subjectsApi.update(subject.id, payload) : await subjectsApi.create(payload)
      toast.success(isEdit ? 'Subject updated' : 'Subject added', `${saved.name} (${saved.code}) was saved.`)
      onSaved?.(saved)
      onClose()
    } catch (err) {
      if (!applyFieldErrors(err, setError, FIELDS)) setError('root.server', { message: getErrorMessage(err) })
    }
  }

  // Pass marks are checked against full marks, so re-check them together.
  const fullMarks = register('full_marks', {
    ...subjectRules.full_marks,
    onChange: () => isSubmitted && trigger('pass_marks'),
  })

  return (
    <Modal
      open={open}
      onClose={isSubmitting ? undefined : onClose}
      dismissible={!isSubmitting}
      icon={isEdit ? PencilLine : BookOpen}
      size="lg"
      title={isEdit ? `Edit ${subject.name}` : 'Add a subject'}
      description="Subjects are shared by every class. Assign them to classes for each academic year."
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={isSubmitting}>
            Cancel
          </Button>
          <Button type="submit" form="subject-form" loading={isSubmitting}>
            {isEdit ? 'Save changes' : 'Add subject'}
          </Button>
        </>
      }
    >
      <form id="subject-form" onSubmit={handleSubmit(onSubmit)} noValidate className="space-y-5">
        {formError && (
          <Alert tone="danger" title="Couldn't save the subject">
            {formError}
          </Alert>
        )}
        <div className="grid gap-5 sm:grid-cols-[minmax(0,1fr)_12rem]">
          <Input label="Subject name" required placeholder="e.g. Mathematics" maxLength={100} error={errors.name?.message} {...register('name', subjectRules.name)} />
          <Input
            label="Subject code"
            required
            placeholder="e.g. MATH"
            maxLength={20}
            inputClassName="uppercase"
            hint="Unique; letters, numbers or dashes."
            error={errors.code?.message}
            {...register('code', subjectRules.code)}
          />
        </div>
        <div className="grid gap-5 sm:grid-cols-2">
          <Select
            label="Subject type"
            hint="Default when assigning it to a class."
            error={errors.subject_type?.message}
            {...register('subject_type')}
          >
            <option value="compulsory">Compulsory</option>
            <option value="optional">Optional</option>
          </Select>
          <Select label="Status" error={errors.status?.message} {...register('status')}>
            <option value="active">Active</option>
            <option value="inactive">Inactive</option>
          </Select>
          <Input
            label="Full marks"
            type="number"
            min={1}
            max={1000}
            placeholder="Not set"
            hint="Optional, 1–1000."
            error={errors.full_marks?.message}
            {...fullMarks}
          />
          <Input
            label="Pass marks"
            type="number"
            min={0}
            max={1000}
            placeholder="Not set"
            hint="Optional, up to the full marks."
            error={errors.pass_marks?.message}
            {...register('pass_marks', subjectRules.pass_marks)}
          />
        </div>
        <TextArea
          label="Description"
          rows={3}
          maxLength={500}
          placeholder="Optional notes, e.g. syllabus or group"
          error={errors.description?.message}
          {...register('description', subjectRules.description)}
        />
      </form>
    </Modal>
  )
}
