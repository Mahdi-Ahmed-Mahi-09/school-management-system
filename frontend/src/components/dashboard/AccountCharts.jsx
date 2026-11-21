import { Bar, BarChart, CartesianGrid, Cell, Pie, PieChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { ChartNoAxesColumn, PieChart as PieIcon, TrendingUp, Users } from 'lucide-react'
import { formatMonth, formatNumber, percent } from '../../lib/utils'
import { ChartLegend, ChartTooltip } from './ChartParts'
import { ModuleUnavailable, NoData, Panel } from './Panel'
import { ROLE_COLORS, axisProps, gridProps } from './chartTheme'

const SERIES = [
  { key: 'students', label: 'Students' },
  { key: 'teachers', label: 'Teachers' },
  { key: 'guardians', label: 'Guardians' },
  { key: 'admins', label: 'Admins' },
]

/** New accounts per month, stacked by role. Backed by users.created_at. */
export function AccountGrowthChart({ section, className }) {
  const points = section?.data ?? []
  const total = points.reduce((sum, p) => sum + SERIES.reduce((s, { key }) => s + p[key], 0), 0)

  return (
    <Panel
      icon={TrendingUp}
      title="New accounts"
      description="Accounts created per month over the last six months, by role."
      className={className}
      action={
        section?.status === 'available' && (
          <span className="rounded-md bg-slate-50 px-2 py-1 text-xs font-medium text-slate-600 ring-1 ring-slate-200/70 tabular-nums">
            {formatNumber(total)} total
          </span>
        )
      }
    >
      {section?.status !== 'available' ? (
        <ModuleUnavailable module={section?.module} icon={ChartNoAxesColumn} title="Account trends unavailable" description="Monthly account trends will appear here." />
      ) : total === 0 ? (
        <NoData icon={ChartNoAxesColumn} title="No new accounts in the last six months" description="New accounts will be charted here as they're created." />
      ) : (
        <div className="space-y-4">
          <ChartLegend items={SERIES.map((s) => ({ label: s.label, color: ROLE_COLORS[s.key] }))} />
          <div className="h-64" role="img" aria-label={`Bar chart of new accounts per month, ${formatNumber(total)} in total`}>
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={points} margin={{ top: 4, right: 4, bottom: 0, left: -20 }} barCategoryGap="28%">
                <CartesianGrid {...gridProps} />
                <XAxis dataKey="month" tickFormatter={(m) => formatMonth(m)} {...axisProps} />
                <YAxis allowDecimals={false} {...axisProps} />
                <Tooltip
                  cursor={{ fill: '#f1f5f9' }}
                  content={<ChartTooltip labelFormatter={(m) => formatMonth(m, 'long')} valueFormatter={formatNumber} />}
                />
                {SERIES.map((s, i) => (
                  <Bar
                    key={s.key}
                    dataKey={s.key}
                    name={s.label}
                    stackId="accounts"
                    fill={ROLE_COLORS[s.key]}
                    radius={i === SERIES.length - 1 ? [4, 4, 0, 0] : 0}
                    maxBarSize={40}
                  />
                ))}
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
      )}
    </Panel>
  )
}

const ROLE_KEYS = [
  { role: 'student', key: 'students', label: 'Students' },
  { role: 'teacher', key: 'teachers', label: 'Teachers' },
  { role: 'guardian', key: 'guardians', label: 'Guardians' },
  { role: 'admin', key: 'admins', label: 'Admins' },
]

/** Donut of all accounts by role. Backed by the users table. */
export function AccountsByRoleChart({ section, className }) {
  const summary = section?.data
  const rows = ROLE_KEYS.map(({ role, key, label }) => ({
    name: label,
    value: summary?.by_role?.[role]?.total ?? 0,
    fill: ROLE_COLORS[key],
  }))

  return (
    <Panel icon={PieIcon} title="Accounts by role" description="Everyone with access to Classora." className={className}>
      {section?.status !== 'available' ? (
        <ModuleUnavailable module={section?.module} icon={Users} title="Account summary unavailable" description="Account totals will appear here." />
      ) : summary.total === 0 ? (
        <NoData icon={Users} title="No accounts yet" />
      ) : (
        <div className="flex flex-col items-center gap-5 sm:flex-row lg:flex-col 2xl:flex-row">
          <div className="relative size-44 shrink-0" role="img" aria-label={`Donut chart of ${formatNumber(summary.total)} accounts by role`}>
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie data={rows} dataKey="value" nameKey="name" innerRadius="68%" outerRadius="100%" paddingAngle={rows.filter((r) => r.value).length > 1 ? 2 : 0} stroke="none" isAnimationActive={false}>
                  {rows.map((r) => (
                    <Cell key={r.name} fill={r.fill} />
                  ))}
                </Pie>
                <Tooltip content={<ChartTooltip valueFormatter={formatNumber} />} />
              </PieChart>
            </ResponsiveContainer>
            <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
              <span className="text-2xl font-semibold text-slate-900 tabular-nums">{formatNumber(summary.total)}</span>
              <span className="text-xs text-slate-500">accounts</span>
            </div>
          </div>
          <ul className="w-full space-y-2.5">
            {rows.map((r) => (
              <li key={r.name} className="flex items-center gap-3 text-sm">
                <span className="size-2.5 rounded-full" style={{ background: r.fill }} aria-hidden="true" />
                <span className="flex-1 text-slate-600">{r.name}</span>
                <span className="font-medium text-slate-900 tabular-nums">{formatNumber(r.value)}</span>
                <span className="w-10 text-right text-xs text-slate-400 tabular-nums">{percent(r.value, summary.total)}%</span>
              </li>
            ))}
            <li className="flex items-center justify-between border-t border-slate-100 pt-2.5 text-[13px] text-slate-500">
              <span>Active / inactive</span>
              <span className="tabular-nums">
                <span className="font-medium text-emerald-600">{formatNumber(summary.active)}</span> /{' '}
                <span className="font-medium text-slate-600">{formatNumber(summary.inactive)}</span>
              </span>
            </li>
          </ul>
        </div>
      )}
    </Panel>
  )
}
