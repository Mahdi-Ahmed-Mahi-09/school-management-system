import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { Printer } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { resultsApi } from '../../api/services'
import { GradeText, PassFailBadge } from '../../components/results/ResultBadges'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { Alert, Card, ErrorState, Skeleton } from '../../components/ui/Display'
import { formatGpa, formatMarks, formatPct } from '../../lib/results'
import { formatDateOnly } from '../../lib/students'
import { formatDateTime } from '../../lib/utils'

/** Printable report card of one student in one examination. */
export default function ReportCardPage() {
  const { examId, studentId } = useParams()
  const [state, setState] = useState({ loading: true, error: null, card: null })

  useEffect(() => {
    let cancelled = false
    resultsApi
      .reportCard(examId, studentId)
      .then((card) => !cancelled && setState({ loading: false, error: null, card }))
      .catch((err) => !cancelled && setState({ loading: false, error: getErrorMessage(err), card: null }))
    return () => {
      cancelled = true
    }
  }, [examId, studentId])

  const crumbs = [{ label: 'Marks & results', to: `/results?exam=${examId}` }, { label: 'Results', to: `/results/${examId}/results` }, { label: 'Report card' }]
  if (state.error) {
    return (
      <div className="space-y-6">
        <Breadcrumbs items={crumbs} />
        <Card>
          <ErrorState title="No report card" message={state.error} />
        </Card>
      </div>
    )
  }
  if (state.loading) return <Skeleton className="h-96" />
  const { examination: x, result: r, published } = state.card

  return (
    <div className="space-y-6 animate-fade-in">
      <div className="flex flex-wrap items-center justify-between gap-3 print:hidden">
        <Breadcrumbs items={crumbs} />
        <Button icon={Printer} onClick={() => window.print()}>
          Print report card
        </Button>
      </div>
      {!published && (
        <Alert tone="warning" title="Not published" className="print:hidden">
          This result isn&apos;t published yet. It is visible to administrators only and may still change.
        </Alert>
      )}

      <Card className="mx-auto max-w-3xl overflow-hidden print:max-w-none print:border-slate-400 print:shadow-none" data-testid="report-card">
        <div className="border-b border-slate-200 px-6 py-5 text-center">
          <p className="text-xs tracking-[0.2em] text-slate-500 uppercase">Academic report card</p>
          <h1 className="mt-1 text-xl font-semibold text-slate-900">{x.name}</h1>
          <p className="text-sm text-slate-600">
            {x.exam_type.name} · Academic year {x.academic_year.name} · {formatDateOnly(x.start_date)} – {formatDateOnly(x.end_date)}
          </p>
        </div>
        <dl className="grid grid-cols-2 gap-x-6 gap-y-3 px-6 py-4 text-sm sm:grid-cols-3">
          {[
            ['Student', r.student.full_name],
            ['Student ID', r.student.admission_number],
            ['Roll number', r.roll_number],
            ['Class', r.class.name],
            ['Section', r.section.name],
            ['Class position', r.class_position ?? '—'],
          ].map(([k, v]) => (
            <div key={k}>
              <dt className="text-xs text-slate-500">{k}</dt>
              <dd className="font-medium text-slate-900">{v}</dd>
            </div>
          ))}
        </dl>
        <table className="min-w-full border-t border-slate-200 text-sm">
          <thead className="bg-slate-50 text-xs font-medium text-slate-500 print:bg-transparent">
            <tr>
              <th className="px-6 py-2 text-left">Subject</th>
              <th className="px-3 py-2 text-right">Full marks</th>
              <th className="px-3 py-2 text-right">Obtained</th>
              <th className="px-3 py-2 text-center">Grade</th>
              <th className="px-6 py-2 text-right">Grade point</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {r.subject_results.map((s) => (
              <tr key={s.id}>
                <td className="px-6 py-2 text-slate-900">
                  {s.subject_name}
                  <span className="ml-1.5 font-mono text-xs text-slate-400">{s.subject_code}</span>
                </td>
                <td className="px-3 py-2 text-right tabular-nums">{formatMarks(s.full_marks)}</td>
                <td className="px-3 py-2 text-right tabular-nums">{s.absent ? <span className="font-medium text-rose-700">Absent</span> : formatMarks(s.obtained_marks)}</td>
                <td className="px-3 py-2 text-center">
                  <GradeText grade={s.letter_grade} />
                </td>
                <td className="px-6 py-2 text-right tabular-nums">{formatGpa(s.grade_point)}</td>
              </tr>
            ))}
          </tbody>
        </table>
        <dl className="grid grid-cols-2 gap-px border-t border-slate-200 bg-slate-100 sm:grid-cols-4">
          {[
            ['Total marks', `${formatMarks(r.total_obtained)} / ${formatMarks(r.total_full)}`],
            ['Percentage', formatPct(r.percentage)],
            ['GPA', formatGpa(r.gpa)],
            ['Overall grade', <GradeText key="g" grade={r.overall_grade} className="text-lg" />],
          ].map(([k, v]) => (
            <div key={k} className="bg-white px-6 py-3">
              <dt className="text-xs text-slate-500">{k}</dt>
              <dd className="text-lg font-semibold text-slate-900 tabular-nums">{v}</dd>
            </div>
          ))}
        </dl>
        <div className="flex flex-wrap items-center justify-between gap-3 border-t border-slate-200 px-6 py-4 text-sm">
          <span className="flex items-center gap-2">
            Result: <PassFailBadge passed={r.passed} />
            {r.failed_subjects > 0 && <span className="text-xs text-rose-700">Failed in {r.failed_subjects} subject{r.failed_subjects === 1 ? '' : 's'} — GPA 0.00 by rule.</span>}
          </span>
          <span className="text-xs text-slate-500">{published ? `Published ${formatDateTime(r.published_at)}` : 'Not published'}</span>
        </div>
      </Card>
    </div>
  )
}
