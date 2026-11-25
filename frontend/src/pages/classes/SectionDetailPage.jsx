import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { History, Info, PencilLine, Power, PowerOff, Rows3, SearchX, Trash2 } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { sectionsApi } from '../../api/services'
import { CurrentYearPill } from '../../components/academic-years/CurrentYearPill'
import { YearStatusBadge } from '../../components/academic-years/YearStatusBadge'
import { SectionFormModal } from '../../components/classes/SectionFormModal'
import { SectionTeachersCard } from '../../components/teacher-assignments/SectionTeachersCard'
import { useSectionActions } from '../../components/classes/useClassActions'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Alert, Card, EmptyState, ErrorState, Skeleton, StatusBadge } from '../../components/ui/Display'
import { DetailItem, DetailSection } from '../../components/ui/Sections'
import { useAuth } from '../../hooks/useAuth'
import { formatYearRange } from '../../lib/academicYears'
import { PERMISSIONS } from '../../lib/roles'
import { formatDateTime } from '../../lib/utils'

export default function SectionDetailPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { can } = useAuth()
  const [state, setState] = useState({ loading: true, error: null, notFound: false, section: null })
  const [editing, setEditing] = useState(false)

  const load = useCallback(async () => {
    setState((s) => ({ ...s, loading: true, error: null }))
    try {
      setState({ loading: false, error: null, notFound: false, section: await sectionsApi.get(id) })
    } catch (err) {
      setState({ loading: false, error: getErrorMessage(err), notFound: err?.response?.status === 404, section: null })
    }
  }, [id])

  useEffect(() => {
    load()
  }, [load])

  const actions = useSectionActions({ onChanged: load, onDeleted: () => navigate('/sections', { replace: true }) })

  if (state.loading && !state.section) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-4 w-56" />
        <Skeleton className="h-28 rounded-xl" />
        <Skeleton className="h-56 rounded-xl" />
      </div>
    )
  }
  if (state.notFound || state.error) {
    return (
      <div className="space-y-6">
        <Breadcrumbs items={[{ label: 'Sections', to: '/sections' }, { label: state.notFound ? 'Not found' : 'Error' }]} />
        <Card>
          {state.notFound ? (
            <EmptyState
              icon={SearchX}
              title="Section not found"
              description="It doesn't exist or may have been deleted."
              action={
                <Link to="/sections" className={buttonClasses('secondary', 'sm')}>
                  Back to sections
                </Link>
              }
            />
          ) : (
            <ErrorState title="Couldn't load this section" message={state.error} onRetry={load} />
          )}
        </Card>
      </div>
    )
  }

  const s = state.section
  const active = s.status === 'active'
  const classInactive = s.class.status !== 'active'

  return (
    <div className="space-y-6 animate-fade-in">
      <Breadcrumbs
        items={[
          { label: 'Sections', to: `/sections?year=${s.academic_year.id}` },
          { label: s.class.name, to: `/classes/${s.class.id}` },
          { label: `Section ${s.name}` },
        ]}
      />

      <Card className="p-5 sm:p-6">
        <div className="flex flex-col gap-5 sm:flex-row sm:items-center">
          <span className="flex size-14 shrink-0 items-center justify-center rounded-2xl bg-brand-50 text-lg font-semibold text-brand-700 ring-1 ring-brand-100">
            {s.name.slice(0, 3)}
          </span>
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-2xl font-semibold tracking-tight text-slate-900">
                {s.class.name} – Section {s.name}
              </h1>
              <StatusBadge status={s.status} />
            </div>
            <p className="mt-1 flex flex-wrap items-center gap-2 text-[15px] text-slate-600">
              {s.academic_year.name}
              <YearStatusBadge status={s.academic_year.status} />
              {s.academic_year.is_current && <CurrentYearPill />}
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            {can(PERMISSIONS.SECTIONS_UPDATE) && (
              <>
                <Button variant="secondary" icon={PencilLine} onClick={() => setEditing(true)}>
                  Edit
                </Button>
                <Button
                  variant="secondary"
                  icon={active ? PowerOff : Power}
                  disabled={!active && classInactive}
                  title={!active && classInactive ? `Activate ${s.class.name} first` : undefined}
                  onClick={() => actions.toggleStatus(s)}
                >
                  {active ? 'Deactivate' : 'Activate'}
                </Button>
              </>
            )}
            {can(PERMISSIONS.SECTIONS_DELETE) && (
              <Button variant="secondary" icon={Trash2} className="text-rose-600 hover:text-rose-700" onClick={() => actions.remove(s)}>
                Delete
              </Button>
            )}
          </div>
        </div>
      </Card>

      {classInactive && (
        <Alert tone="warning" title={`${s.class.name} is inactive`}>
          This section is kept, but it can’t be re-activated while its class is inactive.
        </Alert>
      )}

      <div className="grid gap-6 lg:grid-cols-2">
        <DetailSection title="Section information" icon={Rows3}>
          <DetailItem label="Section name" value={s.name} />
          <DetailItem label="Section code" value={s.code} mono />
          <DetailItem label="Room number" value={s.room_number} />
          <DetailItem label="Student capacity" value={s.capacity == null ? null : String(s.capacity)} />
          <DetailItem label="Status" value={<StatusBadge status={s.status} />} />
          <DetailItem label="Description" value={s.description} wide />
        </DetailSection>

        <DetailSection title="Class & academic year" icon={Info}>
          <DetailItem
            label="Class"
            value={
              <Link to={`/classes/${s.class.id}`} className="text-brand-700 hover:underline">
                {s.class.name}
              </Link>
            }
          />
          <DetailItem label="Class status" value={<StatusBadge status={s.class.status} />} />
          <DetailItem
            label="Academic year"
            value={
              <Link to={`/academic-years/${s.academic_year.id}`} className="text-brand-700 hover:underline">
                {s.academic_year.name}
              </Link>
            }
          />
          <DetailItem label="Year dates" value={formatYearRange(s.academic_year)} />
        </DetailSection>

        <DetailSection title="Record history" icon={History} className="lg:col-span-2">
          <DetailItem label="Created" value={`${formatDateTime(s.created_at)}${s.created_by?.name ? ` by ${s.created_by.name}` : ''}`} />
          <DetailItem label="Last updated" value={`${formatDateTime(s.updated_at)}${s.updated_by?.name ? ` by ${s.updated_by.name}` : ''}`} />
        </DetailSection>
      </div>

      {can(PERMISSIONS.TEACHER_ASSIGNMENTS_READ) && <SectionTeachersCard sectionId={s.id} />}

      <SectionFormModal open={editing} section={s} onClose={() => setEditing(false)} onSaved={load} />
      {actions.dialog}
    </div>
  )
}
