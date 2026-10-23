import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { ArrowRight, Box, Layers } from 'lucide-react'
import { componentsQuery, presetsQuery } from '@/api/queries'

export const Route = createFileRoute('/catalog/')({
  loader: ({ context: { queryClient } }) =>
    Promise.all([queryClient.ensureQueryData(componentsQuery), queryClient.ensureQueryData(presetsQuery)]),
  component: Catalog,
})

function Catalog() {
  const { data: presets } = useSuspenseQuery(presetsQuery)
  const { data: components } = useSuspenseQuery(componentsQuery)

  return (
    <div className="page">
      <header className="page-head">
        <p className="eyebrow">Catalog</p>
        <h1>Presets and components</h1>
        <p className="muted">
          Built-in definitions, plus anything in <code>./assembler-presets/</code> or your user config directory. A
          local file replaces the built-in one with the same name.
        </p>
      </header>

      <h2 className="section-title">
        <Layers size={16} aria-hidden /> Presets <span className="count">{presets.items.length}</span>
      </h2>
      <div className="grid">
        {presets.items.map((p) => (
          <Link key={p.id} to="/catalog/presets/$id" params={{ id: p.id }} className="card tile">
            <span className="mono tile-name">{p.id}</span>
            <span className="muted small">{p.description}</span>
            <ArrowRight size={15} className="tile-arrow" aria-hidden />
          </Link>
        ))}
      </div>

      <h2 className="section-title">
        <Box size={16} aria-hidden /> Components <span className="count">{components.items.length}</span>
      </h2>
      <div className="grid">
        {components.items.map((c) => (
          <Link key={c.id} to="/catalog/components/$id" params={{ id: c.id }} className="card tile">
            <span className="mono tile-name">{c.id}</span>
            <span className="muted small">{c.description}</span>
            <ArrowRight size={15} className="tile-arrow" aria-hidden />
          </Link>
        ))}
      </div>

      {[...presets.skipped, ...components.skipped].length > 0 && (
        <div className="alert warn">
          <p>Some files were skipped:</p>
          <ul role="list">
            {[...presets.skipped, ...components.skipped].map((s) => (
              <li key={s} className="mono small">
                {s}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}
