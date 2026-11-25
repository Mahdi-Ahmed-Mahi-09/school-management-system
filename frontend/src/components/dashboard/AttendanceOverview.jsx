import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from 'recharts'
import { CalendarCheck2, ClipboardCheck } from 'lucide-react'
import { formatNumber, percent } from '../../lib/utils'
import { ChartTooltip } from './ChartParts'
import { ModuleUnavailable, NoData, Panel } from './Panel'
import { ATTENDANCE_COLORS } from './chartTheme'

const CATEGORIES = [
  { key: 'present', label: 'Present' },
  { key: 'late', label: 'Late' },
  { key: 'leave', label: 'On leave' },
  { key: 'absent', label: 'Absent' },
]

/** Today's attendance. Renders only real data from the attendance module. */
export function AttendanceOverview({ section, className }) {
  const data = section?.data
  const recorded = data ? CATEGORIES.reduce((sum, c) => sum + (data[c.key] ?? 0), 0) : 0
  // (present + late) / (present + absent + late): leave and unmarked students
  // are excluded, so the rate never treats a missing record as an absence.
  const rate = data ? percent(data.present + data.late, data.present + data.absent + data.late) : null

  return (
    <Panel icon={CalendarCheck2} title="Today's attendance" description="Student attendance recorded today." className={className}>
      {section?.status !== 'available' ? (
        <ModuleUnavailable
          module={section?.module}
          icon={ClipboardCheck}
          title="Attendance isn't tracked yet"
          description="Present, absent, late and leave figures will appear here once daily attendance is being recorded."
        />
      ) : recorded === 0 ? (
        <NoData
          icon={ClipboardCheck}
          title="No attendance recorded today"
          description={`${formatNumber(data.expected)} students are expected. Figures appear as teachers submit attendance.`}
        />
      ) : (
        <div className="space-y-5">
          <div className="flex items-center gap-5">
            <div className="relative size-32 shrink-0" role="img" aria-label={rate == null ? 'Attendance rate not available' : `Attendance rate ${rate}%`}>
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Pie
                    data={CATEGORIES.map((c) => ({ name: c.label, value: data[c.key] ?? 0, fill: ATTENDANCE_COLORS[c.key] }))}
                    dataKey="value"
                    innerRadius="70%"
                    outerRadius="100%"
                    stroke="none"
                    isAnimationActive={false}
                  >
                    {CATEGORIES.map((c) => (
                      <Cell key={c.key} fill={ATTENDANCE_COLORS[c.key]} />
                    ))}
                  </Pie>
                  <Tooltip content={<ChartTooltip valueFormatter={formatNumber} />} />
                </PieChart>
              </ResponsiveContainer>
              <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
                <span className="text-xl font-semibold text-slate-900 tabular-nums">{rate == null ? 'N/A' : `${rate}%`}</span>
                <span className="text-[11px] text-slate-500">attendance</span>
              </div>
            </div>
            <div className="min-w-0">
              <p className="text-[13px] text-slate-500">Expected today</p>
              <p className="text-2xl font-semibold text-slate-900 tabular-nums">{formatNumber(data.expected)}</p>
              <p className="mt-1 text-xs text-slate-500">
                {formatNumber(recorded)} recorded · {formatNumber(Math.max(data.expected - recorded, 0))} unmarked
              </p>
            </div>
          </div>
          <dl className="grid grid-cols-2 gap-2">
            {CATEGORIES.map((c) => (
              <div key={c.key} className="rounded-lg bg-slate-50 px-3 py-2.5">
                <dt className="flex items-center gap-1.5 text-xs text-slate-500">
                  <span className="size-2 rounded-full" style={{ background: ATTENDANCE_COLORS[c.key] }} aria-hidden="true" />
                  {c.label}
                </dt>
                <dd className="mt-0.5 text-lg font-semibold text-slate-900 tabular-nums">{formatNumber(data[c.key])}</dd>
              </div>
            ))}
          </dl>
        </div>
      )}
    </Panel>
  )
}
