import { useCallback, useEffect, useState } from 'react'
import { Plus, Power, PowerOff, Tags, Trash2 } from 'lucide-react'
import { getErrorMessage, getFieldErrors } from '../../api/client'
import { examTypesApi } from '../../api/services'
import { Button } from '../ui/Button'
import { Badge, ErrorState, Skeleton, StatusBadge } from '../ui/Display'
import { Input } from '../ui/Field'
import { Modal } from '../ui/Modal'
import { useToast } from '../ui/Toast'

function TypesBody({ canManage, onChanged }) {
  const toast = useToast()
  const [state, setState] = useState({ loading: true, error: null, types: [] })
  const [name, setName] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(null)
  const load = useCallback(() => {
    examTypesApi
      .list()
      .then((types) => setState({ loading: false, error: null, types }))
      .catch((err) => setState({ loading: false, error: getErrorMessage(err), types: [] }))
  }, [])
  useEffect(() => {
    load()
  }, [load])

  const run = async (key, fn, success) => {
    setBusy(key)
    try {
      await fn()
      toast.success(success)
      load()
      onChanged?.()
    } catch (err) {
      toast.error('Something went wrong', getErrorMessage(err))
      return err
    } finally {
      setBusy(null)
    }
  }
  const add = async (e) => {
    e.preventDefault()
    if (!name.trim()) {
      setError('Enter a name.')
      return
    }
    const err = await run('add', () => examTypesApi.create({ name: name.trim() }), 'Examination type added')
    if (err) setError(getFieldErrors(err).name ?? '')
    else setName('')
  }

  if (state.error) return <ErrorState message={state.error} onRetry={load} />
  return (
    <div className="space-y-4">
      {canManage && (
        <form onSubmit={add} className="flex items-start gap-2">
          <Input label="New type" className="flex-1" placeholder="e.g. Weekly Quiz" value={name} onChange={(e) => { setName(e.target.value); setError('') }} error={error} maxLength={60} />
          <Button type="submit" icon={Plus} loading={busy === 'add'} className="mt-7">
            Add
          </Button>
        </form>
      )}
      <ul className="max-h-80 divide-y divide-slate-100 overflow-y-auto rounded-lg border border-slate-200">
        {state.loading
          ? [0, 1, 2].map((i) => (
              <li key={i} className="p-3">
                <Skeleton className="h-5" />
              </li>
            ))
          : state.types.map((t) => (
              <li key={t.id} className="flex items-center gap-2 px-3 py-2">
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-sm font-medium text-slate-900">{t.name}</span>
                  <span className="text-xs text-slate-500">
                    {t.examinations} examination{t.examinations === 1 ? '' : 's'}
                  </span>
                </span>
                {t.is_system ? <Badge className="bg-slate-50 text-slate-600 ring-slate-500/15">Predefined</Badge> : <Badge className="bg-brand-50 text-brand-700 ring-brand-600/15">Custom</Badge>}
                <StatusBadge status={t.status} />
                {canManage && (
                  <>
                    <Button
                      variant="ghost"
                      size="sm"
                      icon={t.status === 'active' ? PowerOff : Power}
                      aria-label={t.status === 'active' ? `Deactivate ${t.name}` : `Activate ${t.name}`}
                      loading={busy === t.id}
                      onClick={() => run(t.id, () => examTypesApi.update(t.id, { name: t.name, description: t.description ?? '', status: t.status === 'active' ? 'inactive' : 'active' }), 'Examination type updated')}
                    />
                    {!t.is_system && t.examinations === 0 && (
                      <Button variant="ghost" size="sm" icon={Trash2} aria-label={`Delete ${t.name}`} loading={busy === `del-${t.id}`} onClick={() => run(`del-${t.id}`, () => examTypesApi.remove(t.id), 'Examination type deleted')} />
                    )}
                  </>
                )}
              </li>
            ))}
      </ul>
    </div>
  )
}

/** Predefined and custom examination types. */
export function ExamTypesModal({ open, onClose, canManage, onChanged }) {
  return (
    <Modal open={open} onClose={onClose} icon={Tags} title="Examination types" description="Predefined types follow common school practice; add your own for anything else. Inactive types can't be chosen for new examinations.">
      {open && <TypesBody canManage={canManage} onChanged={onChanged} />}
    </Modal>
  )
}
