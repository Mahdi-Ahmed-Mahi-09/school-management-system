/** Compact, card-styled tooltip used by every chart. */
export function ChartTooltip({ active, payload, label, labelFormatter, valueFormatter = (v) => v }) {
  if (!active || !payload?.length) return null
  return (
    <div className="min-w-36 rounded-lg border border-slate-200 bg-white px-3 py-2 text-[13px] shadow-overlay">
      {label != null && <p className="mb-1.5 font-medium text-slate-900">{labelFormatter ? labelFormatter(label) : label}</p>}
      <ul className="space-y-1">
        {payload.map((p) => (
          <li key={p.dataKey ?? p.name} className="flex items-center justify-between gap-4">
            <span className="flex items-center gap-2 text-slate-600">
              <span className="size-2 rounded-full" style={{ background: p.color ?? p.payload?.fill }} aria-hidden="true" />
              {p.name}
            </span>
            <span className="font-medium text-slate-900 tabular-nums">{valueFormatter(p.value)}</span>
          </li>
        ))}
      </ul>
    </div>
  )
}

/** Legend rendered as HTML for better typography and wrapping. */
export function ChartLegend({ items }) {
  return (
    <ul className="flex flex-wrap items-center gap-x-4 gap-y-1.5">
      {items.map((item) => (
        <li key={item.label} className="flex items-center gap-1.5 text-xs text-slate-600">
          <span className="size-2 rounded-full" style={{ background: item.color }} aria-hidden="true" />
          {item.label}
        </li>
      ))}
    </ul>
  )
}
