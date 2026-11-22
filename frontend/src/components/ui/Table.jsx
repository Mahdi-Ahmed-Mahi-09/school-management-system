import { ArrowDown, ArrowUp, ChevronsUpDown, MoreHorizontal } from 'lucide-react'
import { cn } from '../../lib/utils'
import { Menu } from './Menu'

/** Sortable table header cell. */
export function SortHeader({ label, field, sort, order, onSort, className }) {
  const active = sort === field
  const Icon = !active ? ChevronsUpDown : order === 'asc' ? ArrowUp : ArrowDown
  return (
    <th scope="col" className={cn('px-4 py-3 text-left', className)} aria-sort={active ? (order === 'asc' ? 'ascending' : 'descending') : 'none'}>
      <button
        type="button"
        onClick={() => onSort(field)}
        className={cn('inline-flex items-center gap-1 rounded transition-colors hover:text-slate-900', active && 'text-slate-900')}
      >
        {label}
        <Icon className={cn('size-3.5', active ? 'text-slate-600' : 'text-slate-300')} aria-hidden="true" />
      </button>
    </th>
  )
}

/** "More" button that opens a row's action menu. */
export function RowActionsMenu({ label, items }) {
  return (
    <Menu
      label={label}
      items={items}
      trigger={(props) => (
        <button
          {...props}
          type="button"
          className="flex size-8 items-center justify-center rounded-lg text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700 focus-visible:ring-2 focus-visible:ring-brand-500/40 aria-expanded:bg-slate-100 aria-expanded:text-slate-700"
        >
          <MoreHorizontal className="size-4" />
        </button>
      )}
    />
  )
}

/** Search input used in list toolbars. */
export function SearchInput({ value, onChange, placeholder, label, icon: Icon }) {
  return (
    <div className="relative flex-1">
      <Icon className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
      <input
        type="search"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        aria-label={label}
        maxLength={100}
        className="h-10 w-full rounded-lg bg-white pr-3 pl-9 text-sm shadow-sm ring-1 ring-slate-200 ring-inset placeholder:text-slate-400 hover:ring-slate-300 focus:ring-2 focus:ring-brand-500 focus:outline-none"
      />
    </div>
  )
}
