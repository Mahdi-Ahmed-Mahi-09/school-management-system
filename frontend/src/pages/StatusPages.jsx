import { Link } from 'react-router-dom'
import { Compass, ShieldX } from 'lucide-react'
import { Wordmark } from '../components/brand/Logo'
import { buttonClasses } from '../components/ui/buttonStyles'

function StatusScreen({ icon: Icon, code, title, description, standalone }) {
  const body = (
    <div className="mx-auto flex max-w-md flex-col items-center py-20 text-center animate-fade-in">
      <div className="flex size-14 items-center justify-center rounded-2xl border border-slate-200 bg-white shadow-card">
        <Icon className="size-6 text-slate-500" aria-hidden="true" />
      </div>
      <p className="mt-6 text-sm font-semibold text-brand-600">{code}</p>
      <h1 className="mt-2 text-2xl font-semibold tracking-tight text-slate-900">{title}</h1>
      <p className="mt-2 text-sm text-slate-500">{description}</p>
      <Link to="/" className={buttonClasses('secondary', 'md', 'mt-8')}>
        Back to dashboard
      </Link>
    </div>
  )
  if (!standalone) return body
  return (
    <div className="flex min-h-svh flex-col bg-slate-50 px-6 py-8">
      <Wordmark />
      <div className="flex flex-1 items-center">{body}</div>
    </div>
  )
}

export function ForbiddenPage() {
  return (
    <StatusScreen
      icon={ShieldX}
      code="403"
      title="You don't have access to this page"
      description="Your role doesn't include permission to view this area. If you think this is a mistake, contact your administrator."
    />
  )
}

export function NotFoundPage({ standalone = false }) {
  return (
    <StatusScreen
      icon={Compass}
      code="404"
      title="Page not found"
      description="The page you're looking for doesn't exist or has moved."
      standalone={standalone}
    />
  )
}
