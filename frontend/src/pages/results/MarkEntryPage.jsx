import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { CheckCircle2, ClipboardCheck, History, Lock, RotateCcw, Save, Send, ShieldCheck, Undo2 } from 'lucide-react'
import { getErrorCode, getErrorMessage, getFieldErrors } from '../../api/client'
import { resultsApi } from '../../api/services'
import { SheetStatusBadge } from '../../components/results/ResultBadges'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { Alert, Card, EmptyState, ErrorState, PageHeader, Skeleton } from '../../components/ui/Display'
import { ConfirmDialog, Modal } from '../../components/ui/Modal'
import { PersonAvatar } from '../../components/ui/PersonAvatar'
import { TextArea } from '../../components/ui/Sections'
import { useToast } from '../../components/ui/Toast'
import { formatMarks, fromHundredths, parseMarksInput } from '../../lib/results'
import { formatDateOnly } from '../../lib/students'
import { cn, formatDateTime } from '../../lib/utils'

/** Draft values of a row: attendance plus typed strings. */
function rowDraft(row, components) {
  const m = row.mark
  const comps = {}
  for (const c of components) comps[c.id] = ''
  if (!m) return { attendance: 'present', total: '', comps }
  for (const cm of m.components) comps[cm.component_id] = String(cm.obtained)
  return { attendance: m.attendance, total: m.attendance === 'present' && !components.length ? String(m.total) : '', comps }
}

const sameDraft = (a, b) => JSON.stringify(a) === JSON.stringify(b)

/** Checks one row while typing; the server re-validates and computes totals. */
function checkRow(d, scheme) {
  const errors = {}
  if (d.attendance === 'absent') return { errors, total: 0, empty: false }
  const comps = scheme.components
  if (comps.length) {
    let total = 0
    let filled = 0
    for (const c of comps) {
      const p = parseMarksInput(d.comps[c.id])
      if (p.empty) continue
      filled += 1
      if (p.error) errors[c.id] = p.error
      else if (p.value > Math.round(c.max_marks * 100)) errors[c.id] = `Max ${formatMarks(c.max_marks)}`
      else total += p.value
    }
    if (filled === 0) return { errors, total: null, empty: true }
    if (filled < comps.length) errors.row = 'Fill every component'
    return { errors, total, empty: false }
  }
  const p = parseMarksInput(d.total)
  if (p.empty) return { errors, total: null, empty: true }
  if (p.error) errors.total = p.error
  else if (p.value > Math.round(scheme.full_marks * 100)) errors.total = `Max ${formatMarks(scheme.full_marks)}`
  return { errors, total: p.value ?? null, empty: false }
}

const cellInput =
  'h-9 w-20 rounded-lg bg-white px-2 text-right text-sm tabular-nums ring-1 ring-inset focus:ring-2 focus:ring-brand-500 focus:outline-none disabled:bg-slate-50 disabled:text-slate-500'

export default function MarkEntryPage() {
  const { scheduleId } = useParams()
  const [params] = useSearchParams()
  const sectionId = params.get('section') ?? ''
  const toast = useToast()
  const [state, setState] = useState({ loading: true, error: null, data: null })
  const [drafts, setDrafts] = useState({})
  const [serverErrors, setServerErrors] = useState({})
  const [saving, setSaving] = useState(false)
  const [confirm, setConfirm] = useState(null) // submit | verify | finalize
  const [reopening, setReopening] = useState(false)
  const [reason, setReason] = useState('')
  const [reasonError, setReasonError] = useState('')
  const [busy, setBusy] = useState(false)
  const [history, setHistory] = useState(null)
  const [reloadKey, setReloadKey] = useState(0)
  const reload = useCallback(() => setReloadKey((k) => k + 1), [])

  useEffect(() => {
    let cancelled = false
    setState((s) => ({ ...s, loading: true, error: null }))
    resultsApi
      .sheet(scheduleId, sectionId)
      .then((data) => !cancelled && setState({ loading: false, error: null, data }))
      .catch((err) => !cancelled && setState({ loading: false, error: getErrorMessage(err), data: null }))
    return () => {
      cancelled = true
    }
  }, [scheduleId, sectionId, reloadKey])

  const data = state.data
  const loaded = useMemo(() => {
    const out = {}
    if (data) for (const r of data.students) out[r.enrollment_id] = rowDraft(r, data.scheme.components)
    return out
  }, [data])
  useEffect(() => {
    setDrafts(loaded)
    setServerErrors({})
  }, [loaded])

  const changed = data ? data.students.filter((r) => !sameDraft(drafts[r.enrollment_id], loaded[r.enrollment_id])) : []
  const dirty = changed.length > 0
  useEffect(() => {
    if (!dirty) return
    const h = (e) => {
      e.preventDefault()
      e.returnValue = ''
    }
    window.addEventListener('beforeunload', h)
    return () => window.removeEventListener('beforeunload', h)
  }, [dirty])

  if (state.error) {
    return (
      <div className="space-y-6">
        <Breadcrumbs items={[{ label: 'Marks entry', to: '/marks' }, { label: 'Marks' }]} />
        <Card>
          <ErrorState title="Couldn't load the marks" message={state.error} onRetry={reload} />
        </Card>
      </div>
    )
  }
  if (state.loading || !data) return <Skeleton className="h-96" />

  const { sheet, scheme } = data
  const comps = scheme.components
  const checks = {}
  for (const r of data.students) checks[r.enrollment_id] = checkRow(drafts[r.enrollment_id] ?? rowDraft(r, comps), scheme)
  const hasErrors = changed.some((r) => Object.keys(checks[r.enrollment_id].errors).length > 0)
  const eligible = data.students.filter((r) => r.eligible)
  const marked = eligible.filter((r) => r.mark).length
  const unmarked = eligible.length - marked
  const absentCount = eligible.filter((r) => r.mark?.attendance === 'absent').length
  const setDraft = (id, patch) => {
    setDrafts((d) => ({ ...d, [id]: { ...d[id], ...patch } }))
    setServerErrors((e) => ({ ...e, [id]: undefined }))
  }

  const save = async () => {
    const toSend = changed.filter((r) => !checks[r.enrollment_id].empty)
    const cleared = changed.filter((r) => checks[r.enrollment_id].empty && r.mark)
    if (cleared.length) {
      toast.error("Marks can't be removed", 'Saved marks can be changed but not cleared. Mark the student absent if they missed the paper.')
      return
    }
    if (!toSend.length) return
    const records = toSend.map((r) => {
      const d = drafts[r.enrollment_id]
      const rec = { enrollment_id: r.enrollment_id, attendance: d.attendance, version: r.mark?.version ?? 0 }
      if (d.attendance === 'present') {
        if (comps.length) rec.components = comps.map((c) => ({ component_id: c.id, obtained: d.comps[c.id].trim() }))
        else rec.total = d.total.trim()
      }
      return rec
    })
    setSaving(true)
    try {
      const res = await resultsApi.saveMarks(scheduleId, { section_id: sectionId, records })
      setState({ loading: false, error: null, data: res.sheet })
      toast.success('Marks saved', `${res.saved} student${res.saved === 1 ? '' : 's'} saved as draft.`)
    } catch (err) {
      const byRow = {}
      for (const [key, msg] of Object.entries(getFieldErrors(err))) {
        const m = /^records\[(\d+)\]\.(\w+)/.exec(key)
        if (m && toSend[Number(m[1])]) byRow[toSend[Number(m[1])].enrollment_id] = msg
      }
      setServerErrors(byRow)
      toast.error(getErrorCode(err) === 'MARKS_CHANGED' ? 'Someone else changed these marks' : "Marks weren't saved", getErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  const runAction = async () => {
    setBusy(true)
    try {
      const fn = { submit: resultsApi.submit, verify: resultsApi.verify, finalize: resultsApi.finalize }[confirm]
      const v = await fn(scheduleId, sectionId)
      setState({ loading: false, error: null, data: v })
      toast.success({ submit: 'Marks submitted', verify: 'Marks verified', finalize: 'Marks finalized' }[confirm])
      setConfirm(null)
    } catch (err) {
      toast.error('Action failed', getErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  const reopen = async () => {
    if (!reason.trim()) {
      setReasonError('Give a reason.')
      return
    }
    setBusy(true)
    try {
      const v = await resultsApi.reopen(scheduleId, sectionId, reason.trim())
      setState({ loading: false, error: null, data: v })
      toast.success('Marks reopened', 'They are back in draft for correction.')
      setReopening(false)
      setReason('')
    } catch (err) {
      toast.error("Couldn't reopen", getErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  const openHistory = async (r) => {
    setHistory({ row: r, items: null })
    try {
      setHistory({ row: r, items: await resultsApi.markHistory(r.mark.id) })
    } catch {
      setHistory({ row: r, items: [] })
    }
  }

  const compName = (id) => comps.find((c) => c.id === id)?.name ?? 'Component'

  return (
    <div className="space-y-6 pb-24 animate-fade-in">
      <div className="space-y-3">
        <Breadcrumbs items={[{ label: 'Marks entry', to: `/marks?exam=${sheet.paper.examination.id}` }, { label: `${sheet.paper.subject.name} · Section ${sheet.section.name}` }]} />
        <PageHeader
          eyebrow={<SheetStatusBadge status={sheet.status} started={sheet.started} />}
          title={`${sheet.paper.subject.name} · ${sheet.paper.class.name} – ${sheet.section.name}`}
          description={`${sheet.paper.examination.name} · ${formatDateOnly(sheet.paper.exam_date)} · Full marks ${formatMarks(scheme.full_marks)}, pass marks ${formatMarks(scheme.pass_marks)}${comps.length ? ` (${comps.map((c) => `${c.name} ${formatMarks(c.max_marks)}`).join(' + ')})` : ''}`}
          actions={
            <>
              {data.can_reopen && (
                <Button variant="secondary" icon={Undo2} onClick={() => setReopening(true)}>
                  {sheet.status === 'finalized' ? 'Reopen' : 'Return for correction'}
                </Button>
              )}
              {data.can_verify && (
                <Button icon={ShieldCheck} onClick={() => setConfirm('verify')}>
                  Verify
                </Button>
              )}
              {data.can_finalize && (
                <Button variant="success" icon={Lock} onClick={() => setConfirm('finalize')}>
                  Finalize
                </Button>
              )}
              {data.can_edit && (
                <Button icon={Send} onClick={() => setConfirm('submit')} disabled={dirty || unmarked > 0 || marked === 0}>
                  Submit for verification
                </Button>
              )}
            </>
          }
        />
      </div>

      {data.lock_reason && (
        <Alert tone="info" icon={Lock} title="Read only">
          {data.lock_reason}
        </Alert>
      )}
      {sheet.reopen_reason && sheet.status === 'draft' && (
        <Alert tone="warning" title="Returned for correction">
          {sheet.reopen_reason}
          {sheet.reopened_by && ` — ${sheet.reopened_by.name}, ${formatDateTime(sheet.reopened_at)}`}
        </Alert>
      )}
      {scheme.instructions && <Alert tone="info">{scheme.instructions}</Alert>}

      <dl className="grid grid-cols-2 gap-px overflow-hidden rounded-xl border border-slate-200/80 bg-slate-100 sm:grid-cols-4" aria-live="polite">
        {[
          ['Students', eligible.length],
          ['Marked', marked],
          ['Absent', absentCount],
          ['Unmarked', unmarked],
        ].map(([label, value]) => (
          <div key={label} className={cn('bg-white px-4 py-3', label === 'Unmarked' && value > 0 && 'bg-amber-50/60')}>
            <dt className="text-xs text-slate-500">{label}</dt>
            <dd className="mt-0.5 text-xl font-semibold text-slate-900 tabular-nums">{value}</dd>
          </div>
        ))}
      </dl>

      <Card className="overflow-hidden">
        {data.students.length === 0 ? (
          <EmptyState icon={ClipboardCheck} title="No students" description="Nobody is enrolled in this section." />
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full text-sm">
              <thead className="bg-slate-50/70 text-xs font-medium text-slate-500">
                <tr className="border-b border-slate-100">
                  <th className="w-14 px-4 py-2.5 text-right sm:pl-6">Roll</th>
                  <th className="px-3 py-2.5 text-left">Student</th>
                  <th className="px-3 py-2.5 text-left">Exam attendance</th>
                  {comps.length ? (
                    comps.map((c) => (
                      <th key={c.id} className="px-3 py-2.5 text-right whitespace-nowrap">
                        {c.name} <span className="font-normal text-slate-400">/{formatMarks(c.max_marks)}</span>
                      </th>
                    ))
                  ) : (
                    <th className="px-3 py-2.5 text-right">
                      Marks <span className="font-normal text-slate-400">/{formatMarks(scheme.full_marks)}</span>
                    </th>
                  )}
                  <th className="px-3 py-2.5 text-right">Total</th>
                  <th className="px-4 py-2.5 text-left sm:pr-6">Status</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {data.students.map((r) => {
                  const d = drafts[r.enrollment_id] ?? rowDraft(r, comps)
                  const chk = checks[r.enrollment_id]
                  const editable = data.can_edit && (r.eligible || r.mark)
                  const absent = d.attendance === 'absent'
                  const isChanged = !sameDraft(d, loaded[r.enrollment_id])
                  const compErr = comps.map((c) => chk.errors[c.id] && `${c.name}: ${chk.errors[c.id]}`).find(Boolean)
                  const err = serverErrors[r.enrollment_id] || compErr || chk.errors.row || chk.errors.total
                  return (
                    <tr key={r.enrollment_id} className={cn(isChanged && 'bg-brand-50/40', !r.mark && r.eligible && !isChanged && 'bg-amber-50/30')} data-testid="marks-row">
                      <td className="px-4 py-2 text-right font-mono text-slate-500 tabular-nums sm:pl-6">{r.roll_number}</td>
                      <td className="px-3 py-2">
                        <div className="flex items-center gap-2.5">
                          <PersonAvatar person={r.student} size="sm" />
                          <span className="min-w-0">
                            <span className="block font-medium text-slate-900">{r.student.full_name}</span>
                            <span className="block font-mono text-xs text-slate-500">
                              {r.student.admission_number}
                              {!r.eligible && <span className="font-sans text-amber-700"> · {r.enrollment_status}</span>}
                            </span>
                          </span>
                        </div>
                      </td>
                      <td className="px-3 py-2">
                        <div role="radiogroup" aria-label={`Exam attendance of ${r.student.full_name}`} className="inline-flex rounded-lg bg-slate-100 p-0.5">
                          {['present', 'absent'].map((a) => (
                            <button
                              key={a}
                              type="button"
                              role="radio"
                              aria-checked={d.attendance === a}
                              disabled={!editable}
                              onClick={() => setDraft(r.enrollment_id, a === 'absent' ? { attendance: a, total: '', comps: Object.fromEntries(comps.map((c) => [c.id, ''])) } : { attendance: a })}
                              className={cn(
                                'rounded-md px-2.5 py-1 text-xs font-medium capitalize disabled:cursor-not-allowed',
                                d.attendance === a ? (a === 'absent' ? 'bg-rose-600 text-white' : 'bg-white text-slate-900 shadow-sm') : 'text-slate-600',
                              )}
                            >
                              {a}
                            </button>
                          ))}
                        </div>
                      </td>
                      {comps.length ? (
                        comps.map((c) => (
                          <td key={c.id} className="px-3 py-2 text-right">
                            <input
                              inputMode="decimal"
                              value={absent ? '' : (d.comps[c.id] ?? '')}
                              disabled={!editable || absent}
                              placeholder={absent ? 'AB' : ''}
                              onChange={(e) => setDraft(r.enrollment_id, { comps: { ...d.comps, [c.id]: e.target.value } })}
                              aria-label={`${c.name} marks for ${r.student.full_name}`}
                              aria-invalid={!!chk.errors[c.id] || undefined}
                              title={chk.errors[c.id]}
                              className={cn(cellInput, chk.errors[c.id] ? 'ring-rose-300' : 'ring-slate-200')}
                            />
                          </td>
                        ))
                      ) : (
                        <td className="px-3 py-2 text-right">
                          <input
                            inputMode="decimal"
                            value={absent ? '' : d.total}
                            disabled={!editable || absent}
                            placeholder={absent ? 'AB' : ''}
                            onChange={(e) => setDraft(r.enrollment_id, { total: e.target.value })}
                            aria-label={`Marks for ${r.student.full_name}`}
                            aria-invalid={!!chk.errors.total || undefined}
                            className={cn(cellInput, chk.errors.total ? 'ring-rose-300' : 'ring-slate-200')}
                          />
                        </td>
                      )}
                      <td className="px-3 py-2 text-right font-semibold text-slate-900 tabular-nums" data-testid="row-total">
                        {absent ? 'Absent' : chk.total == null ? '—' : fromHundredths(chk.total)}
                      </td>
                      <td className="px-4 py-2 sm:pr-6">
                        {err ? (
                          <span className="text-xs text-rose-600">{err}</span>
                        ) : isChanged ? (
                          <span className="text-xs text-brand-700">Not saved</span>
                        ) : r.mark ? (
                          <span className="inline-flex items-center gap-1 text-xs text-emerald-700">
                            <CheckCircle2 className="size-3.5" aria-hidden="true" />
                            Saved
                            {r.mark.changes > 0 && (
                              <button type="button" onClick={() => openHistory(r)} className="ml-1 inline-flex items-center gap-0.5 text-slate-500 hover:text-slate-800" aria-label={`Change history of ${r.student.full_name}`}>
                                <History className="size-3.5" />
                                {r.mark.changes}
                              </button>
                            )}
                          </span>
                        ) : (
                          <span className="text-xs text-amber-700">Unmarked</span>
                        )}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      {data.can_edit && (
        <div className="fixed inset-x-0 bottom-0 z-20 border-t border-slate-200 bg-white/95 backdrop-blur lg:pl-64 print:hidden">
          <div className="mx-auto flex max-w-7xl flex-col gap-2 px-4 py-3 sm:flex-row sm:items-center sm:px-6 lg:px-10">
            <p className="flex-1 text-sm text-slate-600" aria-live="polite">
              {hasErrors ? (
                <span className="font-medium text-rose-700">Fix the highlighted marks before saving.</span>
              ) : dirty ? (
                `${changed.length} student${changed.length === 1 ? '' : 's'} changed — not saved yet`
              ) : unmarked > 0 ? (
                `${unmarked} student${unmarked === 1 ? '' : 's'} still unmarked. Unmarked students never get zero automatically.`
              ) : (
                'All marks saved.'
              )}
            </p>
            <Button variant="ghost" icon={RotateCcw} onClick={() => setDrafts(loaded)} disabled={!dirty || saving}>
              Reset
            </Button>
            <Button icon={Save} onClick={save} loading={saving} disabled={!dirty || hasErrors}>
              Save draft
            </Button>
          </div>
        </div>
      )}

      <ConfirmDialog
        open={!!confirm}
        onClose={() => setConfirm(null)}
        onConfirm={runAction}
        loading={busy}
        tone={confirm === 'finalize' ? 'success' : 'brand'}
        icon={confirm === 'finalize' ? Lock : confirm === 'verify' ? ShieldCheck : Send}
        title={{ submit: 'Submit these marks?', verify: 'Verify these marks?', finalize: 'Finalize these marks?' }[confirm] ?? ''}
        description={
          {
            submit: `${marked} student${marked === 1 ? '' : 's'} marked (${absentCount} absent). After submitting, the marks are read-only until an administrator returns them.`,
            verify: 'Confirms the submitted marks are correct. They can still be returned for correction.',
            finalize: 'Finalized marks are locked and used for result processing. Later corrections need a reason.',
          }[confirm]
        }
        confirmLabel={{ submit: 'Submit marks', verify: 'Verify', finalize: 'Finalize' }[confirm] ?? 'Confirm'}
      />

      <Modal open={reopening} onClose={busy ? undefined : () => setReopening(false)} icon={Undo2} title="Return the marks for correction" description="The marks go back to draft. Any unpublished processed results of this examination are discarded and must be processed again.">
        <div className="space-y-4">
          <TextArea label="Reason" required rows={3} value={reason} onChange={(e) => { setReason(e.target.value); setReasonError('') }} error={reasonError} />
          <div className="flex justify-end gap-2">
            <Button variant="secondary" onClick={() => setReopening(false)} disabled={busy}>
              Cancel
            </Button>
            <Button icon={Undo2} onClick={reopen} loading={busy}>
              Return to draft
            </Button>
          </div>
        </div>
      </Modal>

      <Modal open={!!history} onClose={() => setHistory(null)} icon={History} title="Change history" description={history?.row.student.full_name}>
        {!history?.items ? (
          <Skeleton className="h-16" />
        ) : (
          <ol className="space-y-3">
            {history.items.map((h) => (
              <li key={h.id} className="rounded-lg border border-slate-200 p-3 text-sm">
                <p className="text-slate-800">
                  {h.before.attendance === 'absent' ? 'Absent' : h.before.total} → {h.after.attendance === 'absent' ? 'Absent' : h.after.total}
                </p>
                {Object.keys(h.after.components ?? {}).length > 0 && (
                  <p className="text-xs text-slate-500">
                    {Object.entries(h.after.components)
                      .map(([id, v]) => `${compName(id)}: ${h.before.components?.[id] ?? '—'} → ${v}`)
                      .join(' · ')}
                  </p>
                )}
                <p className="mt-1 text-xs text-slate-500">
                  {h.changed_by?.name ?? 'Unknown'} · {formatDateTime(h.changed_at)}
                  {h.reason && ` · ${h.reason}`}
                </p>
              </li>
            ))}
          </ol>
        )}
      </Modal>
      <p className="text-xs text-slate-400 print:hidden">
        Totals, grades and GPAs are always calculated by the server. <Link to="/marks" className="underline">Back to papers</Link>
      </p>
    </div>
  )
}
