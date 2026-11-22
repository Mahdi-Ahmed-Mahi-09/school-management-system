import { useEffect, useRef, useState } from 'react'
import { Camera, Trash2 } from 'lucide-react'
import { prepareProfilePhoto } from '../../lib/image'
import { PersonAvatar } from './PersonAvatar'

/**
 * Photo selector for person forms (students, teachers). The chosen image is cropped and
 * resized in the browser; the parent uploads it after the record is saved.
 *
 * value: { blob: Blob | null, remove: boolean }
 */
export function PhotoPicker({ record, fullName, value, onChange, disabled, label = 'Photo' }) {
  const inputRef = useRef(null)
  const [preview, setPreview] = useState(null)
  const [error, setError] = useState(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!value.blob) {
      setPreview(null)
      return
    }
    const url = URL.createObjectURL(value.blob)
    setPreview(url)
    return () => URL.revokeObjectURL(url)
  }, [value.blob])

  const choose = async (e) => {
    const file = e.target.files?.[0]
    e.target.value = ''
    if (!file) return
    setError(null)
    setBusy(true)
    try {
      onChange({ blob: await prepareProfilePhoto(file), remove: false })
    } catch (err) {
      setError(err.message)
    } finally {
      setBusy(false)
    }
  }

  const hasExisting = !!record?.photo_url && !value.remove
  const shown = preview
    ? { ...record, full_name: fullName, photo_url: preview }
    : { ...record, full_name: fullName || record?.full_name, photo_url: hasExisting ? record.photo_url : null }

  return (
    <div className="flex items-center gap-5">
      <PersonAvatar person={shown} size="xl" />
      <div className="min-w-0">
        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            disabled={disabled || busy}
            onClick={() => inputRef.current?.click()}
            className="inline-flex h-9 items-center gap-2 rounded-lg bg-white px-3 text-[13px] font-medium text-slate-700 shadow-sm ring-1 ring-slate-200 ring-inset transition hover:bg-slate-50 disabled:opacity-50"
          >
            <Camera className="size-4 text-slate-500" aria-hidden="true" />
            {busy ? 'Processing…' : preview || hasExisting ? 'Change photo' : 'Upload photo'}
          </button>
          {(preview || hasExisting) && (
            <button
              type="button"
              disabled={disabled || busy}
              onClick={() => onChange({ blob: null, remove: !!record?.photo_url })}
              className="inline-flex h-9 items-center gap-2 rounded-lg px-3 text-[13px] font-medium text-rose-600 transition hover:bg-rose-50 disabled:opacity-50"
            >
              <Trash2 className="size-4" aria-hidden="true" />
              Remove
            </button>
          )}
        </div>
        <p className="mt-2 text-xs text-slate-500">Optional. JPEG, PNG or WebP — cropped to a square.</p>
        {error && (
          <p className="mt-1 text-xs text-rose-600" role="alert">
            {error}
          </p>
        )}
        <input ref={inputRef} type="file" accept="image/jpeg,image/png,image/webp" className="sr-only" onChange={choose} tabIndex={-1} aria-label={label} />
      </div>
    </div>
  )
}
