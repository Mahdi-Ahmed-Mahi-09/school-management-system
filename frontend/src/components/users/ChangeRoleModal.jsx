import { useState } from 'react'
import { Check, ShieldCheck } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { usersApi } from '../../api/services'
import { ROLE_META, ROLES } from '../../lib/roles'
import { cn } from '../../lib/utils'
import { Button } from '../ui/Button'
import { Alert } from '../ui/Display'
import { Modal } from '../ui/Modal'

export function ChangeRoleModal({ open, onClose, user, onSaved }) {
  const [role, setRole] = useState(user?.role)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState(null)

  if (!user) return null

  const save = async () => {
    setSaving(true)
    setError(null)
    try {
      onSaved(await usersApi.changeRole(user.id, role), 'role')
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal
      open={open}
      onClose={saving ? undefined : onClose}
      dismissible={!saving}
      icon={ShieldCheck}
      title="Change role"
      description={`Choose what ${user.full_name} can access in Classora.`}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={saving}>
            Cancel
          </Button>
          <Button onClick={save} loading={saving} disabled={role === user.role}>
            Update role
          </Button>
        </>
      }
    >
      {error && (
        <Alert tone="danger" className="mb-4">
          {error}
        </Alert>
      )}
      <div className="space-y-2" role="radiogroup" aria-label="Role">
        {ROLES.map((r) => {
          const { icon: Icon, label, description, soft } = ROLE_META[r]
          const selected = role === r
          return (
            <button
              key={r}
              type="button"
              role="radio"
              aria-checked={selected}
              onClick={() => setRole(r)}
              className={cn(
                'flex w-full items-center gap-3 rounded-xl border p-3.5 text-left transition focus-visible:ring-2 focus-visible:ring-brand-500/40',
                selected ? 'border-brand-500 bg-brand-50/50 ring-1 ring-brand-500' : 'border-slate-200 hover:border-slate-300 hover:bg-slate-50',
              )}
            >
              <span className={cn('flex size-9 shrink-0 items-center justify-center rounded-lg', soft)}>
                <Icon className="size-[18px]" aria-hidden="true" />
              </span>
              <span className="min-w-0 flex-1">
                <span className="flex items-center gap-2 text-sm font-semibold text-slate-900">
                  {label}
                  {r === user.role && <span className="text-xs font-normal text-slate-400">Current</span>}
                </span>
                <span className="block text-[13px] text-slate-500">{description}</span>
              </span>
              <span
                className={cn(
                  'flex size-5 shrink-0 items-center justify-center rounded-full border transition',
                  selected ? 'border-brand-600 bg-brand-600 text-white' : 'border-slate-300',
                )}
              >
                {selected && <Check className="size-3" aria-hidden="true" />}
              </span>
            </button>
          )
        })}
      </div>
      {role === 'admin' && role !== user.role && (
        <Alert tone="warning" className="mt-4">
          Admins can manage every account in your school, including other admins.
        </Alert>
      )}
    </Modal>
  )
}
