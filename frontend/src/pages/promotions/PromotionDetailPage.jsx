import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ArrowRight, SearchX } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { promotionsApi } from '../../api/services'
import { DecisionBadge, OutcomeBadge } from '../../components/enrollments/EnrollmentBadges'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Card, EmptyState, ErrorState, Skeleton } from '../../components/ui/Display'
import { PersonAvatar } from '../../components/ui/PersonAvatar'
import { cn, formatDateTime, formatNumber } from '../../lib/utils'

function Tile({ label, value, tone = 'text-slate-900' }) {
  return (
    <Card className="p-4">
      <p className="text-xs font-medium text-slate-500">{label}</p>
      <p className={cn('mt-1 text-2xl font-semibold tabular-nums', value ? tone : 'text-slate-400')}>{formatNumber(value)}</p>
    </Card>
  )
}

export default function PromotionDetailPage() {
  const { id } = useParams()
  const [state, setState] = useState({ loading: true, error: null, notFound: false, batch: null })
  const [filter, setFilter] = useState('')

  const load = useCallback(async () => {
    setState((s) => ({ ...s, loading: true, error: null }))
    try {
      setState({ loading: false, error: null, notFound: false, batch: await promotionsApi.get(id) })
    } catch (err) {
      setState({ loading: false, error: getErrorMessage(err), notFound: err?.response?.status === 404, batch: null })
    }
  }, [id])
  useEffect(() => {
    load()
  }, [load])

  if (state.loading && !state.batch) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-4 w-56" />
        <Skeleton className="h-24 rounded-xl" />
        <Skeleton className="h-72 rounded-xl" />
      </div>
    )
  }
  if (state.notFound || state.error) {
    return (
      <div className="space-y-6">
        <Breadcrumbs items={[{ label: 'Promotions', to: '/promotions' }, { label: state.notFound ? 'Not found' : 'Error' }]} />
        <Card>
          {state.notFound ? (
            <EmptyState
              icon={SearchX}
              title="Promotion not found"
              action={
                <Link to="/promotions" className={buttonClasses('secondary', 'sm')}>
                  Back to promotions
                </Link>
              }
            />
          ) : (
            <ErrorState title="Couldn't load this promotion" message={state.error} onRetry={load} />
          )}
        </Card>
      </div>
    )
  }

  const b = state.batch
  const items = b.items.filter((i) =>
    !filter ? true : filter === 'excluded' || filter === 'skipped' ? i.status === filter : i.decision === filter && i.status === 'processed',
  )

  return (
    <div className="space-y-6 animate-fade-in">
      <Breadcrumbs items={[{ label: 'Promotions', to: '/promotions' }, { label: `${b.source_academic_year.name} → ${b.target_academic_year.name}` }]} />
      <Card className="p-5 sm:p-6">
        <h1 className="flex flex-wrap items-center gap-2 text-2xl font-semibold tracking-tight text-slate-900">
          {b.source_academic_year.name}
          <ArrowRight className="size-5 text-slate-400" aria-hidden="true" />
          {b.target_academic_year.name}
        </h1>
        <p className="mt-1 text-[15px] text-slate-600">
          Processed {formatDateTime(b.processed_at)}
          {b.processed_by?.name ? ` by ${b.processed_by.name}` : ''} · roll numbers {b.roll_strategy === 'keep' ? 'kept where free' : 'renumbered'}
        </p>
        {b.remarks && <p className="mt-2 text-sm text-slate-600">{b.remarks}</p>}
      </Card>

      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
        <Tile label="Students" value={b.total_students} />
        <Tile label="Promoted" value={b.promoted_count} tone="text-brand-700" />
        <Tile label="Repeating" value={b.repeated_count} tone="text-amber-700" />
        <Tile label="Completed" value={b.completed_count} tone="text-violet-700" />
        <Tile label="Excluded" value={b.excluded_count} tone="text-slate-500" />
        <Tile label="Not eligible" value={b.skipped_count} tone="text-rose-700" />
      </div>

      <Card className="overflow-hidden">
        <div className="flex flex-wrap items-center gap-3 border-b border-slate-100 px-5 py-3.5 sm:px-6">
          <h2 className="flex-1 text-sm font-semibold text-slate-900">Student decisions</h2>
          <select aria-label="Filter decisions" value={filter} onChange={(e) => setFilter(e.target.value)} className="h-8 rounded-lg border border-slate-300 bg-white px-2 text-[13px]">
            <option value="">All students</option>
            <option value="promote">Promoted</option>
            <option value="repeat">Repeating</option>
            <option value="complete">Completed</option>
            <option value="excluded">Excluded</option>
            <option value="skipped">Not eligible</option>
          </select>
        </div>
        <div className="overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead className="bg-slate-50/70 text-xs font-medium text-slate-500">
              <tr className="border-b border-slate-100">
                <th className="px-4 py-3 text-left sm:pl-6">Student</th>
                <th className="px-4 py-3 text-left">From ({b.source_academic_year.name})</th>
                <th className="px-4 py-3 text-left">Decision</th>
                <th className="px-4 py-3 text-left">To ({b.target_academic_year.name})</th>
                <th className="px-4 py-3 text-left sm:pr-6">Result</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {items.map((it) => (
                <tr key={it.id}>
                  <td className="px-4 py-2.5 sm:pl-6">
                    <span className="flex items-center gap-3">
                      <PersonAvatar person={it.student} size="sm" />
                      <span>
                        <span className="block font-medium text-slate-900">{it.student.full_name}</span>
                        <span className="block font-mono text-xs text-slate-500">{it.student.admission_number}</span>
                      </span>
                    </span>
                  </td>
                  <td className="px-4 py-2.5 whitespace-nowrap">
                    <Link to={`/enrollments/${it.previous.enrollment_id}`} className="text-slate-700 hover:text-brand-700">
                      {it.previous.class.name} – {it.previous.section.name} · {it.previous.roll_number}
                    </Link>
                  </td>
                  <td className="px-4 py-2.5">
                    <DecisionBadge decision={it.decision} />
                  </td>
                  <td className="px-4 py-2.5 whitespace-nowrap">
                    {it.new ? (
                      <Link to={`/enrollments/${it.new.enrollment_id}`} className="text-slate-700 hover:text-brand-700">
                        {it.new.class.name} – {it.new.section.name} · {it.new.roll_number}
                      </Link>
                    ) : (
                      <span className="text-slate-400">—</span>
                    )}
                  </td>
                  <td className="px-4 py-2.5 sm:pr-6">
                    <OutcomeBadge outcome={it.status} done />
                    {it.message && <span className="mt-0.5 block text-xs text-slate-500">{it.message}</span>}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>
    </div>
  )
}
