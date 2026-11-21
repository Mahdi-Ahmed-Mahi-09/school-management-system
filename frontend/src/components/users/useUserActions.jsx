import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Eye, KeyRound, PencilLine, ShieldCheck, UserCheck, UserX } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { usersApi } from '../../api/services'
import { useAuth } from '../../hooks/useAuth'
import { PERMISSIONS } from '../../lib/roles'
import { ConfirmDialog } from '../ui/Modal'
import { useToast } from '../ui/Toast'
import { ChangeRoleModal } from './ChangeRoleModal'
import { ResetPasswordModal } from './ResetPasswordModal'
import { UserFormModal } from './UserFormModal'

const SUCCESS = {
  created: (u) => ['User created', `${u.full_name} can now sign in.`],
  updated: (u) => ['Changes saved', `${u.full_name}'s details were updated.`],
  role: (u) => ['Role updated', `${u.full_name} is now ${u.role === 'admin' ? 'an' : 'a'} ${u.role}.`],
  password: (u) => ['Password reset', `${u.full_name} has been signed out of all devices.`],
  active: (u) => ['Account activated', `${u.full_name} can sign in again.`],
  inactive: (u) => ['Account deactivated', `${u.full_name} can no longer sign in.`],
}

/**
 * Shared admin actions on a user (edit, role, password, status) used by both
 * the users table and the user detail page, so behaviour stays identical.
 */
export function useUserActions({ onChanged }) {
  const { user: me, can, updateUser } = useAuth()
  const toast = useToast()
  const navigate = useNavigate()
  const [dialog, setDialog] = useState({ type: null, user: null })
  const [statusSaving, setStatusSaving] = useState(false)

  const close = () => setDialog((d) => ({ ...d, type: null }))
  const open = (type, user = null) => setDialog({ type, user })

  const handleSaved = (saved, kind) => {
    close()
    const [title, description] = SUCCESS[kind](saved)
    toast.success(title, description)
    if (saved.id === me.id) updateUser(saved)
    onChanged?.(saved, kind)
  }

  const toggleStatus = async () => {
    const target = dialog.user
    const next = target.status === 'active' ? 'inactive' : 'active'
    setStatusSaving(true)
    try {
      handleSaved(await usersApi.changeStatus(target.id, next), next)
    } catch (err) {
      close()
      toast.error('Could not change status', getErrorMessage(err))
    } finally {
      setStatusSaving(false)
    }
  }

  const menuItems = (user, { includeView = true } = {}) => {
    const isSelf = user.id === me.id
    return [
      includeView && { label: 'View details', icon: Eye, onSelect: () => navigate(`/users/${user.id}`) },
      can(PERMISSIONS.USERS_UPDATE) && { label: 'Edit details', icon: PencilLine, onSelect: () => open('edit', user) },
      can(PERMISSIONS.USERS_MANAGE_ROLES) && {
        label: 'Change role',
        icon: ShieldCheck,
        disabled: isSelf,
        onSelect: () => open('role', user),
      },
      can(PERMISSIONS.USERS_RESET_PASSWORD) && {
        label: 'Reset password',
        icon: KeyRound,
        disabled: isSelf,
        onSelect: () => open('password', user),
      },
      can(PERMISSIONS.USERS_MANAGE_STATUS) && { separator: true },
      can(PERMISSIONS.USERS_MANAGE_STATUS) &&
        (user.status === 'active'
          ? { label: 'Deactivate account', icon: UserX, danger: true, disabled: isSelf, onSelect: () => open('status', user) }
          : { label: 'Activate account', icon: UserCheck, disabled: isSelf, onSelect: () => open('status', user) }),
    ].filter(Boolean)
  }

  const target = dialog.user
  const deactivating = target?.status === 'active'

  const modals = (
    <>
      <UserFormModal open={dialog.type === 'create' || dialog.type === 'edit'} onClose={close} user={dialog.type === 'edit' ? target : null} onSaved={handleSaved} />
      <ChangeRoleModal key={`${target?.id}-${dialog.type === 'role'}`} open={dialog.type === 'role'} onClose={close} user={target} onSaved={handleSaved} />
      <ResetPasswordModal open={dialog.type === 'password'} onClose={close} user={target} onSaved={handleSaved} />
      <ConfirmDialog
        open={dialog.type === 'status'}
        onClose={close}
        onConfirm={toggleStatus}
        loading={statusSaving}
        tone={deactivating ? 'danger' : 'success'}
        icon={deactivating ? UserX : UserCheck}
        title={deactivating ? `Deactivate ${target?.full_name}?` : `Activate ${target?.full_name}?`}
        description={
          deactivating
            ? 'They will be signed out immediately and won’t be able to sign in until the account is activated again.'
            : 'They will be able to sign in with their existing password.'
        }
        confirmLabel={deactivating ? 'Deactivate' : 'Activate'}
      />
    </>
  )

  return { menuItems, modals, openCreate: () => open('create'), openEdit: (u) => open('edit', u), open }
}
