import { Link } from 'react-router-dom'
import { ChevronRight } from 'lucide-react'

/** items: [{ label, to? }] — the last item is the current page. */
export function Breadcrumbs({ items }) {
  return (
    <nav aria-label="Breadcrumb">
      <ol className="flex flex-wrap items-center gap-1.5 text-[13px] text-slate-500">
        {items.map((item, i) => {
          const last = i === items.length - 1
          return (
            <li key={`${item.label}-${i}`} className="flex min-w-0 items-center gap-1.5">
              {item.to && !last ? (
                <Link to={item.to} className="truncate font-medium transition-colors hover:text-slate-900">
                  {item.label}
                </Link>
              ) : (
                <span className="truncate font-medium text-slate-700" aria-current={last ? 'page' : undefined}>
                  {item.label}
                </span>
              )}
              {!last && <ChevronRight className="size-3.5 shrink-0 text-slate-300" aria-hidden="true" />}
            </li>
          )
        })}
      </ol>
    </nav>
  )
}
