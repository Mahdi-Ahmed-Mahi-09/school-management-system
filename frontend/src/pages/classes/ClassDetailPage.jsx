import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { BookOpen, BookPlus, Info, LayoutGrid, PencilLine, Plus, Power, PowerOff, Rows3, SearchX, Trash2 } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { classesApi } from '../../api/services'
import { CurrentYearPill } from '../../components/academic-years/CurrentYearPill'
import { YearStatusBadge } from '../../components/academic-years/YearStatusBadge'
import { ClassFormModal } from '../../components/classes/ClassFormModal'
import { SectionFormModal } from '../../components/classes/SectionFormModal'
import { useClassActions } from '../../components/classes/useClassActions'
import { AssignSubjectsModal } from '../../components/subjects/AssignSubjectsModal'
import { ClassTeachersCard } from '../../components/teacher-assignments/ClassTeachersCard'
import { AssignmentTypeBadge } from '../../components/subjects/SubjectTypeBadge'
import { Breadcrumbs } from '../../components/ui/Breadcrumbs'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Alert, Card, EmptyState, ErrorState, Skeleton, StatusBadge } from '../../components/ui/Display'
import { DetailItem, DetailSection } from '../../components/ui/Sections'
import { useAuth } from '../../hooks/useAuth'
import { PERMISSIONS } from '../../lib/roles'
import { groupAssignmentsByYear } from '../../lib/subjects'
import { formatDateTime } from '../../lib/utils'

/** Groups sections (already ordered newest year first) by academic year. */
function groupByYear(sections) {
  const groups = []
  for (const s of sections) {
    const last = groups[groups.length - 1]
    if (last?.year.id === s.academic_year.id) last.sections.push(s)
    else groups.push({ year: s.academic_year, sections: [s] })
  }
  return groups
}

function DetailSkeleton() {
  return (
    <div className="space-y-6">
      <Skeleton className="h-4 w-40" />
      <Skeleton className="h-28 rounded-xl" />
      <div className="grid gap-6 lg:grid-cols-3">
        <Skeleton className="h-56 rounded-xl" />
        <Skeleton className="h-56 rounded-xl lg:col-span-2" />
      </div>
    </div>
  )
}

export default function ClassDetailPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { can } = useAuth()
  const canReadSubjects = can(PERMISSIONS.SUBJECTS_READ)
  const [state, setState] = useState({ loading: true, error: null, notFound: false, cls: null, sections: [], subjects: [] })
  const [editing, setEditing] = useState(false)
  const [addingSection, setAddingSection] = useState(false)
  const [assigning, setAssigning] = useState(false)

  const load = useCallback(async () => {
    setState((s) => ({ ...s, loading: true, error: null }))
    try {
      const [cls, sections, subjects] = await Promise.all([
        classesApi.get(id),
        classesApi.sections(id),
        canReadSubjects ? classesApi.subjects(id) : Promise.resolve([]),
      ])
      setState({ loading: false, error: null, notFound: false, cls, sections, subjects })
    } catch (err) {
      setState({ loading: false, error: getErrorMessage(err), notFound: err?.response?.status === 404, cls: null, sections: [], subjects: [] })
    }
  }, [id, canReadSubjects])

  useEffect(() => {
    load()
  }, [load])

  const actions = useClassActions({ onChanged: load, onDeleted: () => navigate('/classes', { replace: true }) })

  if (state.loading && !state.cls) return <DetailSkeleton />
  if (state.notFound || state.error) {
    return (
      <div className="space-y-6">
        <Breadcrumbs items={[{ label: 'Classes', to: '/classes' }, { label: state.notFound ? 'Not found' : 'Error' }]} />
        <Card>
          {state.notFound ? (
            <EmptyState
              icon={SearchX}
              title="Class not found"
              description="It doesn't exist or may have been deleted."
              action={
                <Link to="/classes" className={buttonClasses('secondary', 'sm')}>
                  Back to classes
                </Link>
              }
            />
          ) : (
            <ErrorState title="Couldn't load this class" message={state.error} onRetry={load} />
          )}
        </Card>
      </div>
    )
  }

  const c = state.cls
  const groups = groupByYear(state.sections)
  const active = c.status === 'active'
  const canAddSection = can(PERMISSIONS.SECTIONS_CREATE)
  const subjectGroups = groupAssignmentsByYear(state.subjects)
  const inUse = c.section_count > 0 || state.subjects.length > 0

  return (
    <div className="space-y-6 animate-fade-in">
      <Breadcrumbs items={[{ label: 'Classes', to: '/classes' }, { label: c.name }]} />

      <Card className="p-5 sm:p-6">
        <div className="flex flex-col gap-5 sm:flex-row sm:items-center">
          <span className="flex size-14 shrink-0 items-center justify-center rounded-2xl bg-brand-50 text-brand-600 ring-1 ring-brand-100">
            <LayoutGrid className="size-6" aria-hidden="true" />
          </span>
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-2xl font-semibold tracking-tight text-slate-900">{c.name}</h1>
              <StatusBadge status={c.status} />
            </div>
            <p className="mt-1 text-[15px] text-slate-600">
              {c.code && <span className="font-mono">{c.code} · </span>}Display order {c.display_order} · {c.section_count} section{c.section_count === 1 ? '' : 's'} in all years
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            {can(PERMISSIONS.CLASSES_UPDATE) && (
              <>
                <Button variant="secondary" icon={PencilLine} onClick={() => setEditing(true)}>
                  Edit
                </Button>
                <Button variant="secondary" icon={active ? PowerOff : Power} onClick={() => actions.toggleStatus(c)}>
                  {active ? 'Deactivate' : 'Activate'}
                </Button>
              </>
            )}
            {can(PERMISSIONS.CLASSES_DELETE) && (
              <Button
                variant="secondary"
                icon={Trash2}
                className="text-rose-600 hover:text-rose-700"
                disabled={inUse}
                title={c.section_count > 0 ? 'Delete its sections first' : state.subjects.length > 0 ? 'Remove its subject assignments first' : undefined}
                onClick={() => actions.remove(c)}
              >
                Delete
              </Button>
            )}
          </div>
        </div>
      </Card>

      {!active && (
        <Alert tone="warning" title="This class is inactive">
          Its existing sections are kept, but new sections can’t be added until the class is activated.
        </Alert>
      )}

      <div className="grid gap-6 lg:grid-cols-3">
        <DetailSection title="Class information" icon={Info} className="h-fit">
          <DetailItem label="Class name" value={c.name} wide />
          <DetailItem label="Class code" value={c.code} mono />
          <DetailItem label="Display order" value={String(c.display_order)} />
          <DetailItem label="Status" value={<StatusBadge status={c.status} />} />
          <DetailItem label="Sections (all years)" value={String(c.section_count)} />
          <DetailItem label="Description" value={c.description} wide />
          <DetailItem label="Created" value={`${formatDateTime(c.created_at)}${c.created_by?.name ? ` by ${c.created_by.name}` : ''}`} wide />
          <DetailItem label="Last updated" value={`${formatDateTime(c.updated_at)}${c.updated_by?.name ? ` by ${c.updated_by.name}` : ''}`} wide />
        </DetailSection>

        <Card className="lg:col-span-2">
          <div className="flex flex-wrap items-center gap-2.5 border-b border-slate-100 px-5 py-3.5 sm:px-6">
            <Rows3 className="size-4 text-slate-400" aria-hidden="true" />
            <h2 className="flex-1 text-sm font-semibold text-slate-900">Sections by academic year</h2>
            {canAddSection && (
              <Button size="sm" icon={Plus} disabled={!active} title={active ? undefined : 'Activate the class first'} onClick={() => setAddingSection(true)}>
                Add section
              </Button>
            )}
          </div>
          {groups.length === 0 ? (
            <EmptyState icon={Rows3} title="No sections yet" description="Sections are created for each academic year, e.g. A, B and C for 2026–2027." />
          ) : (
            <div className="divide-y divide-slate-100">
              {groups.map(({ year, sections }) => (
                <section key={year.id} className="px-5 py-4 sm:px-6" aria-label={`Sections in ${year.name}`}>
                  <div className="mb-3 flex flex-wrap items-center gap-2">
                    <Link to={`/academic-years/${year.id}`} className="text-sm font-semibold text-slate-900 hover:text-brand-700">
                      {year.name}
                    </Link>
                    <YearStatusBadge status={year.status} />
                    {year.is_current && <CurrentYearPill />}
                    <span className="ml-auto text-xs text-slate-500">
                      {sections.length} section{sections.length === 1 ? '' : 's'}
                    </span>
                  </div>
                  <ul className="grid gap-2 sm:grid-cols-2">
                    {sections.map((s) => (
                      <li key={s.id}>
                        <Link
                          to={`/sections/${s.id}`}
                          className="flex items-center gap-3 rounded-lg border border-slate-200/80 px-3 py-2.5 transition hover:border-brand-200 hover:bg-brand-50/40"
                        >
                          <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-slate-50 text-sm font-semibold text-slate-700 ring-1 ring-slate-200/70">
                            {s.name.slice(0, 3)}
                          </span>
                          <span className="min-w-0 flex-1">
                            <span className="block truncate text-sm font-medium text-slate-900">Section {s.name}</span>
                            <span className="block truncate text-xs text-slate-500">
                              {[s.room_number, s.capacity != null && `Capacity ${s.capacity}`].filter(Boolean).join(' · ') || 'No room or capacity set'}
                            </span>
                          </span>
                          <StatusBadge status={s.status} />
                        </Link>
                      </li>
                    ))}
                  </ul>
                </section>
              ))}
            </div>
          )}
        </Card>
      </div>

      {canReadSubjects && (
        <Card>
          <div className="flex flex-wrap items-center gap-2.5 border-b border-slate-100 px-5 py-3.5 sm:px-6">
            <BookOpen className="size-4 text-slate-400" aria-hidden="true" />
            <h2 className="flex-1 text-sm font-semibold text-slate-900">Subjects by academic year</h2>
            {can(PERMISSIONS.SUBJECTS_ASSIGN) && (
              <Button size="sm" icon={BookPlus} disabled={!active} title={active ? undefined : 'Activate the class first'} onClick={() => setAssigning(true)}>
                Assign subjects
              </Button>
            )}
          </div>
          {subjectGroups.length === 0 ? (
            <EmptyState icon={BookOpen} title="No subjects assigned yet" description="Choose the subjects this class takes in each academic year, as compulsory or optional." />
          ) : (
            <div className="divide-y divide-slate-100">
              {subjectGroups.map(({ year, items }) => (
                <section key={year.id} className="px-5 py-4 sm:px-6" aria-label={`Subjects in ${year.name}`}>
                  <div className="mb-3 flex flex-wrap items-center gap-2">
                    <Link to={`/academic-years/${year.id}`} className="text-sm font-semibold text-slate-900 hover:text-brand-700">
                      {year.name}
                    </Link>
                    <YearStatusBadge status={year.status} />
                    {year.is_current && <CurrentYearPill />}
                    <span className="ml-auto text-xs text-slate-500">
                      {items.filter((a) => a.is_compulsory).length} compulsory · {items.filter((a) => !a.is_compulsory).length} optional ·{' '}
                      <Link to={`/class-subjects?year=${year.id}&class=${c.id}`} className="font-medium text-brand-700 hover:underline">
                        Manage
                      </Link>
                    </span>
                  </div>
                  <ul className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
                    {items.map((a) => (
                      <li key={a.id}>
                        <Link
                          to={`/subjects/${a.subject.id}`}
                          className="flex items-center gap-3 rounded-lg border border-slate-200/80 px-3 py-2.5 transition hover:border-brand-200 hover:bg-brand-50/40"
                        >
                          <span className="min-w-0 flex-1">
                            <span className="block truncate text-sm font-medium text-slate-900">{a.subject.name}</span>
                            <span className="block truncate font-mono text-xs text-slate-500">{a.subject.code}</span>
                          </span>
                          <AssignmentTypeBadge compulsory={a.is_compulsory} />
                        </Link>
                      </li>
                    ))}
                  </ul>
                </section>
              ))}
            </div>
          )}
        </Card>
      )}

      {can(PERMISSIONS.TEACHER_ASSIGNMENTS_READ) && <ClassTeachersCard classId={c.id} />}

      <ClassFormModal open={editing} classItem={c} onClose={() => setEditing(false)} onSaved={load} />
      <SectionFormModal open={addingSection} defaults={{ class_id: c.id }} onClose={() => setAddingSection(false)} onSaved={load} />
      <AssignSubjectsModal open={assigning} defaults={{ class_id: c.id }} onClose={() => setAssigning(false)} onSaved={load} />
      {actions.dialog}
    </div>
  )
}
