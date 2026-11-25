import { useCallback, useEffect, useState } from 'react'
import { getErrorMessage } from '../../api/client'
import { attendanceApi } from '../../api/services'

/**
 * Academic years and the sections the signed-in user can take attendance for
 * (administrators: every section; teachers: their assigned sections). With no
 * year given the server picks the current year.
 */
export function useAttendanceOptions(yearId) {
  const [state, setState] = useState({ loading: true, error: null, data: null })
  const [reloadKey, setReloadKey] = useState(0)
  const reload = useCallback(() => setReloadKey((k) => k + 1), [])

  useEffect(() => {
    let cancelled = false
    setState((s) => ({ ...s, loading: true, error: null }))
    attendanceApi
      .options(yearId ? { academic_year_id: yearId } : undefined)
      .then((data) => !cancelled && setState({ loading: false, error: null, data }))
      .catch((err) => !cancelled && setState({ loading: false, error: getErrorMessage(err), data: null }))
    return () => {
      cancelled = true
    }
  }, [yearId, reloadKey])

  return { ...state, reload }
}
