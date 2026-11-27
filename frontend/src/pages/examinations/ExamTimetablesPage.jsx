import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { ArrowRight, CalendarDays } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { examinationsApi } from '../../api/services'
import { ExamStatusBadge } from '../../components/examinations/ExamStatusBadge'
import { Card, EmptyState, ErrorState, PageHeader, Skeleton } from '../../components/ui/Display'
import { Select } from '../../components/ui/Field'
import { Pagination } from '../../components/ui/Pagination'
import { useAuth } from '../../hooks/useAuth'
import { useListParams } from '../../hooks/useListParams'
import { PERMISSIONS } from '../../lib/roles'
import { formatDateOnly } from '../../lib/students'

/** Published and past examination timetables the user can see (read-only). */
export default function ExamTimetablesPage() {
  const { can } = useAuth()
  const { params, updateParams, page } = useListParams()
  const status = params.get('status') ?? ''
  const [list, setList] = useState({ loading: true, error: null, exams: [], meta: null })
  const [reloadKey, setReloadKey] = useState(0)
  const reload = useCallback(() => setReloadKey((k) => k + 1), [])

  useEffect(() => {
    const controller = new AbortController()
    setList((s) => ({ ...s, loading: true, error: null }))
    examinationsApi
      .timetables({ status: status || undefined, page, page_size: 20 }, { signal: controller.signal })
      .then(({ exams, meta }) => setList({ loading: false, error: null, exams, meta }))
      .catch((err) => !controller.signal.aborted && setList({ loading: false, error: getErrorMessage(err), exams: [], meta: null }))
    return () => controller.abort()
  }, [status, page, reloadKey])

  const teacher = !can(PERMISSIONS.EXAMS_READ)
  return (
    <div className="space-y-6 animate-fade-in">
      <PageHeader
        title="Exam timetables"
        description={teacher ? 'Published examination timetables of the classes you teach. Papers of your subjects are highlighted.' : 'Published and past examination timetables.'}
      />
      <Card className="p-4 sm:p-5">
        <Select label="Status" className="sm:w-64" value={status} onChange={(e) => updateParams({ status: e.target.value })}>
          <option value="">Published, completed and cancelled</option>
          <option value="published">Published</option>
          <option value="completed">Completed</option>
          <option value="cancelled">Cancelled</option>
        </Select>
      </Card>
      <Card className="overflow-hidden">
        {list.error ? (
          <ErrorState title="Couldn't load timetables" message={list.error} onRetry={reload} />
        ) : list.loading ? (
          <div className="space-y-2 p-5">
            {[0, 1, 2].map((i) => (
              <Skeleton key={i} className="h-12" />
            ))}
          </div>
        ) : list.exams.length === 0 ? (
          <EmptyState icon={CalendarDays} title="No timetables yet" description={list.meta?.note || 'Timetables appear here once an examination is published.'} />
        ) : (
          <>
            <ul className="divide-y divide-slate-100">
              {list.exams.map((x) => (
                <li key={x.id}>
                  <Link to={`/examinations/${x.id}/timetable`} className="flex items-center gap-4 px-5 py-3.5 hover:bg-slate-50 sm:px-6">
                    <span className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-brand-50 text-brand-600">
                      <CalendarDays className="size-5" aria-hidden="true" />
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-sm font-semibold text-slate-900">{x.name}</span>
                      <span className="block text-[13px] text-slate-500">
                        {x.exam_type.name} · {x.academic_year.name} · {formatDateOnly(x.start_date)} – {formatDateOnly(x.end_date)}
                      </span>
                    </span>
                    <ExamStatusBadge status={x.status} />
                    <ArrowRight className="size-4 text-slate-300" aria-hidden="true" />
                  </Link>
                </li>
              ))}
            </ul>
            {list.meta && (
              <Pagination page={list.meta.page} pageSize={list.meta.page_size} total={list.meta.total} totalPages={list.meta.total_pages} onPageChange={(p) => updateParams({ page: p }, { resetPage: false })} />
            )}
          </>
        )}
      </Card>
    </div>
  )
}
