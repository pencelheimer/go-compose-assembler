import { useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { ArrowLeft, Check, Plus } from 'lucide-react'
import { componentQuery, presetQuery } from '@/api/queries'
import { CodeView } from '@/components/CodeView'
import { RouteError } from '@/components/RouteError'
import { toYaml } from '@/lib/highlight'
import { secretStore } from '@/lib/secrets'
import { isSecret, mergeValues, presetAsValues, splitSecrets, type BuilderSearch } from '@/lib/stack'
import { builderSearch } from '@/lib/useUrlValues'

export const Route = createFileRoute('/catalog/components/$id')({
  loader: ({ context: { queryClient }, params }) => queryClient.ensureQueryData(componentQuery(params.id)),
  errorComponent: RouteError,
  component: ComponentPage,
})

function ComponentPage() {
  const { id } = Route.useParams()
  const { data: def } = useSuspenseQuery(componentQuery(id))
  const queryClient = useQueryClient()
  const navigate = useNavigate()

  const last = builderSearch.get()
  const inStack = last.services?.includes(id) ?? false

  // Adds this component to the stack the builder last showed; a preset turns
  // into a custom stack, as it does in the builder.
  const addToStack = async () => {
    let next: BuilderSearch
    if (last.preset) {
      const preset = await queryClient.ensureQueryData(presetQuery(last.preset))
      const ids = preset.services.map((s) => s.component)
      const { visible, secret } = splitSecrets(presetAsValues(preset))
      secretStore.seed(secret)
      next = { services: ids.includes(id) ? ids : [...ids, id], values: mergeValues(visible, last.values), base: preset.id }
    } else {
      const ids = last.services ?? []
      next = { ...last, services: ids.includes(id) ? ids : [...ids, id] }
    }
    await navigate({ to: '/', search: next })
  }

  const { extra, ...service } = def.service
  const template = toYaml({ ...service, ...extra })
  const topLevel = toYaml({
    ...(def.volumes?.length ? { volumes: Object.fromEntries(def.volumes.map((v) => [v.name, {}])) } : {}),
    ...(def.networks?.length ? { networks: Object.fromEntries(def.networks.map((n) => [n.name, {}])) } : {}),
  })

  return (
    <div className="page">
      <Link to="/catalog" className="back">
        <ArrowLeft size={14} /> Catalog
      </Link>
      <header className="page-head with-actions">
        <div>
          <p className="eyebrow">Component</p>
          <h1 className="mono">{def.id}</h1>
          <p className="muted">{def.description}</p>
        </div>
        <button type="button" className="btn primary" onClick={() => void addToStack()} disabled={inStack}>
          {inStack ? <Check size={15} /> : <Plus size={15} />} {inStack ? 'In your stack' : 'Add to stack'}
        </button>
      </header>

      <div className="detail-grid">
        <section>
          <h2 className="section-title">Parameters</h2>
          {def.params.length === 0 ? (
            <p className="muted">This component takes no parameters.</p>
          ) : (
            <div className="table-wrap">
              <table className="table">
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Type</th>
                    <th>Default</th>
                    <th>Description</th>
                  </tr>
                </thead>
                <tbody>
                  {def.params.map((p) => (
                    <tr key={p.name}>
                      <td className="mono">
                        {p.name}
                        {p.required && <span className="req">*</span>}
                        {isSecret(p.name) && <span className="tag">secret</span>}
                      </td>
                      <td>
                        <span className="tag">{p.type === 'list' ? `list → ${p.target}` : 'scalar'}</span>
                      </td>
                      <td className="mono">{p.default ?? (p.required ? <span className="muted">required</span> : '—')}</td>
                      <td className="muted">{p.description}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          <p className="muted small">
            Scalars go to <code>.env</code> and are referenced as <code>{'${NAME}'}</code>; lists are merged into the
            service by their target.
          </p>
        </section>

        <section>
          <h2 className="section-title">Service template</h2>
          <div className="card code-card">
            <CodeView text={`${def.id}:\n${template.replace(/^/gm, '  ')}`} lang="yaml" label="Service template" />
          </div>
          {topLevel.trim() !== '{}' && (
            <>
              <h3 className="section-subtitle">Adds at top level</h3>
              <div className="card code-card">
                <CodeView text={topLevel} lang="yaml" label="Top-level volumes and networks" />
              </div>
            </>
          )}
        </section>
      </div>
    </div>
  )
}
