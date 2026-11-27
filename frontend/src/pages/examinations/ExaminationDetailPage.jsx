import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { Ban, CalendarDays, CheckCircle2, ClipboardPen, FileText, History, Info, Layers, Pencil, Plus, Send, Trash2, X } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { examinationsApi } from '../../api/services'
import { ExamStatusBadge } from '../../components/examinations/ExamStatusBadge'
import { TimetableTable } from '../../components/examinations/TimetableView'
import { useExamActions } from '../../components/examinations/useExamActions'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Alert, Card, CardHeader, EmptyState, ErrorState, PageHeader, Skeleton } from '../../components/ui/Display'
import { ConfirmDialog, Modal } from '../../components/ui/Modal'
import { DetailItem, DetailSection, ProfileSkeleton } from '../../components/ui/Sections'
import { useToast } from '../../components/ui/Toast'
import { useAuth } from '../../hooks/useAuth'
import { loadClasses } from '../../lib/enrollments'
import { examActions } from '../../lib/examinations'
import { PERMISSIONS } from '../../lib/roles'
import { formatDateOnly } from '../../lib/students'
import { cn, formatDateTime } from '../../lib/utils'

function AddClassesModal({ exam, onClose, onSaved }) {
  const toast = useToast()
  const [classes, setClasses] = useState(null)
  const [selected, setSelected] = useState([])
  const [busy, setBusy] = useState(false)
  useEffect(() => {
    loadClasses()
      .then(setClasses)
      .catch(() => setClasses([]))
  }, [])
  const taken = new Set(exam.classes.map((c) => c.class.id))
  const available = (classes ?? []).filter((c) => !taken.has(c.id))
  const save = async () => {
    setBusy(true)
    try {
      await examinationsApi.addClasses(exam.id, selected)
      toast.success('Classes added', `${selected.length} class${selected.length === 1 ? '' : 'es'} added to ${exam.name}.`)
      onSaved()
      onClose()
    } catch (err) {
      toast.error("Couldn't add classes", getErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Modal open onClose={busy ? undefined : onClose} icon={Layers} title="Add classes" description="Every section of a class sits the same timetable.">
      <div className="space-y-4">
        {!classes ? (
          <Skeleton className="h-24" />
        ) : available.length === 0 ? (
          <p className="text-sm text-slate-500">Every class is already part of this examination.</p>
        ) : (
          <div className="grid grid-cols-2 gap-2" role="group" aria-label="Classes">
            {available.map((c) => {
              const on = selected.includes(c.id)
              return (
                <label
                  key={c.id}
                  className={cn(
                    'flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-sm',
                    on ? 'border-brand-300 bg-brand-50' : 'border-slate-200',
                    c.status !== 'active' && 'cursor-not-allowed opacity-50',
                  )}
                >
                  <input
                    type="checkbox"
                    disabled={c.status !== 'active'}
                    checked={on}
                    onChange={() => setSelected((cur) => (on ? cur.filter((x) => x !== c.id) : [...cur, c.id]))}
                    className="size-4 rounded border-slate-300 text-brand-600"
                  />
                  {c.name}
                  {c.status !== 'active' && <span className="text-xs text-slate-500">(inactive)</span>}
                </label>
              )
            })}
          </div>
        )}
        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            Cancel
          </Button>
          <Button icon={Plus} onClick={save} loading={busy} disabled={selected.length === 0}>
            Add {selected.length || ''} class{selected.length === 1 ? '' : 'es'}
          </Button>
        </div>
      </div>
    </Modal>
  )
}

export default function ExaminationDetailPage() {
  const { id } = useParams()
  const { can } = useAuth()
  const navigate = useNavigate()
  const toast = useToast()
  const [state, setState] = useState({ loading: true, error: null, exam: null })
  const [history, setHistory] = useState(null)
  const [reloadKey, setReloadKey] = useState(0)
  const [adding, setAdding] = useState(false)
  const [removing, setRemoving] = useState(null)
  const [busy, setBusy] = useState(false)
  const reload = useCallback(() => setReloadKey((k) => k + 1), [])
  const actions = useExamActions({ onChanged: reload, onDeleted: () => navigate('/examinations') })

  useEffect(() => {
    let cancelled = false
    setState((s) => ({ ...s, loading: true, error: null }))
    examinationsApi
      .get(id)
      .then((exam) => !cancelled && setState({ loading: false, error: null, exam }))
      .catch((err) => !cancelled && setState({ loading: false, error: getErrorMessage(err), exam: null }))
    examinationsApi
      .history(id)
      .then((h) => !cancelled && setHistory(h))
      .catch(() => !cancelled && setHistory([]))
    return () => {
      cancelled = true
    }
  }, [id, reloadKey])

  const x = state.exam
  const crumbs = [{ label: 'Examinations', to: '/examinations' }, { label: x?.name ?? 'Examination' }]
  if (state.error) {
    return (
      <div className="space-y-6">
        <Breadcrumbs items={crumbs} />
        <Card>
          <ErrorState title="Couldn't load the examination" message={state.error} onRetry={reload} />
        </Card>
      </div>
    )
  }
  if (state.loading && !x) return <ProfileSkeleton />

  const a = examActions(x)
  const removeClass = async () => {
    setBusy(true)
    try {
      await examinationsApi.removeClass(x.id, removing.class.id)
      toast.success('Class removed', `${removing.class.name} and its papers were removed from ${x.name}.`)
      setRemoving(null)
      reload()
    } catch (err) {
      toast.error("Couldn't remove the class", getErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-6 animate-fade-in">
      <div className="space-y-3">
        <Breadcrumbs items={crumbs} />
        <PageHeader
          eyebrow={<ExamStatusBadge status={x.status} />}
          title={x.name}
          description={`${x.exam_type.name} · ${x.academic_year.name} · ${formatDateOnly(x.start_date)} – ${formatDateOnly(x.end_date)}`}
          actions={
            <>
              <Link to={`/examinations/${x.id}/timetable`} className={buttonClasses('secondary')}>
                <FileText className="size-4" aria-hidden="true" />
                Timetable
              </Link>
              {can(PERMISSIONS.EXAMS_UPDATE) && a.edit && (
                <Link to={`/examinations/${x.id}/edit`} className={buttonClasses('secondary')}>
                  <Pencil className="size-4" aria-hidden="true" />
                  Edit
                </Link>
              )}
              {can(PERMISSIONS.EXAMS_UPDATE) && a.schedule && (
                <Link to={`/examinations/${x.id}/schedule`} className={buttonClasses(a.publish ? 'secondary' : 'primary')}>
                  <ClipboardPen className="size-4" aria-hidden="true" />
                  Manage schedule
                </Link>
              )}
              {can(PERMISSIONS.EXAMS_PUBLISH) && a.publish && (
                <Button icon={Send} onClick={() => actions.publish(x)}>
                  Publish
                </Button>
              )}
              {can(PERMISSIONS.EXAMS_PUBLISH) && a.complete && (
                <Button variant="success" icon={CheckCircle2} onClick={() => actions.complete(x)}>
                  Mark completed
                </Button>
              )}
              {can(PERMISSIONS.EXAMS_PUBLISH) && a.cancel && (
                <Button variant="ghost" icon={Ban} onClick={() => actions.cancel(x)}>
                  Cancel
                </Button>
              )}
              {can(PERMISSIONS.EXAMS_DELETE) && a.remove && (
                <Button variant="ghost" icon={Trash2} onClick={() => actions.remove(x)}>
                  Delete
                </Button>
              )}
            </>
          }
        />
      </div>

      {x.status === 'published' && (
        <Alert tone="info" icon={Send} title="Published timetable">
          Teachers of these classes can see it. Changes need a reason and are recorded in the history below; papers and classes can no longer be removed.
        </Alert>
      )}
      {x.status === 'cancelled' && (
        <Alert tone="danger" icon={Ban} title="Cancelled">
          {x.cancel_reason}
        </Alert>
      )}
      {x.status === 'completed' && (
        <Alert tone="success" icon={CheckCircle2} title="Completed">
          This examination is history. Its timetable is kept unchanged for marks and results.
        </Alert>
      )}

      <div className="grid gap-6 lg:grid-cols-3">
        <DetailSection title="Examination" icon={Info} className="lg:col-span-2">
          <DetailItem label="Type" value={x.exam_type.name} />
          <DetailItem label="Academic year" value={x.academic_year.name} />
          <DetailItem label="Starts" value={formatDateOnly(x.start_date)} />
          <DetailItem label="Ends" value={formatDateOnly(x.end_date)} />
          <DetailItem label="Classes" value={x.class_count} />
          <DetailItem label="Scheduled papers" value={x.schedule_count} />
          <DetailItem label="Description" value={x.description} wide />
          <DetailItem label="Instructions" value={x.instructions} wide />
        </DetailSection>
        <DetailSection title="Record" icon={CalendarDays}>
          <DetailItem label="Created" value={`${formatDateTime(x.created_at)}${x.created_by ? ` · ${x.created_by.name}` : ''}`} wide />
          <DetailItem label="Last updated" value={`${formatDateTime(x.updated_at)}${x.updated_by ? ` · ${x.updated_by.name}` : ''}`} wide />
          {x.published_at && <DetailItem label="Published" value={`${formatDateTime(x.published_at)}${x.published_by ? ` · ${x.published_by.name}` : ''}`} wide />}
          {x.completed_at && <DetailItem label="Completed" value={formatDateTime(x.completed_at)} wide />}
          {x.cancelled_at && <DetailItem label="Cancelled" value={formatDateTime(x.cancelled_at)} wide />}
        </DetailSection>
      </div>

      <Card className="overflow-hidden">
        <CardHeader
          title="Classes and timetables"
          description="Subject-wise papers per class. All sections of a class follow its timetable."
          action={
            can(PERMISSIONS.EXAMS_UPDATE) &&
            a.classes && (
              <Button size="sm" variant="secondary" icon={Plus} onClick={() => setAdding(true)}>
                Add classes
              </Button>
            )
          }
        />
        {x.classes.length === 0 ? (
          <EmptyState icon={Layers} title="No classes yet" description="Add the classes that sit this examination." />
        ) : (
          <div className="divide-y divide-slate-100">
            {x.classes.map((c) => (
              <section key={c.id} aria-label={c.class.name}>
                <div className="flex flex-wrap items-center gap-3 px-5 py-3 sm:px-6">
                  <h3 className="flex-1 text-sm font-semibold text-slate-900">
                    {c.class.name}
                    <span className="ml-2 text-xs font-normal text-slate-500">
                      {c.sections} section{c.sections === 1 ? '' : 's'} · {c.students} student{c.students === 1 ? '' : 's'} · {c.schedules.length}/{c.subjects.length} subjects scheduled
                    </span>
                  </h3>
                  {can(PERMISSIONS.EXAMS_UPDATE) && a.schedule && (
                    <Link to={`/examinations/${x.id}/schedule?class=${c.class.id}`} className={buttonClasses('ghost', 'sm')}>
                      <ClipboardPen className="size-4" aria-hidden="true" />
                      Edit schedule
                    </Link>
                  )}
                  {can(PERMISSIONS.EXAMS_UPDATE) && a.classes && (
                    <Button size="sm" variant="ghost" icon={X} onClick={() => setRemoving(c)} aria-label={`Remove ${c.class.name}`}>
                      Remove
                    </Button>
                  )}
                </div>
                <TimetableTable schedules={c.schedules} showInstructions={false} />
              </section>
            ))}
          </div>
        )}
      </Card>

      <Card className="overflow-hidden">
        <CardHeader title="History" description="Every change to this examination, with who made it and why." />
        {!history ? (
          <div className="p-5">
            <Skeleton className="h-10" />
          </div>
        ) : history.length === 0 ? (
          <EmptyState icon={History} title="No history yet" />
        ) : (
          <ol className="divide-y divide-slate-100">
            {history.map((h) => (
              <li key={h.id} className="px-5 py-3 sm:px-6">
                <p className="text-sm text-slate-800">{h.summary}</p>
                <p className="text-xs text-slate-500">
                  {h.actor?.name ?? 'System'} · {formatDateTime(h.created_at)}
                  {h.metadata?.reason && <span className="text-slate-700"> · Reason: {h.metadata.reason}</span>}
                </p>
                {Array.isArray(h.metadata?.changes) && h.metadata.changes.length > 0 && (
                  <ul className="mt-1 text-xs text-slate-500">
                    {h.metadata.changes.map((c, i) => (
                      <li key={i}>
                        {c.subject}: {c.from} → {c.to}
                      </li>
                    ))}
                  </ul>
                )}
              </li>
            ))}
          </ol>
        )}
      </Card>

      {adding && <AddClassesModal exam={x} onClose={() => setAdding(false)} onSaved={reload} />}
      <ConfirmDialog
        open={!!removing}
        onClose={() => setRemoving(null)}
        onConfirm={removeClass}
        loading={busy}
        title={`Remove ${removing?.class.name ?? 'class'}?`}
        description={`Its ${removing?.schedules.length ?? 0} scheduled paper(s) in this draft will be removed too.`}
        confirmLabel="Remove class"
      />
      {actions.dialog}
    </div>
  )
}
