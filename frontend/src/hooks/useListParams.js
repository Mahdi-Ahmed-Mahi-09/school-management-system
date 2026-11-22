import { useCallback, useEffect, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'

/** Debounced copy of value, plus a setter to update it immediately. */
function useDebounced(value, delay) {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const id = setTimeout(() => setDebounced(value), delay)
    return () => clearTimeout(id)
  }, [value, delay])
  return [debounced, setDebounced]
}

/**
 * URL-backed state for list pages (search, filters, sorting, page), so lists
 * are shareable and survive reloads.
 *
 * - `updateParams` composes consecutive updates. React Router's functional
 *   setSearchParams computes from the current render's params, so two updates
 *   in one tick would otherwise overwrite each other.
 * - The search box is debounced into `?q=`; `clearSearch` also resets the
 *   debounced value so re-typing the same term after clearing is detected.
 */
export function useListParams({ debounce = 300 } = {}) {
  const [params, setParams] = useSearchParams()

  const latestParams = useRef(params)
  useEffect(() => {
    latestParams.current = params
  }, [params])

  const updateParams = useCallback(
    (changes, { resetPage = true } = {}) => {
      const next = new URLSearchParams(latestParams.current)
      for (const [k, v] of Object.entries(changes)) {
        if (v === '' || v == null) next.delete(k)
        else next.set(k, String(v))
      }
      if (resetPage) next.delete('page')
      latestParams.current = next
      setParams(next, { replace: true })
    },
    [setParams],
  )

  const [searchInput, setSearchInput] = useState(params.get('q') ?? '')
  const [search, setSearchNow] = useDebounced(searchInput.trim(), debounce)

  // Keep the debounced search in the URL, only when it actually changes.
  const lastSearch = useRef(search)
  useEffect(() => {
    if (search === lastSearch.current) return
    lastSearch.current = search
    updateParams({ q: search })
  }, [search, updateParams])

  const clearSearch = useCallback(() => {
    setSearchInput('')
    setSearchNow('')
  }, [setSearchNow])

  const page = Math.max(1, Number(params.get('page')) || 1)

  return { params, updateParams, searchInput, setSearchInput, clearSearch, page }
}
