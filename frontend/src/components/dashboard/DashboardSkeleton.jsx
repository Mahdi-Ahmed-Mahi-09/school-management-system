import { Card, Skeleton } from '../ui/Display'

function PanelSkeleton({ className, bodyHeight = 'h-56' }) {
  return (
    <Card className={className}>
      <div className="flex items-center gap-3 px-6 pt-5">
        <Skeleton className="size-8 rounded-lg" />
        <div className="space-y-1.5">
          <Skeleton className="h-3.5 w-32" />
          <Skeleton className="h-3 w-48" />
        </div>
      </div>
      <div className="px-6 pt-4 pb-5">
        <Skeleton className={`w-full rounded-lg ${bodyHeight}`} />
      </div>
    </Card>
  )
}

/** Mirrors the dashboard grid so content doesn't jump when data arrives. */
export function DashboardSkeleton() {
  return (
    <div className="space-y-6" role="status" aria-label="Loading dashboard">
      <div className="grid grid-cols-2 gap-3 sm:gap-4 md:grid-cols-3 xl:grid-cols-5">
        {Array.from({ length: 5 }, (_, i) => (
          <Card key={i} className="p-5">
            <Skeleton className="size-9 rounded-lg" />
            <Skeleton className="mt-4 h-3.5 w-24" />
            <Skeleton className="mt-2 h-7 w-14" />
            <Skeleton className="mt-3 h-3 w-28" />
          </Card>
        ))}
      </div>
      <div className="grid gap-6 lg:grid-cols-3">
        <PanelSkeleton className="lg:col-span-2" bodyHeight="h-64" />
        <PanelSkeleton bodyHeight="h-64" />
      </div>
      <div className="grid gap-6 lg:grid-cols-3">
        <PanelSkeleton />
        <PanelSkeleton className="lg:col-span-2" />
      </div>
      <span className="sr-only">Loading dashboard…</span>
    </div>
  )
}
