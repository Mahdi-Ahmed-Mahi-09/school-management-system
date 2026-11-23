import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { LayoutGrid, PencilLine } from 'lucide-react'
import { applyFieldErrors, getErrorMessage } from '../../api/client'
import { classesApi } from '../../api/services'
import { classRules, toIntOrNull } from '../../lib/classes'
import { Button } from '../ui/Button'
import { Alert } from '../ui/Display'
import { Input, Select } from '../ui/Field'
import { Modal } from '../ui/Modal'
import { TextArea } from '../ui/Sections'
import { useToast } from '../ui/Toast'

const FIELDS = ['name', 'code', 'display_order', 'description']
const empty = { name: '', code: '', display_order: '', description: '', status: 'active' }

/** Create a class (no `classItem`) or edit one's details. */
export function ClassFormModal({ open, onClose, classItem, onSaved }) {
  const toast = useToast()
  const isEdit = !!classItem
  const {
    register,
    handleSubmit,
    reset,
    setError,
    formState: { errors, isSubmitting },
  } = useForm({ defaultValues: empty })
  const formError = errors.root?.server?.message

  useEffect(() => {
    if (!open) return
    reset(
      isEdit
        ? { ...empty, name: classItem.name, code: classItem.code ?? '', display_order: String(classItem.display_order), description: classItem.description ?? '' }
        : empty,
    )
  }, [open, isEdit, classItem, reset])

  const onSubmit = async (values) => {
    const payload = {
      name: values.name.trim(),
      code: values.code.trim(),
      display_order: toIntOrNull(values.display_order),
      description: values.description.trim(),
    }
    try {
      const saved = isEdit ? await classesApi.update(classItem.id, payload) : await classesApi.create({ ...payload, status: values.status })
      toast.success(isEdit ? 'Class updated' : 'Class added', `${saved.name} was saved.`)
      onSaved?.(saved)
      onClose()
    } catch (err) {
      if (!applyFieldErrors(err, setError, [...FIELDS, 'status'])) setError('root.server', { message: getErrorMessage(err) })
    }
  }

  return (
    <Modal
      open={open}
      onClose={isSubmitting ? undefined : onClose}
      dismissible={!isSubmitting}
      icon={isEdit ? PencilLine : LayoutGrid}
      title={isEdit ? `Edit ${classItem.name}` : 'Add a class'}
      description="Classes are reusable levels such as Play, KG or Class 6. Their sections are set up per academic year."
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={isSubmitting}>
            Cancel
          </Button>
          <Button type="submit" form="class-form" loading={isSubmitting}>
            {isEdit ? 'Save changes' : 'Add class'}
          </Button>
        </>
      }
    >
      <form id="class-form" onSubmit={handleSubmit(onSubmit)} noValidate className="space-y-5">
        {formError && (
          <Alert tone="danger" title="Couldn't save the class">
            {formError}
          </Alert>
        )}
        <Input label="Class name" required placeholder="e.g. Class 6" maxLength={60} error={errors.name?.message} {...register('name', classRules.name)} />
        <div className="grid gap-5 sm:grid-cols-2">
          <Input
            label="Class code"
            placeholder="e.g. C6"
            maxLength={20}
            inputClassName="uppercase"
            hint="Optional, unique."
            error={errors.code?.message}
            {...register('code', classRules.code)}
          />
          <Input
            label="Display order"
            type="number"
            min={1}
            max={9999}
            placeholder={isEdit ? undefined : 'After the last class'}
            hint="Lower numbers are listed first."
            error={errors.display_order?.message}
            {...register('display_order', classRules.display_order)}
          />
        </div>
        {!isEdit && (
          <Select label="Status" {...register('status')}>
            <option value="active">Active</option>
            <option value="inactive">Inactive</option>
          </Select>
        )}
        <TextArea label="Description" rows={3} maxLength={500} placeholder="Optional notes about this class" error={errors.description?.message} {...register('description', classRules.description)} />
      </form>
    </Modal>
  )
}
