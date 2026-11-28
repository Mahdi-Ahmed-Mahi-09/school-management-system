import { useCallback, useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { AlertTriangle, Award, BarChart3, Check, CheckCircle2, ClipboardList, FileCheck2, ListOrdered, NotebookPen, Settings2, Trophy, Users, XCircle } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { examinationsApi, resultsApi } from '../../api/services'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Card, CardHeader, EmptyState, ErrorState, PageHeader, Skeleton, StatCard } from '../../components/ui/Display'
import { Select } from '../../components/ui/Field'
import { ConfirmDialog, Modal } from '../../components/ui/Modal'
import { TextArea } from '../../components/ui/Sections'
import { useToast } from '../../components/ui/Toast'
import { useAuth } from '../../hooks/useAuth'
import { loadSections } from '../../lib/enrollments'
import { formatGpa, formatMarks, formatPct, GRADE_SCALE, STAGE_LABEL, STAGES } from '../../lib/results'
import { PERMISSIONS } from '../../lib/roles'
import { cn, formatDateTime, formatNumber } from '../../lib/utils'

const ACTIONS = {
  process_subjects: { label: 'Process subject results', run: resultsApi.processSubjects, perm: PERMISSIONS.RESULTS_PROCESS, done: 'Subject results processed',
    text: 'Grades every finalized paper of every student. Running it again replaces unpublished results.' },
  process_overall: { label: 'Process overall results', run: resultsApi.processOverall, perm: PERMISSIONS.RESULTS_PROCESS, done: 'Overall results processed',
    text: 'Calculates totals, GPAs, overall grades and class/section positions. One failed subject means GPA 0.00 and F.' },
  verify: { label: 'Verify results', run: resultsApi.verifyResults, perm: PERMISSIONS.RESULTS_PROCESS, done: 'Results verified',
    text: 'Confirms the processed results have been reviewed and are ready to publish.' },
  publish: { label: 'Publish results', run: resultsApi.publish, perm: PERMISSIONS.RESULTS_PUBLISH, done: 'Results published',
    text: 'Publishes the results and keeps a permanent copy. Afterwards, changes need an audited correction.' },
}

function Stepper({ stage }) {
  const order = STAGES.map((s) => s.key)
  const at = stage === 'correction' ? 0 : order.indexOf(stage)
  return (
    <ol className="flex flex-wrap gap-2" aria-label="Result stages">
      {STAGES.map((s, i) => {
        const done = i < at || (stage === 'published' && i === at)
        const current = i === at && stage !== 'published'
        return (
          <li
            key={s.key}
            className={cn(
              'flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium ring-1 ring-inset',
              done ? 'bg-emerald-50 text-emerald-700 ring-emerald-600/15' : current ? 'bg-brand-50 text-brand-700 ring-brand-600/20' : 'bg-slate-50 text-slate-500 ring-slate-400/15',
            )}
            aria-current={current ? 'step' : undefined}
          >
            {done ? <Check className="size-3.5" aria-hidden="true" /> : <span className="tabular-nums">{i + 1}</span>}
            {s.label}
          </li>
        )
      })}
    </ol>
  )
}

export default function ResultsDashboardPage() {
  const { can } = useAuth()
  const toast = useToast()
  const [params, setParams] = useSearchParams()
  const [exams, setExams] = useState({ loading: true, error: null, items: [] })
  const [status, setStatus] = useState({ loading: false, error: null, data: null })
  const [classes, setClasses] = useState([])
  const [sectionList, setSectionList] = useState([])
  const [reloadKey, setReloadKey] = useState(0)
  const [confirm, setConfirm] = useState(null)
  const [correcting, setCorrecting] = useState(false)
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  const reload = useCallback(() => setReloadKey((k) => k + 1), [])

  useEffect(() => {
    resultsApi
      .markExams()
      .then(({ exams: items }) => setExams({ loading: false, error: null, items }))
      .catch((err) => setExams({ loading: false, error: getErrorMessage(err), items: [] }))
  }, [])
  const examId = params.get('exam') || exams.items[0]?.id || ''
  const classId = params.get('class') ?? ''
  const sectionId = params.get('section') ?? ''

  useEffect(() => {
    if (!examId) return
    examinationsApi
      .get(examId)
      .then((x) => setClasses(x.classes))
      .catch(() => setClasses([]))
  }, [examId])

  useEffect(() => {
    if (!examId) return
    let cancelled = false
    setStatus((s) => ({ ...s, loading: true, error: null }))
    resultsApi
      .status(examId, { class_id: classId || undefined, section_id: sectionId || undefined })
      .then((data) => !cancelled && setStatus({ loading: false, error: null, data }))
      .catch((err) => !cancelled && setStatus({ loading: false, error: getErrorMessage(err), data: null }))
    return () => {
      cancelled = true
    }
  }, [examId, classId, sectionId, reloadKey])

  const yearId = exams.items.find((x) => x.id === examId)?.academic_year.id
  useEffect(() => {
    let cancelled = false
    loadSections(yearId, classId)
      .then((items) => !cancelled && setSectionList(items))
      .catch(() => !cancelled && setSectionList([]))
    return () => {
      cancelled = true
    }
  }, [yearId, classId])

  const set = (changes) => {
    const next = new URLSearchParams(params)
    for (const [k, v] of Object.entries(changes)) {
      if (v) next.set(k, v)
      else next.delete(k)
    }
    setParams(next, { replace: true })
  }

  const run = async () => {
    const a = ACTIONS[confirm]
    setBusy(true)
    try {
      setStatus({ loading: false, error: null, data: await a.run(examId) })
      toast.success(a.done)
      setConfirm(null)
    } catch (err) {
      toast.error(`${a.label} failed`, getErrorMessage(err))
      setConfirm(null)
      reload()
    } finally {
      setBusy(false)
    }
  }
  const correct = async () => {
    if (!reason.trim()) return
    setBusy(true)
    try {
      setStatus({ loading: false, error: null, data: await resultsApi.correct(examId, reason.trim()) })
      toast.success('Correction started', 'Reopen the marks that need changing, then process and publish again.')
      setCorrecting(false)
      setReason('')
    } catch (err) {
      toast.error("Couldn't start a correction", getErrorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  const d = status.data
  const st = d?.stats
  const next = d?.next && ACTIONS[d.next]

  return (
    <div className="space-y-6 animate-fade-in">
      <PageHeader title="Marks & results" description="Follow each examination from marks entry to published results. Grades follow the Bangladesh grading system; one failed subject means GPA 0.00." />
      {exams.error ? (
        <Card>
          <ErrorState title="Couldn't load examinations" message={exams.error} />
        </Card>
      ) : exams.loading ? (
        <Skeleton className="h-40" />
      ) : exams.items.length === 0 ? (
        <Card>
          <EmptyState icon={NotebookPen} title="No published examinations" description="Results start once an examination timetable is published and marks are entered." />
        </Card>
      ) : (
        <>
          <Card className="p-4 sm:p-5">
            <div className="grid gap-3 sm:grid-cols-3">
              <Select label="Examination" value={examId} onChange={(e) => set({ exam: e.target.value, class: '', section: '' })}>
                {exams.items.map((x) => (
                  <option key={x.id} value={x.id}>
                    {x.name} · {x.academic_year.name}
                  </option>
                ))}
              </Select>
              <Select label="Class" value={classId} onChange={(e) => set({ class: e.target.value, section: '' })}>
                <option value="">All classes</option>
                {classes.map((c) => (
                  <option key={c.class.id} value={c.class.id}>
                    {c.class.name}
                  </option>
                ))}
              </Select>
              <Select label="Section" value={sectionId} disabled={!classId} onChange={(e) => set({ section: e.target.value })}>
                <option value="">{classId ? 'All sections' : 'Choose a class first'}</option>
                {sectionList.map((s) => (
                  <option key={s.id} value={s.id}>
                    Section {s.name}
                  </option>
                ))}
              </Select>
            </div>
          </Card>

          {status.error ? (
            <Card>
              <ErrorState title="Couldn't load the results" message={status.error} onRetry={reload} />
            </Card>
          ) : !d ? (
            <Skeleton className="h-64" />
          ) : (
            <>
              <div className="grid grid-cols-2 gap-4 lg:grid-cols-4 xl:grid-cols-7">
                <StatCard label="Students" value={formatNumber(st.students)} icon={Users} tone="brand" />
                <StatCard label="Marks entered" value={formatNumber(st.papers_marked)} icon={NotebookPen} tone="emerald" hint={`of ${formatNumber(st.papers_expected)} papers`} />
                <StatCard label="Marks pending" value={formatNumber(st.papers_expected - st.papers_marked)} icon={ClipboardList} tone="amber" hint={`${st.sheets_finalized}/${st.sheets_total} sheets finalized`} />
                <StatCard label="Results processed" value={formatNumber(st.results_processed)} icon={FileCheck2} tone="slate" />
                <StatCard label="Passed" value={formatNumber(st.passed)} icon={CheckCircle2} tone="emerald" />
                <StatCard label="Failed" value={formatNumber(st.failed)} icon={XCircle} tone="rose" />
                <StatCard label="Published" value={formatNumber(st.published)} icon={Award} tone="brand" />
              </div>

              <Card>
                <CardHeader
                  title={STAGE_LABEL[d.run.stage] ?? d.run.stage}
                  description={
                    d.run.stage === 'published'
                      ? `Published ${formatDateTime(d.run.published_at)}${d.run.published_by ? ` by ${d.run.published_by.name}` : ''}.`
                      : d.run.stage === 'correction'
                        ? `Correcting: ${d.run.correction_reason}`
                        : 'Marks must be finalized for every paper and section before results can be processed.'
                  }
                  action={
                    <div className="flex flex-wrap gap-2">
                      {d.run.stage === 'published' && can(PERMISSIONS.RESULTS_PUBLISH) && (
                        <Button variant="secondary" icon={AlertTriangle} onClick={() => setCorrecting(true)}>
                          Start correction
                        </Button>
                      )}
                      {next && can(next.perm) && (
                        <Button icon={next === ACTIONS.publish ? Award : FileCheck2} onClick={() => setConfirm(d.next)}>
                          {next.label}
                        </Button>
                      )}
                    </div>
                  }
                />
                <div className="space-y-4 p-5 sm:p-6">
                  <Stepper stage={d.run.stage} />
                  {d.issues.length > 0 && (
                    <div className="rounded-lg border border-amber-200 bg-amber-50/60 p-3">
                      <p className="mb-1.5 text-sm font-semibold text-amber-900">Before results can be processed</p>
                      <ul className="list-disc space-y-0.5 pl-5 text-sm text-amber-900">
                        {d.issues.map((is, i) => (
                          <li key={i}>{is.message}</li>
                        ))}
                      </ul>
                    </div>
                  )}
                  <div className="flex flex-wrap gap-2">
                    {can(PERMISSIONS.MARKS_CONFIGURE) && (
                      <Link to={`/results/${examId}/configuration`} className={buttonClasses('secondary', 'sm')}>
                        <Settings2 className="size-4" aria-hidden="true" />
                        Marks configuration
                      </Link>
                    )}
                    {can(PERMISSIONS.MARKS_VERIFY) && (
                      <Link to={`/results/${examId}/review`} className={buttonClasses('secondary', 'sm')}>
                        <ClipboardList className="size-4" aria-hidden="true" />
                        Marks review
                      </Link>
                    )}
                    <Link to={`/results/${examId}/results`} className={buttonClasses('secondary', 'sm')}>
                      <BarChart3 className="size-4" aria-hidden="true" />
                      Results
                    </Link>
                    <Link to={`/results/${examId}/results?tab=merit`} className={buttonClasses('secondary', 'sm')}>
                      <ListOrdered className="size-4" aria-hidden="true" />
                      Merit list
                    </Link>
                  </div>
                </div>
              </Card>

              <div className="grid gap-6 lg:grid-cols-3">
                <Card className="lg:col-span-2">
                  <CardHeader title="Result summary" description={st.results_processed ? 'From the processed results.' : 'Appears once overall results are processed.'} />
                  {st.results_processed === 0 ? (
                    <EmptyState icon={Trophy} title="No results yet" description="Process the results to see pass rates, GPAs and top marks." />
                  ) : (
                    <dl className="grid grid-cols-2 gap-px bg-slate-100 sm:grid-cols-3">
                      {[
                        ['Total students', formatNumber(st.results_processed)],
                        ['Passed', formatNumber(st.passed)],
                        ['Failed', formatNumber(st.failed)],
                        ['Pass percentage', formatPct(st.pass_percentage)],
                        ['Average GPA', formatGpa(st.average_gpa)],
                        ['Highest GPA', formatGpa(st.highest_gpa)],
                        ['Highest marks', formatMarks(st.highest_total)],
                      ].map(([k, v]) => (
                        <div key={k} className="bg-white px-5 py-3">
                          <dt className="text-xs text-slate-500">{k}</dt>
                          <dd className="text-lg font-semibold text-slate-900 tabular-nums">{v}</dd>
                        </div>
                      ))}
                    </dl>
                  )}
                </Card>
                <Card>
                  <CardHeader title="Grading scale" description="Bangladesh grading system" />
                  <table className="min-w-full text-sm">
                    <tbody className="divide-y divide-slate-100">
                      {GRADE_SCALE.map(([range, g, gp]) => (
                        <tr key={g}>
                          <td className="px-5 py-1.5 text-slate-600">{range}</td>
                          <td className="px-3 py-1.5 font-semibold text-slate-900">{g}</td>
                          <td className="px-5 py-1.5 text-right text-slate-700 tabular-nums">{gp}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </Card>
              </div>

              {d.publications.length > 0 && (
                <Card>
                  <CardHeader title="Publication history" description="Each publication keeps a copy of the results as published." />
                  <ul className="divide-y divide-slate-100">
                    {d.publications.map((p) => (
                      <li key={p.id} className="px-5 py-3 text-sm sm:px-6">
                        <span className="font-medium text-slate-900">Version {p.version}</span> · {formatDateTime(p.published_at)}
                        {p.published_by && ` · ${p.published_by.name}`} · {p.passed}/{p.students} passed
                        {p.correction_reason && <span className="block text-xs text-slate-500">Correction: {p.correction_reason}</span>}
                      </li>
                    ))}
                  </ul>
                </Card>
              )}
            </>
          )}
        </>
      )}

      <ConfirmDialog
        open={!!confirm}
        onClose={() => setConfirm(null)}
        onConfirm={run}
        loading={busy}
        tone={confirm === 'publish' ? 'success' : 'brand'}
        icon={confirm === 'publish' ? Award : FileCheck2}
        title={confirm ? `${ACTIONS[confirm].label}?` : ''}
        description={confirm ? ACTIONS[confirm].text : ''}
        confirmLabel={confirm ? ACTIONS[confirm].label : 'Confirm'}
      />
      <Modal open={correcting} onClose={busy ? undefined : () => setCorrecting(false)} icon={AlertTriangle} title="Correct published results" description="The published copy stays in the publication history. Reopen the marks that need changing, then process, verify and publish again.">
        <div className="space-y-4">
          <TextArea label="Reason" required rows={3} value={reason} onChange={(e) => setReason(e.target.value)} />
          <div className="flex justify-end gap-2">
            <Button variant="secondary" onClick={() => setCorrecting(false)} disabled={busy}>
              Cancel
            </Button>
            <Button variant="danger" onClick={correct} loading={busy} disabled={!reason.trim()}>
              Start correction
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  )
}
