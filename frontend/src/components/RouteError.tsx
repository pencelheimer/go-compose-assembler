import { Link, type ErrorComponentProps } from '@tanstack/react-router'
import { ApiError } from '@/api/client'

/** Error screen for catalog routes; a 404 from the API reads as "not found". */
export function RouteError({ error }: ErrorComponentProps) {
  const notFound = error instanceof ApiError && error.status === 404
  const message = error instanceof Error ? error.message : String(error)
  return (
    <div className="page narrow">
      <p className="eyebrow">{notFound ? 'Not found' : 'Error'}</p>
      <h1>{notFound ? 'This definition does not exist' : 'Something went wrong'}</h1>
      <p className="alert danger">{message}</p>
      <p>
        <Link to="/catalog">Back to the catalog</Link>
      </p>
    </div>
  )
}
