import { useCallback, useEffect, useRef, useState } from 'react'
import { getErrorMessage } from '../../api/client'
import { dashboardApi } from '../../api/services'
import { AcademicOverview } from '../../components/dashboard/AcademicOverview'
import { AccountGrowthChart, AccountsByRoleChart } from '../../components/dashboard/AccountCharts'
import { AttendanceOverview } from '../../components/dashboard/AttendanceOverview'
import { DashboardHeader } from '../../components/dashboard/DashboardHeader'
import { ExamOverview } from '../../components/dashboard/ExamOverview'
import { DashboardSkeleton } from '../../components/dashboard/DashboardSkeleton'
import { FeeCollection } from '../../components/dashboard/FeeCollection'
import { KpiCards } from '../../components/dashboard/KpiCards'
import { QuickActions } from '../../components/dashboard/QuickActions'
import { RecentActivity, RecentAdmissions, RecentNotices } from '../../components/dashboard/RecentLists'
import { Card, ErrorState } from '../../components/ui/Display'
import { useToast } from '../../components/ui/Toast'

/**
 * Admin dashboard. Every number comes from GET /admin/dashboard; sections
 * whose module isn't implemented yet are rendered as "not available yet"
 * rather than as zero.
 */
export default function AdminDashboardPage() {
  const toast = useToast()
  const [state, setState] = useState({ status: 'loading', data: null, error: null })
  const [refreshing, setRefreshing] = useState(false)
  const inFlight = useRef(false)

  const load = useCallback(
    async ({ manual = false } = {}) => {
      // Avoid duplicate concurrent requests (e.g. repeated refresh clicks).
      if (inFlight.current) return
      inFlight.current = true
      if (manual) setRefreshing(true)
      try {
        const data = await dashboardApi.admin()
        setState({ status: 'ready', data, error: null })
        if (manual) toast.success('Dashboard updated', 'Showing the latest figures.')
      } catch (err) {
        const message = getErrorMessage(err)
        setState((s) => (s.data ? s : { status: 'error', data: null, error: message }))
        if (manual) toast.error("Couldn't refresh the dashboard", message)
      } finally {
        inFlight.current = false
        setRefreshing(false)
      }
    },
    [toast],
  )

  useEffect(() => {
    load()
  }, [load])

  const retry = () => {
    setState({ status: 'loading', data: null, error: null })
    load()
  }

  const { status, data, error } = state

  return (
    <div className="space-y-6 animate-fade-in">
      <DashboardHeader
        generatedAt={data?.generated_at}
        refreshing={refreshing}
        disabled={status !== 'ready'}
        onRefresh={() => load({ manual: true })}
      />

      {status === 'loading' && <DashboardSkeleton />}

      {status === 'error' && (
        <Card>
          <ErrorState title="Couldn't load the dashboard" message={error} onRetry={retry} />
        </Card>
      )}

      {status === 'ready' && (
        <div className="space-y-6">
          <KpiCards overview={data.overview} />

          <div className="grid gap-6 lg:grid-cols-3">
            <AccountGrowthChart section={data.account_growth} className="lg:col-span-2" />
            <AccountsByRoleChart section={data.accounts} />
          </div>

          <div className="grid gap-6 lg:grid-cols-3">
            <AttendanceOverview section={data.attendance} />
            <FeeCollection section={data.fees} className="lg:col-span-2" />
          </div>

          <div className="grid gap-6 lg:grid-cols-3">
            <AcademicOverview academic={data.academic} className="lg:col-span-2" />
            <QuickActions modules={data.modules} />
          </div>

          <div className="grid gap-6 lg:grid-cols-2">
            <ExamOverview section={data.examinations} />
            <RecentActivity section={data.recent_activity} />
          </div>

          <div className="grid gap-6 lg:grid-cols-2">
            <RecentAdmissions section={data.recent_admissions} />
            <RecentNotices section={data.recent_notices} />
          </div>
        </div>
      )}
    </div>
  )
}
