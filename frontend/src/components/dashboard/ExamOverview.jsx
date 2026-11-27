import { Link } from 'react-router-dom'
import { FilePenLine } from 'lucide-react'
import { formatDateOnly } from '../../lib/students'
import { ExamStatusBadge } from '../examinations/ExamStatusBadge'
import { ModuleUnavailable, NoData, Panel } from './Panel'

function ExamRow({ x }) {
  return (
    <li>
      <Link to={`/examinations/${x.id}`} className="flex items-center gap-3 rounded-lg px-2 py-2 hover:bg-slate-50">
        <span className="min-w-0 flex-1">
          <span className="block truncate text-sm font-medium text-slate-900">{x.name}</span>
          <span className="block truncate text-xs text-slate-500">
            {x.exam_type} · {formatDateOnly(x.start_date)} – {formatDateOnly(x.end_date)} · {x.classes} class{x.classes === 1 ? '' : 'es'}
          </span>
        </span>
        <ExamStatusBadge status={x.status} />
      </Link>
    </li>
  )
}

/** Upcoming and recent examinations (Feature 11). */
export function ExamOverview({ section, className }) {
  const data = section?.data
  return (
    <Panel
      icon={FilePenLine}
      title="Examinations"
      description={data ? `${data.published} published this academic year.` : 'Examination timetables.'}
      className={className}
      action={
        section?.status === 'available' && (
          <Link to="/examinations" className="text-[13px] font-medium text-brand-700 hover:underline">
            View all
          </Link>
        )
      }
    >
      {section?.status !== 'available' ? (
        <ModuleUnavailable module={section?.module} icon={FilePenLine} title="Examinations aren't set up yet" description="Upcoming examinations will appear here." />
      ) : data.upcoming.length === 0 && data.recent.length === 0 ? (
        <NoData icon={FilePenLine} title="No examinations yet" description="Create an examination to plan its timetable." />
      ) : (
        <div className="space-y-4">
          <div>
            <p className="mb-1 text-xs font-medium tracking-wide text-slate-400 uppercase">Upcoming</p>
            {data.upcoming.length === 0 ? (
              <p className="px-2 py-1 text-sm text-slate-500">No published examination is coming up.</p>
            ) : (
              <ul>
                {data.upcoming.map((x) => (
                  <ExamRow key={x.id} x={x} />
                ))}
              </ul>
            )}
          </div>
          <div>
            <p className="mb-1 text-xs font-medium tracking-wide text-slate-400 uppercase">Recently created</p>
            <ul>
              {data.recent.map((x) => (
                <ExamRow key={x.id} x={x} />
              ))}
            </ul>
          </div>
        </div>
      )}
    </Panel>
  )
}
