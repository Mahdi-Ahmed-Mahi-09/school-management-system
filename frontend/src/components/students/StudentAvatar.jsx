import { useState } from 'react'
import { cn } from '../../lib/utils'
import { Avatar } from '../ui/Display'

const sizes = { sm: 'size-8', md: 'size-10', lg: 'size-14', xl: 'size-24' }

function nameParts(fullName = '') {
  const parts = fullName.trim().split(/\s+/)
  return { first_name: parts[0] ?? '', last_name: parts.length > 1 ? parts[parts.length - 1] : '' }
}

/** Student photo (served by the authenticated API) with an initials fallback. */
export function StudentAvatar({ student, size = 'md', className }) {
  const [failedUrl, setFailedUrl] = useState(null)
  const url = student?.photo_url
  if (url && failedUrl !== url) {
    return (
      <img
        src={url}
        alt=""
        loading="lazy"
        onError={() => setFailedUrl(url)}
        className={cn('shrink-0 rounded-full bg-slate-100 object-cover ring-1 ring-slate-900/5', sizes[size], className)}
      />
    )
  }
  const user = { id: student?.id, ...nameParts(student?.full_name) }
  return <Avatar user={user} size={size} className={cn(size === 'xl' && 'size-24 text-3xl', className)} />
}
