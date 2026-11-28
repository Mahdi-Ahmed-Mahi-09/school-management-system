import { useCallback, useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { ArrowRight, FileSpreadsheet, NotebookPen } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { resultsApi } from '../../api/services'
import { Progress, SheetStatusBadge } from '../../components/results/ResultBadges'
import { Card, EmptyState, ErrorState, PageHeader, Skeleton } from '../../components/ui/Display'
import { Select } from '../../components/ui/Field'
import { formatDateOnly } from '../../lib/students'

/** Pick an examination, then a paper and section to enter marks for. */
export default function MarksHomePage() {
  const [params, setParams] = useSearchParams()
  const [exams, setExams] = useState({ loading: true, error: null, items: [], note: '' })
  const [sheets, setSheets] = useState({ loading: false, error: null, items: [] })
  const [reloadKey, setReloadKey] = useState(0)
  const reload = useCallback(() => setReloadKey((k) => k + 1), [])

  useEffect(() => {
    resultsApi
      .markExams()
      .then(({ exams: items, note }) => setExams({ loading: false, error: null, items, note: note ?? '' }))
      .catch((err) => setExams({ loading: false, error: getErrorMessage(err), items: [], note: '' }))
  }, [])

  const examId = params.get('exam') || exams.items[0]?.id || ''
  useEffect(() => {
    if (!examId) return
    let cancelled = false
    setSheets((s) => ({ ...s, loading: true, error: null }))
    resultsApi
      .sheets(examId)
      .then((items) => !cancelled && setSheets({ loading: false, error: null, items }))
      .catch((err) => !cancelled && setSheets({ loading: false, error: getErrorMessage(err), items: [] }))
    return () => {
      cancelled = true
    }
  }, [examId, reloadKey])

  // Group sheets by class for a readable list.
  const groups = []
  for (const s of sheets.items) {
    let g = groups.find((x) => x.class.id === s.paper.class.id)
    if (!g) groups.push((g = { class: s.paper.class, items: [] }))
    g.items.push(s)
  }

  return (
    <div className="space-y-6 animate-fade-in">
      <PageHeader title="Marks entry" description="Choose a paper and section, enter every student's marks, then submit them for verification." />
      {exams.error ? (
        <Card>
          <ErrorState title="Couldn't load examinations" message={exams.error} />
        </Card>
      ) : exams.loading ? (
        <Skeleton className="h-40" />
      ) : exams.items.length === 0 ? (
        <Card>
          <EmptyState icon={NotebookPen} title="No marks to enter" description={exams.note || 'Marks can be entered once an examination timetable is published.'} />
        </Card>
      ) : (
        <>
          <Card className="p-4 sm:p-5">
            <Select label="Examination" className="sm:w-96" value={examId} onChange={(e) => setParams({ exam: e.target.value }, { replace: true })}>
              {exams.items.map((x) => (
                <option key={x.id} value={x.id}>
                  {x.name} · {x.academic_year.name}
                </option>
              ))}
            </Select>
          </Card>
          <Card className="overflow-hidden">
            {sheets.error ? (
              <ErrorState title="Couldn't load the papers" message={sheets.error} onRetry={reload} />
            ) : sheets.loading ? (
              <div className="space-y-2 p-5">
                {[0, 1, 2].map((i) => (
                  <Skeleton key={i} className="h-12" />
                ))}
              </div>
            ) : groups.length === 0 ? (
              <EmptyState icon={FileSpreadsheet} title="No papers for you here" description="Only the subjects and sections you teach are listed." />
            ) : (
              groups.map((g) => (
                <section key={g.class.id} className="border-b border-slate-100 last:border-0">
                  <h2 className="bg-slate-50/70 px-5 py-2 text-xs font-semibold tracking-wide text-slate-500 uppercase sm:px-6">{g.class.name}</h2>
                  <ul className="divide-y divide-slate-100">
                    {g.items.map((s) => (
                      <li key={`${s.paper.schedule_id}-${s.section.id}`}>
                        <Link
                          to={`/marks/${s.paper.schedule_id}?section=${s.section.id}`}
                          className="flex flex-col gap-2 px-5 py-3 hover:bg-slate-50 sm:flex-row sm:items-center sm:px-6"
                          data-testid="sheet-link"
                        >
                          <span className="min-w-0 flex-1">
                            <span className="block text-sm font-medium text-slate-900">
                              {s.paper.subject.name} · Section {s.section.name}
                            </span>
                            <span className="block text-xs text-slate-500">
                              {formatDateOnly(s.paper.exam_date)} · {s.students} student{s.students === 1 ? '' : 's'}
                              {s.absent > 0 && ` · ${s.absent} absent`}
                            </span>
                          </span>
                          <Progress value={s.marked} total={s.students} />
                          <SheetStatusBadge status={s.status} started={s.started} />
                          <ArrowRight className="hidden size-4 text-slate-300 sm:block" aria-hidden="true" />
                        </Link>
                      </li>
                    ))}
                  </ul>
                </section>
              ))
            )}
          </Card>
        </>
      )}
    </div>
  )
}
