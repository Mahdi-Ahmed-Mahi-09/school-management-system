import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { Download, FileText, ListOrdered, Printer, Search, Trophy } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { examinationsApi, resultsApi } from '../../api/services'
import { GradeText, PassFailBadge } from '../../components/results/ResultBadges'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { Alert, Card, EmptyState, ErrorState, PageHeader, Skeleton } from '../../components/ui/Display'
import { Select } from '../../components/ui/Field'
import { Pagination } from '../../components/ui/Pagination'
import { SearchInput } from '../../components/ui/Table'
import { useListParams } from '../../hooks/useListParams'
import { loadSections } from '../../lib/enrollments'
import { formatGpa, formatMarks, formatPct } from '../../lib/results'
import { cn } from '../../lib/utils'

function csvCell(v) {
  let s = v == null ? '' : String(v)
  if (/^[=+\-@\t\r]/.test(s)) s = `'${s}`
  return /[",\n\r]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s
}

function downloadMerit(exam, rows) {
  const header = ['Position', 'Roll', 'Student ID', 'Student', 'Class', 'Section', 'Total', 'Full marks', 'Percentage', 'GPA', 'Grade', 'Result']
  const lines = [header, ...rows.map((m) => {
    const r = m.result
    return [m.position ?? '', r.roll_number, r.student.admission_number, r.student.full_name, r.class.name, r.section.name, r.total_obtained, r.total_full,
      r.percentage, formatGpa(r.gpa), r.overall_grade, r.passed ? 'Passed' : 'Failed']
  })]
  const blob = new Blob(['﻿' + lines.map((l) => l.map(csvCell).join(',')).join('\r\n')], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${exam?.name ?? 'results'}-merit-list.csv`.replace(/[^\w.-]+/g, '-').toLowerCase()
  document.body.appendChild(a)
  a.click()
  a.remove()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}

function ResultRow({ r, examId, position }) {
  return (
    <tr className="hover:bg-slate-50/70" data-testid="result-row">
      {position !== undefined && <td className="px-4 py-2.5 text-center font-semibold text-slate-900 tabular-nums sm:pl-6">{position ?? '—'}</td>}
      <td className={cn('px-3 py-2.5 text-right font-mono text-slate-500 tabular-nums', position === undefined && 'sm:pl-6')}>{r.roll_number}</td>
      <td className="px-3 py-2.5">
        <span className="block font-medium text-slate-900">{r.student.full_name}</span>
        <span className="block font-mono text-xs text-slate-500">{r.student.admission_number}</span>
      </td>
      <td className="px-3 py-2.5 text-slate-700">
        {r.class.name} – {r.section.name}
      </td>
      <td className="px-3 py-2.5 text-right tabular-nums">
        {formatMarks(r.total_obtained)}
        <span className="text-slate-400">/{formatMarks(r.total_full)}</span>
      </td>
      <td className="px-3 py-2.5 text-right tabular-nums">{formatPct(r.percentage)}</td>
      <td className="px-3 py-2.5 text-right font-semibold tabular-nums">{formatGpa(r.gpa)}</td>
      <td className="px-3 py-2.5 text-center">
        <GradeText grade={r.overall_grade} />
      </td>
      <td className="px-3 py-2.5">
        <PassFailBadge passed={r.passed} />
        {r.failed_subjects > 0 && <span className="ml-1.5 text-xs text-rose-600">{r.failed_subjects} subject{r.failed_subjects === 1 ? '' : 's'}</span>}
      </td>
      <td className="px-4 py-2.5 text-right sm:pr-6 print:hidden">
        <Link to={`/results/${examId}/students/${r.student.id}`} className="inline-flex items-center gap-1 text-[13px] font-medium text-brand-700 hover:underline">
          <FileText className="size-3.5" aria-hidden="true" />
          Report card
        </Link>
      </td>
    </tr>
  )
}

const HEAD = ['Roll', 'Student', 'Class & section', 'Total', '%', 'GPA', 'Grade', 'Result']

export default function ExamResultsPage() {
  const { examId } = useParams()
  const { params, updateParams, searchInput, setSearchInput, page } = useListParams()
  const tab = params.get('tab') === 'merit' ? 'merit' : 'results'
  const classId = params.get('class') ?? ''
  const sectionId = params.get('section') ?? ''
  const status = params.get('status') ?? ''
  const q = params.get('q') ?? ''
  const [exam, setExam] = useState(null)
  const [sections, setSections] = useState([])
  const [state, setState] = useState({ loading: true, error: null, rows: [], meta: null, tab: null })
  const [reloadKey, setReloadKey] = useState(0)
  const reload = useCallback(() => setReloadKey((k) => k + 1), [])

  useEffect(() => {
    examinationsApi
      .get(examId)
      .then(setExam)
      .catch(() => setExam(null))
  }, [examId])
  useEffect(() => {
    let cancelled = false
    loadSections(exam?.academic_year.id, classId)
      .then((s) => !cancelled && setSections(s))
      .catch(() => !cancelled && setSections([]))
    return () => {
      cancelled = true
    }
  }, [exam, classId])

  useEffect(() => {
    const controller = new AbortController()
    setState((s) => ({ ...s, loading: true, error: null }))
    const p = { class_id: classId || undefined, section_id: sectionId || undefined }
    const req =
      tab === 'merit'
        ? resultsApi.meritList(examId, p).then((rows) => ({ rows, meta: null }))
        : resultsApi.results(examId, { ...p, status: status || undefined, search: q || undefined, page, page_size: 50 }, { signal: controller.signal }).then(({ results, meta }) => ({ rows: results, meta }))
    req
      .then(({ rows, meta }) => setState({ loading: false, error: null, rows, meta, tab }))
      .catch((err) => !controller.signal.aborted && setState({ loading: false, error: getErrorMessage(err), rows: [], meta: null, tab }))
    return () => controller.abort()
  }, [examId, tab, classId, sectionId, status, q, page, reloadKey])

  // Rows loaded for the other tab have another shape: treat them as still loading.
  const loading = state.loading || state.tab !== tab
  const first = loading ? null : state.rows[0]
  const unpublished = Boolean(first) && !(tab === 'merit' ? first.result.published_at : first.published_at)
  const scope = sectionId ? 'Section' : classId ? 'Class' : 'Examination'

  return (
    <div className="space-y-6 animate-fade-in">
      <div className="space-y-3 print:hidden">
        <Breadcrumbs items={[{ label: 'Marks & results', to: `/results?exam=${examId}` }, { label: tab === 'merit' ? 'Merit list' : 'Results' }]} />
        <PageHeader
          title={`${tab === 'merit' ? 'Merit list' : 'Results'} · ${exam?.name ?? ''}`}
          description="Passed students rank by total marks, then GPA; equal totals and GPAs share a position (1, 2, 2, 4). Failed students get no position."
          actions={
            tab === 'merit' && (
              <>
                <Button variant="secondary" icon={Download} onClick={() => downloadMerit(exam, state.rows)} disabled={loading || !state.rows.length}>
                  CSV
                </Button>
                <Button icon={Printer} onClick={() => window.print()} disabled={loading || !state.rows.length}>
                  Print
                </Button>
              </>
            )
          }
        />
      </div>
      <div className="hidden print:block">
        <p className="text-xs tracking-wide text-slate-500 uppercase">Classora · Merit list</p>
        <h1 className="text-xl font-semibold">{exam?.name}</h1>
        <p className="text-sm text-slate-700">{exam?.academic_year.name}</p>
      </div>

      <div role="tablist" aria-label="View" className="inline-flex rounded-xl bg-slate-100 p-1 print:hidden">
        {[
          ['results', 'Results', FileText],
          ['merit', 'Merit list', ListOrdered],
        ].map(([k, label, Icon]) => (
          <button key={k} type="button" role="tab" aria-selected={tab === k} onClick={() => updateParams({ tab: k === 'results' ? '' : k })}
            className={cn('inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm font-medium', tab === k ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-600')}>
            <Icon className="size-4" aria-hidden="true" />
            {label}
          </button>
        ))}
      </div>

      <Card className="p-4 sm:p-5 print:hidden">
        <div className="grid gap-3 sm:grid-cols-3">
          <Select label="Class" value={classId} onChange={(e) => updateParams({ class: e.target.value, section: '' })}>
            <option value="">All classes</option>
            {(exam?.classes ?? []).map((c) => (
              <option key={c.class.id} value={c.class.id}>
                {c.class.name}
              </option>
            ))}
          </Select>
          <Select label="Section" value={sectionId} disabled={!classId} onChange={(e) => updateParams({ section: e.target.value })}>
            <option value="">{classId ? 'All sections' : 'Choose a class first'}</option>
            {sections.map((s) => (
              <option key={s.id} value={s.id}>
                Section {s.name}
              </option>
            ))}
          </Select>
          {tab === 'results' && (
            <Select label="Result" value={status} onChange={(e) => updateParams({ status: e.target.value })}>
              <option value="">Passed and failed</option>
              <option value="passed">Passed</option>
              <option value="failed">Failed</option>
            </Select>
          )}
        </div>
      </Card>

      {unpublished && (
        <Alert tone="warning" title="Not published" className="print:hidden">
          These results are processed but not published yet. Only administrators can see them.
        </Alert>
      )}

      <Card className="overflow-hidden print:border-0 print:shadow-none">
        {tab === 'results' && (
          <div className="border-b border-slate-100 p-4 sm:px-6 print:hidden">
            <SearchInput icon={Search} value={searchInput} onChange={setSearchInput} placeholder="Search by student name or ID" label="Search results" />
          </div>
        )}
        {state.error ? (
          <ErrorState title="Couldn't load the results" message={state.error} onRetry={reload} />
        ) : loading ? (
          <Skeleton className="m-5 h-40" />
        ) : state.rows.length === 0 ? (
          <EmptyState icon={Trophy} title="No results" description="Results appear once the overall results are processed." />
        ) : (
          <>
            <div className="overflow-x-auto">
              <table className="min-w-full text-sm">
                <thead className="bg-slate-50/70 text-xs font-medium text-slate-500">
                  <tr className="border-b border-slate-100">
                    {tab === 'merit' && <th className="px-4 py-2.5 text-center sm:pl-6">{scope} position</th>}
                    {HEAD.map((h, i) => (
                      <th key={h} className={cn('px-3 py-2.5', ['Roll', 'Total', '%', 'GPA'].includes(h) ? 'text-right' : h === 'Grade' ? 'text-center' : 'text-left', i === 0 && tab !== 'merit' && 'sm:pl-6')}>
                        {h}
                      </th>
                    ))}
                    <th className="px-4 py-2.5 sm:pr-6 print:hidden">
                      <span className="sr-only">Report card</span>
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {tab === 'merit'
                    ? state.rows.map((m) => <ResultRow key={m.result.id} r={m.result} examId={examId} position={m.position} />)
                    : state.rows.map((r) => <ResultRow key={r.id} r={r} examId={examId} />)}
                </tbody>
              </table>
            </div>
            {state.meta && (
              <Pagination page={state.meta.page} pageSize={state.meta.page_size} total={state.meta.total} totalPages={state.meta.total_pages}
                onPageChange={(p) => updateParams({ page: p }, { resetPage: false })} />
            )}
          </>
        )}
      </Card>
    </div>
  )
}
