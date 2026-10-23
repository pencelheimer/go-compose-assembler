import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { ArrowLeft, ArrowRight } from 'lucide-react'
import { presetQuery } from '@/api/queries'
import { RouteError } from '@/components/RouteError'
import { isSecret, toText } from '@/lib/stack'

export const Route = createFileRoute('/catalog/presets/$id')({
  loader: ({ context: { queryClient }, params }) => queryClient.ensureQueryData(presetQuery(params.id)),
  errorComponent: RouteError,
  component: PresetPage,
})

function PresetPage() {
  const { id } = Route.useParams()
  const { data: preset } = useSuspenseQuery(presetQuery(id))

  return (
    <div className="page">
      <Link to="/catalog" className="back">
        <ArrowLeft size={14} /> Catalog
      </Link>
      <header className="page-head with-actions">
        <div>
          <p className="eyebrow">Preset</p>
          <h1 className="mono">{preset.id}</h1>
          <p className="muted">{preset.description}</p>
        </div>
        <Link to="/" search={{ preset: preset.id }} className="btn primary">
          Open in builder <ArrowRight size={15} />
        </Link>
      </header>

      <h2 className="section-title">
        Services <span className="count">{preset.services.length}</span>
      </h2>
      <div className="grid wide">
        {preset.services.map((s) => {
          const params = Object.entries(s.params ?? {})
          return (
            <article key={s.component} className="card tile static">
              <Link to="/catalog/components/$id" params={{ id: s.component }} className="mono tile-name">
                {s.component}
              </Link>
              {params.length ? (
                <dl className="kv">
                  {params.map(([k, v]) => (
                    <div key={k}>
                      <dt className="mono">{k}</dt>
                      <dd className="mono">{isSecret(k) ? '••••••••' : toText(v) || '—'}</dd>
                    </div>
                  ))}
                </dl>
              ) : (
                <p className="muted small">Uses component defaults.</p>
              )}
            </article>
          )
        })}
      </div>
    </div>
  )
}
