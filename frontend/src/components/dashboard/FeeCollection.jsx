import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { Banknote, Wallet } from 'lucide-react'
import { formatCurrency, formatCurrencyCompact, formatMonth } from '../../lib/utils'
import { ChartTooltip } from './ChartParts'
import { ModuleUnavailable, NoData, Panel } from './Panel'
import { BRAND, axisProps, gridProps } from './chartTheme'

/** Monthly fee collection. Renders only real data from the fees module. */
export function FeeCollection({ section, className }) {
  const data = section?.data
  const monthly = (data?.monthly ?? []).map((m) => ({ month: m.month, amount: Number(m.amount) }))
  const hasHistory = monthly.some((m) => m.amount > 0)
  const money = (v) => formatCurrency(v, data?.currency)

  return (
    <Panel icon={Wallet} title="Fee collection" description="Payments received and balances outstanding." className={className}>
      {section?.status !== 'available' ? (
        <ModuleUnavailable
          module={section?.module}
          icon={Banknote}
          title="Fee data isn't available yet"
          description="Monthly collections, outstanding balances and payment trends will appear here once fees and payments are recorded."
        />
      ) : (
        <div className="space-y-5">
          <dl className="grid gap-3 sm:grid-cols-2">
            <div className="rounded-lg bg-emerald-50/60 px-4 py-3 ring-1 ring-emerald-100">
              <dt className="text-xs font-medium text-emerald-700">Collected this month</dt>
              <dd className="mt-1 text-xl font-semibold text-slate-900 tabular-nums">{money(data.collected_this_month)}</dd>
            </div>
            <div className="rounded-lg bg-amber-50/60 px-4 py-3 ring-1 ring-amber-100">
              <dt className="text-xs font-medium text-amber-700">Outstanding</dt>
              <dd className="mt-1 text-xl font-semibold text-slate-900 tabular-nums">{money(data.outstanding)}</dd>
            </div>
          </dl>
          {hasHistory ? (
            <div className="h-56" role="img" aria-label="Bar chart of monthly fee collection">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={monthly} margin={{ top: 4, right: 4, bottom: 0, left: 0 }}>
                  <CartesianGrid {...gridProps} />
                  <XAxis dataKey="month" tickFormatter={(m) => formatMonth(m)} {...axisProps} />
                  <YAxis width={72} tickFormatter={(v) => formatCurrencyCompact(v, data.currency)} {...axisProps} />
                  <Tooltip
                    cursor={{ fill: '#f1f5f9' }}
                    content={<ChartTooltip labelFormatter={(m) => formatMonth(m, 'long')} valueFormatter={money} />}
                  />
                  <Bar dataKey="amount" name="Collected" fill={BRAND} radius={[4, 4, 0, 0]} maxBarSize={36} />
                </BarChart>
              </ResponsiveContainer>
            </div>
          ) : (
            <NoData icon={Banknote} title="No payments recorded yet" description="The collection trend appears after the first payments." />
          )}
        </div>
      )}
    </Panel>
  )
}
