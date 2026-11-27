import { useEffect, useState } from 'react'
import { AlertTriangle, Ban, CheckCircle2, CircleAlert, Send } from 'lucide-react'
import { getErrorMessage, getFieldErrors } from '../../api/client'
import { examinationsApi } from '../../api/services'
import { Button } from '../ui/Button'
import { Alert, Skeleton } from '../ui/Display'
import { Modal } from '../ui/Modal'
import { TextArea } from '../ui/Sections'
import { useToast } from '../ui/Toast'

function IssueList({ items, tone }) {
  const Icon = tone === 'danger' ? CircleAlert : AlertTriangle
  return (
    <ul className="space-y-1.5">
      {items.map((i, n) => (
        <li key={`${i.code}-${n}`} className="flex items-start gap-2 text-sm">
          <Icon className={tone === 'danger' ? 'mt-0.5 size-4 shrink-0 text-rose-600' : 'mt-0.5 size-4 shrink-0 text-amber-600'} aria-hidden="true" />
          <span className="text-slate-700">{i.message}</span>
        </li>
      ))}
    </ul>
  )
}

export function PublishDialog({ exam, onClose, onDone }) {
  const toast = useToast()
  const [report, setReport] = useState(null)
  const [error, setError] = useState(null)
  const [busy, setBusy] = useState(false)
  useEffect(() => {
    let cancelled = false
    examinationsApi
      .publishCheck(exam.id)
      .then((r) => !cancelled && setReport(r))
      .catch((err) => !cancelled && setError(getErrorMessage(err)))
    return () => {
      cancelled = true
    }
  }, [exam.id])
  const publish = async () => {
    setBusy(true)
    try {
      const x = await examinationsApi.publish(exam.id)
      toast.success('Timetable published', `${x.name} is now visible to the teachers of its classes.`)
      onDone(x)
      onClose()
    } catch (err) {
      toast.error("Couldn't publish", getErrorMessage(err))
      examinationsApi.publishCheck(exam.id).then(setReport).catch(() => {})
    } finally {
      setBusy(false)
    }
  }
  return (
    <Modal open onClose={busy ? undefined : onClose} dismissible={!busy} icon={Send} title={`Publish ${exam.name}?`} description="Publishing makes the timetable official. After that, changes need a reason and papers can't be removed.">
      <div className="space-y-4">
        {error ? (
          <Alert tone="danger">{error}</Alert>
        ) : !report ? (
          <div className="space-y-2">
            <Skeleton className="h-6" />
            <Skeleton className="h-6" />
          </div>
        ) : (
          <>
            <p className="text-sm text-slate-600">
              {report.classes} class{report.classes === 1 ? '' : 'es'} · {report.papers} paper{report.papers === 1 ? '' : 's'}
            </p>
            {report.ready ? (
              <Alert tone="success" icon={CheckCircle2} title="Ready to publish">
                Every class has a valid schedule with no conflicts.
              </Alert>
            ) : (
              <div className="rounded-lg border border-rose-200 bg-rose-50/60 p-3">
                <p className="mb-2 text-sm font-semibold text-rose-900">Fix these before publishing</p>
                <IssueList items={report.issues} tone="danger" />
              </div>
            )}
            {report.warnings.length > 0 && (
              <div className="rounded-lg border border-amber-200 bg-amber-50/60 p-3">
                <p className="mb-2 text-sm font-semibold text-amber-900">Worth checking</p>
                <IssueList items={report.warnings} tone="warning" />
              </div>
            )}
          </>
        )}
        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            Cancel
          </Button>
          <Button icon={Send} onClick={publish} loading={busy} disabled={!report?.ready}>
            Publish timetable
          </Button>
        </div>
      </div>
    </Modal>
  )
}

export function CancelDialog({ exam, onClose, onDone }) {
  const toast = useToast()
  const [reason, setReason] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const submit = async () => {
    if (!reason.trim()) {
      setError('Give a reason for cancelling.')
      return
    }
    setBusy(true)
    try {
      const x = await examinationsApi.cancel(exam.id, reason.trim())
      toast.success('Examination cancelled', `${x.name} is kept as a cancelled record.`)
      onDone(x)
      onClose()
    } catch (err) {
      setError(getFieldErrors(err).reason ?? '')
      toast.error("Couldn't cancel", getErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Modal open onClose={busy ? undefined : onClose} dismissible={!busy} icon={Ban} title={`Cancel ${exam.name}?`} description="The examination and its timetable stay in the records, marked as cancelled. This can't be undone.">
      <div className="space-y-4">
        <TextArea label="Reason" required rows={3} value={reason} onChange={(e) => { setReason(e.target.value); setError('') }} error={error} />
        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            Keep examination
          </Button>
          <Button variant="danger" icon={Ban} onClick={submit} loading={busy}>
            Cancel examination
          </Button>
        </div>
      </div>
    </Modal>
  )
}
