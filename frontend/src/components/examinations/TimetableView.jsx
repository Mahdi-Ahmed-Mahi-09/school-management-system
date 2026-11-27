import { BookOpen, CalendarDays, DoorOpen } from 'lucide-react'
import { formatDateOnly } from '../../lib/students'
import { cn } from '../../lib/utils'
import { durationLabel, timeRange, weekday } from '../../lib/examinations'
import { Badge } from '../ui/Display'

/** A class's papers as a table (also used for printing). */
export function TimetableTable({ schedules, showInstructions = true }) {
  if (!schedules.length) return <p className="px-5 py-6 text-sm text-slate-500 sm:px-6">No papers scheduled.</p>
  return (
    <div className="overflow-x-auto print:overflow-visible">
      <table className="min-w-full text-sm">
        <thead className="bg-slate-50/70 text-xs font-medium text-slate-500 print:bg-transparent">
          <tr className="border-b border-slate-100 print:border-slate-400">
            <th className="px-4 py-2.5 text-left sm:pl-6">Date</th>
            <th className="px-4 py-2.5 text-left">Time</th>
            <th className="px-4 py-2.5 text-left">Subject</th>
            <th className="px-4 py-2.5 text-left">Room</th>
            {showInstructions && <th className="px-4 py-2.5 text-left sm:pr-6">Instructions</th>}
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100 print:divide-slate-300">
          {schedules.map((s) => (
            <tr key={s.id} className={cn(s.teacher_role && 'bg-brand-50/50')}>
              <td className="px-4 py-2.5 whitespace-nowrap text-slate-800 sm:pl-6">
                <span className="font-medium">{formatDateOnly(s.exam_date)}</span>
                <span className="ml-1.5 text-xs text-slate-500">{weekday(s.exam_date)}</span>
              </td>
              <td className="px-4 py-2.5 whitespace-nowrap text-slate-700">
                {timeRange(s)}
                <span className="block text-xs text-slate-400">{durationLabel(s.start_time, s.end_time)}</span>
              </td>
              <td className="px-4 py-2.5">
                <span className="font-medium text-slate-900">{s.subject.name}</span>
                <span className="ml-1.5 font-mono text-xs text-slate-500">{s.subject.code}</span>
                {s.teacher_role && <Badge className="ml-2 bg-brand-50 text-brand-700 ring-brand-600/15 print:hidden">You teach this</Badge>}
              </td>
              <td className="px-4 py-2.5 text-slate-700">{s.room_number || <span className="text-slate-300">—</span>}</td>
              {showInstructions && <td className="max-w-xs px-4 py-2.5 text-slate-600 sm:pr-6">{s.instructions || <span className="text-slate-300">—</span>}</td>}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

/** Papers grouped by day across classes: a compact calendar-style agenda. */
export function TimetableAgenda({ classes }) {
  const byDate = new Map()
  for (const c of classes) {
    for (const s of c.schedules) {
      if (!byDate.has(s.exam_date)) byDate.set(s.exam_date, [])
      byDate.get(s.exam_date).push({ ...s, className: c.class.name })
    }
  }
  const days = [...byDate.keys()].sort()
  if (!days.length) return <p className="px-6 py-6 text-sm text-slate-500">No papers scheduled.</p>
  return (
    <div className="grid gap-3 p-4 sm:grid-cols-2 sm:p-5 xl:grid-cols-3">
      {days.map((d) => (
        <div key={d} className="rounded-xl border border-slate-200/80 bg-white">
          <div className="flex items-center gap-2 border-b border-slate-100 px-4 py-2.5">
            <CalendarDays className="size-4 text-brand-500" aria-hidden="true" />
            <p className="text-sm font-semibold text-slate-900">{formatDateOnly(d)}</p>
            <span className="text-xs text-slate-500">{weekday(d)}</span>
          </div>
          <ul className="divide-y divide-slate-100">
            {byDate
              .get(d)
              .sort((a, b) => a.start_time.localeCompare(b.start_time))
              .map((s) => (
                <li key={s.id} className={cn('px-4 py-2.5', s.teacher_role && 'bg-brand-50/50')}>
                  <p className="text-xs font-medium text-slate-500">{timeRange(s)}</p>
                  <p className="flex items-center gap-1.5 text-sm font-medium text-slate-900">
                    <BookOpen className="size-3.5 text-slate-400" aria-hidden="true" />
                    {s.subject.name}
                    <span className="font-normal text-slate-500">· {s.className}</span>
                  </p>
                  {s.room_number && (
                    <p className="flex items-center gap-1.5 text-xs text-slate-500">
                      <DoorOpen className="size-3.5" aria-hidden="true" />
                      Room {s.room_number}
                    </p>
                  )}
                </li>
              ))}
          </ul>
        </div>
      ))}
    </div>
  )
}
