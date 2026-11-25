import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import {
  ArrowRightLeft,
  BookOpen,
  BookUser,
  CalendarRange,
  CircleSlash,
  Eye,
  Layers,
  Power,
  PowerOff,
  Search,
  SearchX,
  Trash2,
  UserRoundCheck,
  Users,
  X,
} from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { teacherAssignmentsApi, teachersApi } from '../../api/services'
import { AssignmentKindBadge } from '../../components/teacher-assignments/AssignmentTypeBadge'
import { AssignTeacherModal } from '../../components/teacher-assignments/AssignTeacherModal'
import { useAssignmentActions } from '../../components/teacher-assignments/useAssignmentActions'
import { Button } from '../../components/ui/Button'
import { buttonClasses } from '../../components/ui/buttonStyles'
import { Alert, Card, EmptyState, ErrorState, PageHeader, Skeleton, StatCard, StatusBadge } from '../../components/ui/Display'
import { Select } from '../../components/ui/Field'
import { Pagination } from '../../components/ui/Pagination'
import { PersonAvatar } from '../../components/ui/PersonAvatar'
import { RowActionsMenu, SearchInput } from '../../components/ui/Table'
import { useAuth } from '../../hooks/useAuth'
import { useListParams } from '../../hooks/useListParams'
import { loadYearOptions, yearOptionLabel } from '../../lib/classes'
import { loadAllPages, loadClasses, loadSections } from '../../lib/enrollments'
import { PERMISSIONS } from '../../lib/roles'
import { loadAllSubjects } from '../../lib/subjects'
import { changeable, removable } from '../../lib/teacherAssignments'
import { formatDate, formatNumber } from '../../lib/utils'

const PAGE_SIZE = 20

function RowSkeleton() {
  return Array.from({ length: 6 }, (_, i) => (
    <tr key={i}>
      {['w-40', 'w-24', 'w-24', 'w-24', 'w-16', 'w-16'].map((w, j) => (
        <td key={j} className={j === 0 ? 'px-4 py-3.5 sm:pl-6' : 'px-4 py-3.5'}>
          <Skeleton className={`h-3.5 ${w}`} />
        </td>
      ))}
      <td className="px-4 sm:pr-6">
        <Skeleton className="ml-auto size-8" />
      </td>
    </tr>
  ))
}

export default function TeacherAssignmentsPage() {
  const { can } = useAuth()
  const navigate = useNavigate()
  const { params, updateParams, searchInput, setSearchInput, clearSearch, page } = useListParams()
  const yearParam = params.get('year') ?? ''
  const classId = params.get('class') ?? ''
  const sectionId = params.get('section') ?? ''
  const subjectId = params.get('subject') ?? ''
  const teacherId = params.get('teacher') ?? ''
  const type = params.get('type') ?? ''
  const status = params.get('status') ?? 'active'
  const q = params.get('q') ?? ''

  const [options, setOptions] = useState({ loading: true, error: null, years: [], current: null, defaultYear: null, classes: [], subjects: [], teachers: [] })
  const [sections, setSections] = useState([])
  const [list, setList] = useState({ loading: true, error: null, assignments: [], meta: null })
  const [stats, setStats] = useState(null)
  const [reloadKey, setReloadKey] = useState(0)
  const [assigning, setAssigning] = useState(null) // 'subject_teacher' | 'class_teacher'
  const reload = useCallback(() => setReloadKey((k) => k + 1), [])
  const actions = useAssignmentActions({ onChanged: reload })

  const loadOptions = useCallback(() => {
    setOptions((o) => ({ ...o, loading: true, error: null }))
    Promise.all([
      loadYearOptions(),
      loadClasses(),
      loadAllSubjects(),
      loadAllPages((p) => teachersApi.list({ sort: 'name', order: 'asc', page: p, page_size: 100 }), 'teachers'),
    ])
      .then(([yearOpts, classes, subjects, teachers]) => setOptions({ loading: false, error: null, ...yearOpts, classes, subjects, teachers }))
      .catch((err) => setOptions((o) => ({ ...o, loading: false, error: getErrorMessage(err) })))
  }, [])
  useEffect(() => {
    loadOptions()
  }, [loadOptions])

  const year = options.years.find((y) => y.id === yearParam) ?? options.defaultYear
  const yearId = year?.id ?? ''

  useEffect(() => {
    let cancelled = false
    loadSections(yearId, classId)
      .then((items) => !cancelled && setSections(items))
      .catch(() => !cancelled && setSections([]))
    return () => {
      cancelled = true
    }
  }, [yearId, classId])

  useEffect(() => {
    if (!yearId) return
    const controller = new AbortController()
    setList((s) => ({ ...s, loading: true, error: null }))
    teacherAssignmentsApi
      .list(
        {
          academic_year_id: yearId,
          class_id: classId || undefined,
          section_id: sectionId || undefined,
          subject_id: subjectId || undefined,
          teacher_id: teacherId || undefined,
          type: type || undefined,
          status: status === 'all' ? undefined : status,
          search: q || undefined,
          page,
          page_size: PAGE_SIZE,
        },
        { signal: controller.signal },
      )
      .then(({ assignments, meta }) => setList({ loading: false, error: null, assignments, meta }))
      .catch((err) => !controller.signal.aborted && setList({ loading: false, error: getErrorMessage(err), assignments: [], meta: null }))
    return () => controller.abort()
  }, [yearId, classId, sectionId, subjectId, teacherId, type, status, q, page, reloadKey])

  useEffect(() => {
    if (!yearId) return
    teacherAssignmentsApi
      .stats({ academic_year_id: yearId })
      .then(setStats)
      .catch(() => setStats(null))
  }, [yearId, reloadKey])

  const hasFilters = Boolean(classId || sectionId || subjectId || teacherId || type || q || status !== 'active')
  const clearFilters = () => {
    clearSearch()
    updateParams({ class: '', section: '', subject: '', teacher: '', type: '', status: '', q: '' })
  }
  const canCreate = can(PERMISSIONS.TEACHER_ASSIGNMENTS_CREATE) && year && year.status !== 'completed'

  const menuItems = (a) =>
    [
      { label: 'View details', icon: Eye, onSelect: () => navigate(`/teacher-assignments/${a.id}`) },
      can(PERMISSIONS.TEACHER_ASSIGNMENTS_UPDATE) && {
        label: changeable(a) ? 'Change teacher' : 'Edit remarks',
        icon: ArrowRightLeft,
        onSelect: () => actions.edit(a),
      },
      can(PERMISSIONS.TEACHER_ASSIGNMENTS_UPDATE) &&
        (a.status === 'active' || a.academic_year.status !== 'completed') && {
          label: a.status === 'active' ? 'Deactivate' : 'Reactivate',
          icon: a.status === 'active' ? PowerOff : Power,
          onSelect: () => actions.toggleStatus(a),
        },
      can(PERMISSIONS.TEACHER_ASSIGNMENTS_DELETE) && removable(a) && { separator: true },
      can(PERMISSIONS.TEACHER_ASSIGNMENTS_DELETE) && removable(a) && { label: 'Remove (made by mistake)', icon: Trash2, danger: true, onSelect: () => actions.remove(a) },
    ].filter(Boolean)

  const headerActions = canCreate && (
    <>
      <Link to={`/teacher-assignments/bulk?year=${yearId}`} className={buttonClasses('secondary')}>
        <Layers className="size-4" aria-hidden="true" />
        Bulk assign
      </Link>
      <Button variant="secondary" icon={UserRoundCheck} onClick={() => setAssigning('class_teacher')}>
        Assign class teacher
      </Button>
      <Button icon={BookUser} onClick={() => setAssigning('subject_teacher')}>
        Assign subject teacher
      </Button>
    </>
  )

  if (options.error) {
    return (
      <div className="space-y-8">
        <PageHeader title="Teacher assignments" />
        <Card>
          <ErrorState title="Couldn't load the filters" message={options.error} onRetry={loadOptions} />
        </Card>
      </div>
    )
  }

  return (
    <div className="space-y-6 animate-fade-in">
      <PageHeader title="Teacher assignments" description="Who teaches each subject in every section, and each section's class teacher, per academic year." actions={headerActions} />

      {!options.loading && options.years.length === 0 ? (
        <Card>
          <EmptyState
            icon={CalendarRange}
            title="No academic years yet"
            description="Teachers are assigned to sections of an academic year. Add one first."
            action={
              <Link to="/academic-years" className={buttonClasses('primary', 'sm')}>
                Go to academic years
              </Link>
            }
          />
        </Card>
      ) : (
        <>
          <div className="grid grid-cols-2 gap-4 lg:grid-cols-3 xl:grid-cols-5">
            <StatCard label="Active assignments" value={formatNumber(stats?.active_assignments)} icon={BookUser} tone="brand" loading={!stats} hint={stats?.academic_year?.name} />
            <StatCard label="Assigned teachers" value={formatNumber(stats?.assigned_teachers)} icon={Users} tone="emerald" loading={!stats} />
            <StatCard label="Subject assignments" value={formatNumber(stats?.subject_assignments)} icon={BookOpen} tone="slate" loading={!stats} />
            <StatCard label="Class teachers" value={formatNumber(stats?.class_teachers)} icon={UserRoundCheck} tone="slate" loading={!stats} />
            <StatCard
              label="Subjects without a teacher"
              value={formatNumber(stats?.unassigned_subjects)}
              icon={CircleSlash}
              tone="amber"
              loading={!stats}
              hint={stats && `${formatNumber(stats.sections_without_class_teacher)} section${stats.sections_without_class_teacher === 1 ? '' : 's'} without a class teacher`}
            />
          </div>

          <Card className="p-4 sm:p-5">
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              {options.loading ? (
                [0, 1, 2, 3, 4, 5, 6].map((i) => <Skeleton key={i} className="h-16" />)
              ) : (
                <>
                  <Select label="Academic year" value={yearId} onChange={(e) => updateParams({ year: e.target.value, section: '' })}>
                    {options.years.map((y) => (
                      <option key={y.id} value={y.id}>
                        {yearOptionLabel(y)}
                      </option>
                    ))}
                  </Select>
                  <Select label="Class" value={classId} onChange={(e) => updateParams({ class: e.target.value, section: '' })}>
                    <option value="">All classes</option>
                    {options.classes.map((c) => (
                      <option key={c.id} value={c.id}>
                        {c.name}
                      </option>
                    ))}
                  </Select>
                  <Select label="Section" value={sectionId} disabled={!classId} onChange={(e) => updateParams({ section: e.target.value })}>
                    <option value="">{classId ? 'All sections' : 'Choose a class first'}</option>
                    {sections.map((s) => (
                      <option key={s.id} value={s.id}>
                        Section {s.name}
                      </option>
                    ))}
                  </Select>
                  <Select label="Subject" value={subjectId} onChange={(e) => updateParams({ subject: e.target.value })}>
                    <option value="">All subjects</option>
                    {options.subjects.map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.name} ({s.code})
                      </option>
                    ))}
                  </Select>
                  <Select label="Teacher" value={teacherId} onChange={(e) => updateParams({ teacher: e.target.value })}>
                    <option value="">All teachers</option>
                    {options.teachers.map((t) => (
                      <option key={t.id} value={t.id}>
                        {t.full_name} ({t.employee_id})
                      </option>
                    ))}
                  </Select>
                  <Select label="Assignment type" value={type} onChange={(e) => updateParams({ type: e.target.value })}>
                    <option value="">Subject & class teachers</option>
                    <option value="subject_teacher">Subject teachers</option>
                    <option value="class_teacher">Class teachers</option>
                  </Select>
                  <Select label="Status" value={status} onChange={(e) => updateParams({ status: e.target.value === 'active' ? '' : e.target.value })}>
                    <option value="active">Active</option>
                    <option value="inactive">Inactive (history)</option>
                    <option value="all">All</option>
                  </Select>
                </>
              )}
            </div>
          </Card>

          {!options.loading && !options.current && year && (
            <Alert tone="warning" title="No academic year is current">
              Showing {year.name}. <Link to="/academic-years" className="font-semibold underline">Set a current academic year</Link> so this page defaults to it.
            </Alert>
          )}
          {year?.status === 'completed' && (
            <Alert tone="info" title={`${year.name} is completed`}>
              Its assignments are kept for records. New assignments can only be made for the current or an upcoming year.
            </Alert>
          )}

          <Card className="overflow-hidden">
            <div className="flex flex-col gap-3 border-b border-slate-100 p-4 sm:flex-row sm:items-center sm:px-6">
              <SearchInput icon={Search} value={searchInput} onChange={setSearchInput} placeholder="Search by teacher name or ID, or subject" label="Search assignments" />
              {hasFilters && (
                <Button variant="ghost" icon={X} onClick={clearFilters}>
                  Clear filters
                </Button>
              )}
            </div>
            {list.error ? (
              <ErrorState title="Couldn't load assignments" message={list.error} onRetry={reload} />
            ) : !list.loading && list.assignments.length === 0 ? (
              hasFilters ? (
                <EmptyState
                  icon={SearchX}
                  title="No matching assignments"
                  description="Try different filters or a different search."
                  action={
                    <Button variant="secondary" size="sm" onClick={clearFilters}>
                      Clear filters
                    </Button>
                  }
                />
              ) : (
                <EmptyState
                  icon={BookUser}
                  title={`No teachers assigned in ${year?.name ?? 'this year'} yet`}
                  description="Assign subject teachers to each section, and a class teacher to look after it."
                  action={
                    canCreate && (
                      <Button size="sm" icon={BookUser} onClick={() => setAssigning('subject_teacher')}>
                        Assign subject teacher
                      </Button>
                    )
                  }
                />
              )
            ) : (
              <>
                <div className="hidden overflow-x-auto md:block">
                  <table className="min-w-full text-sm">
                    <thead className="bg-slate-50/70 text-xs font-medium text-slate-500">
                      <tr className="border-b border-slate-100">
                        <th className="px-4 py-3 text-left sm:pl-6">Teacher</th>
                        <th className="px-4 py-3 text-left">Type</th>
                        <th className="px-4 py-3 text-left">Class & section</th>
                        <th className="px-4 py-3 text-left">Subject</th>
                        <th className="hidden px-4 py-3 text-left lg:table-cell">Assigned</th>
                        <th className="px-4 py-3 text-left">Status</th>
                        <th className="px-4 py-3 sm:pr-6">
                          <span className="sr-only">Actions</span>
                        </th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100">
                      {list.loading ? (
                        <RowSkeleton />
                      ) : (
                        list.assignments.map((a) => (
                          <tr key={a.id} className="transition-colors hover:bg-slate-50/70">
                            <td className="px-4 py-2.5 sm:pl-6">
                              <Link to={`/teacher-assignments/${a.id}`} className="flex items-center gap-3">
                                <PersonAvatar person={a.teacher} size="sm" />
                                <span>
                                  <span className="block font-medium text-slate-900 hover:text-brand-700">{a.teacher.full_name}</span>
                                  <span className="block font-mono text-xs text-slate-500">{a.teacher.employee_id}</span>
                                </span>
                              </Link>
                            </td>
                            <td className="px-4 py-2.5">
                              <AssignmentKindBadge type={a.assignment_type} />
                            </td>
                            <td className="px-4 py-2.5 text-slate-700">
                              {a.class.name} <span className="text-slate-400">·</span> Section {a.section.name}
                              <span className="block text-xs text-slate-400">{a.academic_year.name}</span>
                            </td>
                            <td className="px-4 py-2.5 text-slate-700">{a.subject ? a.subject.name : <span className="text-slate-400">—</span>}</td>
                            <td className="hidden px-4 py-2.5 whitespace-nowrap text-slate-500 lg:table-cell">{formatDate(a.assigned_at)}</td>
                            <td className="px-4 py-2.5">
                              <StatusBadge status={a.status} />
                            </td>
                            <td className="px-4 py-2.5 text-right sm:pr-6">
                              <RowActionsMenu label={`Actions for ${a.teacher.full_name}, ${a.subject?.name ?? 'class teacher'} in ${a.section.name}`} items={menuItems(a)} />
                            </td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
                <ul className="divide-y divide-slate-100 md:hidden">
                  {list.loading
                    ? Array.from({ length: 4 }, (_, i) => (
                        <li key={i} className="space-y-2 p-4">
                          <Skeleton className="h-4 w-32" />
                          <Skeleton className="h-3 w-44" />
                        </li>
                      ))
                    : list.assignments.map((a) => (
                        <li key={a.id} className="flex items-start gap-3 p-4">
                          <PersonAvatar person={a.teacher} size="sm" />
                          <Link to={`/teacher-assignments/${a.id}`} className="min-w-0 flex-1">
                            <span className="block truncate text-sm font-medium text-slate-900">{a.teacher.full_name}</span>
                            <span className="block text-[13px] text-slate-500">
                              {a.subject ? a.subject.name : 'Class teacher'} · {a.class.name} – {a.section.name}
                            </span>
                            <span className="mt-2 flex flex-wrap gap-1.5">
                              <AssignmentKindBadge type={a.assignment_type} />
                              <StatusBadge status={a.status} />
                            </span>
                          </Link>
                          <RowActionsMenu label={`Actions for ${a.teacher.full_name}`} items={menuItems(a)} />
                        </li>
                      ))}
                </ul>
                {list.meta && (
                  <Pagination
                    page={list.meta.page}
                    pageSize={list.meta.page_size}
                    total={list.meta.total}
                    totalPages={list.meta.total_pages}
                    onPageChange={(p) => updateParams({ page: p }, { resetPage: false })}
                  />
                )}
              </>
            )}
          </Card>
        </>
      )}

      <AssignTeacherModal
        open={!!assigning}
        type={assigning ?? 'subject_teacher'}
        defaults={{ academic_year_id: yearId, class_id: classId, section_id: sectionId, subject_id: subjectId }}
        onClose={() => setAssigning(null)}
        onSaved={reload}
      />
      {actions.dialog}
    </div>
  )
}
