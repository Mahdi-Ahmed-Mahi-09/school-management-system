import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ClipboardList, Eye, Lock, ShieldCheck } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { resultsApi } from '../../api/services'
import { Progress, SheetStatusBadge } from '../../components/results/ResultBadges'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Card, EmptyState, ErrorState, PageHeader, Skeleton } from '../../components/ui/Display'
import { Select } from '../../components/ui/Field'
import { useToast } from '../../components/ui/Toast'
import { SHEET_STATUS_META } from '../../lib/results'

/** Every paper × section of an examination with its marks status (admins). */
export default function MarksReviewPage() {
  const { examId } = useParams()
  const toast = useToast()
  const [state, setState] = useState({ loading: true, error: null, items: [] })
  const [filter, setFilter] = useState('')
  const [busy, setBusy] = useState(null)
  const [reloadKey, setReloadKey] = useState(0)
  const reload = useCallback(() => setReloadKey((k) => k + 1), [])

  useEffect(() => {
    let cancelled = false
    resultsApi
      .sheets(examId)
      .then((items) => !cancelled && setState({ loading: false, error: null, items }))
      .catch((err) => !cancelled && setState({ loading: false, error: getErrorMessage(err), items: [] }))
    return () => {
      cancelled = true
    }
  }, [examId, reloadKey])

  const statusOf = (s) => (s.started ? s.status : 'not_started')
  const items = state.items.filter((s) => !filter || statusOf(s) === filter)
  const counts = {}
  for (const s of state.items) counts[statusOf(s)] = (counts[statusOf(s)] ?? 0) + 1

  const act = async (s, action) => {
    const key = `${s.paper.schedule_id}-${s.section.id}`
    setBusy(key)
    try {
      await resultsApi[action](s.paper.schedule_id, s.section.id)
      toast.success(action === 'verify' ? 'Marks verified' : 'Marks finalized', `${s.paper.class.name} – ${s.paper.subject.name}, Section ${s.section.name}`)
      reload()
    } catch (err) {
      toast.error('Action failed', getErrorMessage(err))
    } finally {
      setBusy(null)
    }
  }

  return (
    <div className="space-y-6 animate-fade-in">
      <div className="space-y-3">
        <Breadcrumbs items={[{ label: 'Marks & results', to: `/results?exam=${examId}` }, { label: 'Marks review' }]} />
        <PageHeader title="Marks review" description="Verify submitted marks, then finalize them. Open a sheet to see every student or return it for correction." />
      </div>
      <Card className="p-4 sm:p-5">
        <Select label="Status" className="sm:w-72" value={filter} onChange={(e) => setFilter(e.target.value)}>
          <option value="">All ({state.items.length})</option>
          {Object.entries(SHEET_STATUS_META).map(([k, m]) => (
            <option key={k} value={k}>
              {m.label} ({counts[k] ?? 0})
            </option>
          ))}
        </Select>
      </Card>
      <Card className="overflow-hidden">
        {state.error ? (
          <ErrorState title="Couldn't load the marks" message={state.error} onRetry={reload} />
        ) : state.loading ? (
          <Skeleton className="m-5 h-40" />
        ) : items.length === 0 ? (
          <EmptyState icon={ClipboardList} title="Nothing here" description="No mark sheets match this status." />
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full text-sm">
              <thead className="bg-slate-50/70 text-xs font-medium text-slate-500">
                <tr className="border-b border-slate-100">
                  <th className="px-4 py-2.5 text-left sm:pl-6">Class & section</th>
                  <th className="px-3 py-2.5 text-left">Subject</th>
                  <th className="px-3 py-2.5 text-left">Marked</th>
                  <th className="px-3 py-2.5 text-left">Status</th>
                  <th className="px-4 py-2.5 sm:pr-6">
                    <span className="sr-only">Actions</span>
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {items.map((s) => {
                  const key = `${s.paper.schedule_id}-${s.section.id}`
                  return (
                    <tr key={key} data-testid="review-row">
                      <td className="px-4 py-2.5 sm:pl-6">
                        {s.paper.class.name} – {s.section.name}
                      </td>
                      <td className="px-3 py-2.5 font-medium text-slate-900">{s.paper.subject.name}</td>
                      <td className="px-3 py-2.5">
                        <Progress value={s.marked} total={s.students} />
                      </td>
                      <td className="px-3 py-2.5">
                        <SheetStatusBadge status={s.status} started={s.started} />
                      </td>
                      <td className="px-4 py-2.5 text-right whitespace-nowrap sm:pr-6">
                        {s.started && s.status === 'submitted' && (
                          <Button size="sm" variant="secondary" icon={ShieldCheck} loading={busy === key} onClick={() => act(s, 'verify')}>
                            Verify
                          </Button>
                        )}
                        {s.started && s.status === 'verified' && (
                          <Button size="sm" variant="secondary" icon={Lock} loading={busy === key} onClick={() => act(s, 'finalize')}>
                            Finalize
                          </Button>
                        )}
                        <Link to={`/marks/${s.paper.schedule_id}?section=${s.section.id}`} className={buttonClasses('ghost', 'sm')}>
                          <Eye className="size-4" aria-hidden="true" />
                          Open
                        </Link>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </div>
  )
}
