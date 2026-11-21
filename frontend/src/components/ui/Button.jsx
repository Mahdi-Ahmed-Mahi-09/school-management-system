import { buttonClasses } from './buttonStyles'
import { Spinner } from './Spinner'

export function Button({
  variant = 'primary',
  size = 'md',
  loading = false,
  icon: Icon,
  iconRight: IconRight,
  className,
  children,
  disabled,
  type = 'button',
  ...props
}) {
  return (
    <button
      type={type}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      className={buttonClasses(variant, size, className)}
      {...props}
    >
      {loading ? <Spinner /> : Icon && <Icon className="size-4" aria-hidden="true" />}
      {children}
      {IconRight && !loading && <IconRight className="size-4" aria-hidden="true" />}
    </button>
  )
}
