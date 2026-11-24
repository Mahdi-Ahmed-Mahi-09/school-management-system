import { useEffect, useId, useState } from 'react'
import { Search, X } from 'lucide-react'
import { getErrorMessage } from '../../api/client'
import { enrollmentsApi } from '../../api/services'
import { cn } from '../../lib/utils'
import { PersonAvatar } from '../ui/PersonAvatar'
import { Spinner } from '../ui/Spinner'

/**
 * Searches existing, active students who are not yet enrolled in `yearId`
 * (from Student Management) and picks one. `value` is a student ref.
 */
export function StudentPicker({ yearId, value, onChange, error, disabled }) {
  const id = useId()
  const [query, setQuery] = useState('')
  const [state, setState] = useState({ loading: false, error: null, students: [], total: 0 })

  useEffect(() => {
    if (!yearId || value) return
    const controller = new AbortController()
    const t = setTimeout(() => {
      setState((s) => ({ ...s, loading: true, error: null }))
      enrollmentsApi
        .unenrolled({ academic_year_id: yearId, search: query.trim() || undefined, page_size: 8 }, { signal: controller.signal })
        .then(({ students, meta }) => setState({ loading: false, error: null, students, total: meta.total }))
        .catch((err) => !controller.signal.aborted && setState({ loading: false, error: getErrorMessage(err), students: [], total: 0 }))
    }, 250)
    return () => {
      clearTimeout(t)
      controller.abort()
    }
  }, [yearId, query, value])

  if (value) {
    return (
      <div>
        <p className="mb-1.5 text-sm font-medium text-slate-700">
          Student <span className="text-rose-500">*</span>
        </p>
        <div className="flex items-center gap-3 rounded-lg border border-slate-200 bg-slate-50/60 px-3 py-2.5">
          <PersonAvatar person={value} size="sm" />
          <span className="min-w-0 flex-1">
            <span className="block truncate text-sm font-medium text-slate-900">{value.full_name}</span>
            <span className="block font-mono text-xs text-slate-500">{value.admission_number}</span>
          </span>
          {!disabled && (
            <button
              type="button"
              onClick={() => onChange(null)}
              className="flex size-8 items-center justify-center rounded-lg text-slate-400 hover:bg-white hover:text-slate-700"
              aria-label="Choose another student"
            >
              <X className="size-4" />
            </button>
          )}
        </div>
      </div>
    )
  }

  return (
    <div>
      <label htmlFor={id} className="mb-1.5 block text-sm font-medium text-slate-700">
        Student <span className="text-rose-500">*</span>
      </label>
      <div className="relative">
        <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
        <input
          id={id}
          type="search"
          value={query}
          disabled={!yearId || disabled}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={yearId ? 'Search by name or student ID' : 'Choose an academic year first'}
          className={cn(
            'w-full rounded-lg border bg-white py-2 pr-3 pl-9 text-sm shadow-sm placeholder:text-slate-400 focus:ring-4 focus:outline-none disabled:bg-slate-50',
            error ? 'border-rose-400 focus:border-rose-500 focus:ring-rose-500/15' : 'border-slate-300 focus:border-brand-500 focus:ring-brand-500/15',
          )}
        />
      </div>
      {error && <p className="mt-1.5 text-[13px] text-rose-600">{error}</p>}
      {yearId && (
        <div className="mt-2 rounded-lg border border-slate-200/80">
          {state.loading && !state.students.length ? (
            <p className="flex items-center gap-2 px-3 py-3 text-sm text-slate-500">
              <Spinner className="size-4" /> Searching…
            </p>
          ) : state.error ? (
            <p className="px-3 py-3 text-sm text-rose-600">{state.error}</p>
          ) : state.students.length === 0 ? (
            <p className="px-3 py-3 text-sm text-slate-500">
              {query ? `No unenrolled students match “${query}”.` : 'Every active student is already enrolled in this year.'}
            </p>
          ) : (
            <ul className="max-h-60 divide-y divide-slate-100 overflow-y-auto" aria-label="Students not yet enrolled">
              {state.students.map((s) => (
                <li key={s.id}>
                  <button type="button" onClick={() => onChange(s)} className="flex w-full items-center gap-3 px-3 py-2 text-left hover:bg-brand-50/50">
                    <PersonAvatar person={s} size="sm" />
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-sm font-medium text-slate-900">{s.full_name}</span>
                      <span className="block font-mono text-xs text-slate-500">{s.admission_number}</span>
                    </span>
                  </button>
                </li>
              ))}
              {state.total > state.students.length && (
                <li className="px-3 py-2 text-xs text-slate-500">Showing {state.students.length} of {state.total}. Refine the search to find others.</li>
              )}
            </ul>
          )}
        </div>
      )}
    </div>
  )
}
