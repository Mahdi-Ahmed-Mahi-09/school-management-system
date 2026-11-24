import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { ArrowRight, History, Sparkles } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { promotionsApi } from '../../api/services'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Card, EmptyState, ErrorState, PageHeader, Skeleton } from '../../components/ui/Display'
import { Pagination } from '../../components/ui/Pagination'
import { useAuth } from '../../hooks/useAuth'
import { PERMISSIONS } from '../../lib/roles'
import { formatDateTime } from '../../lib/utils'

const PAGE_SIZE = 20

function Count({ n, label, className }) {
  if (!n) return null
  return (
    <span className={className}>
      {n} {label}
    </span>
  )
}

export default function PromotionsPage() {
  const { can } = useAuth()
  const [page, setPage] = useState(1)
  const [state, setState] = useState({ loading: true, error: null, batches: [], meta: null })
  const [reloadKey, setReloadKey] = useState(0)
  const reload = useCallback(() => setReloadKey((k) => k + 1), [])

  useEffect(() => {
    const controller = new AbortController()
    setState((s) => ({ ...s, loading: true, error: null }))
    promotionsApi
      .list({ page, page_size: PAGE_SIZE }, { signal: controller.signal })
      .then(({ batches, meta }) => setState({ loading: false, error: null, batches, meta }))
      .catch((err) => !controller.signal.aborted && setState({ loading: false, error: getErrorMessage(err), batches: [], meta: null }))
    return () => controller.abort()
  }, [page, reloadKey])

  const newButton = can(PERMISSIONS.PROMOTIONS_EXECUTE) && (
    <Link to="/promotions/new" className={buttonClasses('primary')}>
      <Sparkles className="size-4" aria-hidden="true" />
      Promote students
    </Link>
  )

  return (
    <div className="space-y-6 animate-fade-in">
      <PageHeader title="Promotions" description="Every confirmed promotion, with what happened to each student." actions={newButton} />
      <Card className="overflow-hidden">
        {state.error ? (
          <ErrorState title="Couldn't load promotion history" message={state.error} onRetry={reload} />
        ) : state.loading && !state.batches.length ? (
          <div className="space-y-3 p-6">
            {[0, 1, 2].map((i) => (
              <Skeleton key={i} className="h-12" />
            ))}
          </div>
        ) : state.batches.length === 0 ? (
          <EmptyState
            icon={History}
            title="No promotions yet"
            description="At the end of an academic year, promote students into the next one. Each run is recorded here."
            action={newButton}
          />
        ) : (
          <>
            <ul className="divide-y divide-slate-100">
              {state.batches.map((b) => (
                <li key={b.id}>
                  <Link to={`/promotions/${b.id}`} className="flex flex-col gap-2 px-5 py-4 transition-colors hover:bg-slate-50/70 sm:flex-row sm:items-center sm:px-6">
                    <div className="min-w-0 flex-1">
                      <p className="flex flex-wrap items-center gap-2 text-sm font-semibold text-slate-900">
                        {b.source_academic_year.name}
                        <ArrowRight className="size-4 text-slate-400" aria-hidden="true" />
                        {b.target_academic_year.name}
                      </p>
                      <p className="mt-0.5 text-[13px] text-slate-500">
                        {formatDateTime(b.processed_at)}
                        {b.processed_by?.name ? ` · by ${b.processed_by.name}` : ''} · {b.total_students} student{b.total_students === 1 ? '' : 's'}
                      </p>
                    </div>
                    <div className="flex flex-wrap gap-x-3 gap-y-1 text-[13px] font-medium">
                      <Count n={b.promoted_count} label="promoted" className="text-brand-700" />
                      <Count n={b.repeated_count} label="repeating" className="text-amber-700" />
                      <Count n={b.completed_count} label="completed" className="text-violet-700" />
                      <Count n={b.excluded_count} label="excluded" className="text-slate-500" />
                      <Count n={b.skipped_count} label="not eligible" className="text-rose-700" />
                    </div>
                  </Link>
                </li>
              ))}
            </ul>
            {state.meta && (
              <Pagination page={state.meta.page} pageSize={state.meta.page_size} total={state.meta.total} totalPages={state.meta.total_pages} onPageChange={setPage} />
            )}
          </>
        )}
      </Card>
    </div>
  )
}
