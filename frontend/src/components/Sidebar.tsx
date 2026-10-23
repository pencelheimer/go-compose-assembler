import { Link } from '@tanstack/react-router'
import { Check, FileWarning, Info, Layers, Plus, Search, SquareDashed } from 'lucide-react'
import { useMemo, useState } from 'react'
import type { ComponentSummary, ListResponse, PresetSummary } from '@/api/types'

interface SidebarProps {
  presets?: ListResponse<PresetSummary>
  components?: ListResponse<ComponentSummary>
  activePreset?: string
  custom: boolean
  inStack: string[]
  onAdd: (id: string) => void
}

export function Sidebar({ presets, components, activePreset, custom, inStack, onAdd }: SidebarProps) {
  const [query, setQuery] = useState('')
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    const items = components?.items ?? []
    return q ? items.filter((c) => c.id.includes(q) || c.description.toLowerCase().includes(q)) : items
  }, [components, query])
  const skipped = [...(presets?.skipped ?? []), ...(components?.skipped ?? [])]

  return (
    <aside className="sidebar" aria-label="Catalog">
      <section>
        <h2 className="eyebrow">Start from</h2>
        <nav className="preset-list" aria-label="Presets">
          {presets?.items.map((p) => (
            <Link
              key={p.id}
              to="/"
              search={{ preset: p.id }}
              className="preset-item"
              data-active={activePreset === p.id || undefined}
              aria-current={activePreset === p.id ? 'page' : undefined}
            >
              <Layers size={15} aria-hidden />
              <span>
                <span className="mono preset-name">{p.id}</span>
                <span className="preset-desc">{p.description}</span>
              </span>
            </Link>
          ))}
          <Link
            to="/"
            search={{ services: [] }}
            className="preset-item"
            data-active={(custom && inStack.length === 0) || undefined}
          >
            <SquareDashed size={15} aria-hidden />
            <span>
              <span className="preset-name">Empty stack</span>
              <span className="preset-desc">Pick components yourself</span>
            </span>
          </Link>
        </nav>
      </section>

      <section className="components-section">
        <h2 className="eyebrow">Components</h2>
        <label className="search">
          <Search size={14} aria-hidden />
          <span className="sr-only">Filter components</span>
          <input
            type="search"
            placeholder="Filter…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </label>
        <ul className="component-list" role="list">
          {filtered.map((c) => {
            const added = inStack.includes(c.id)
            return (
              <li key={c.id} className="component-item">
                <button
                  type="button"
                  className="component-add"
                  disabled={added}
                  onClick={() => onAdd(c.id)}
                  title={added ? 'Already in the stack' : `Add ${c.id}`}
                >
                  <span className="add-icon" data-added={added || undefined} aria-hidden>
                    {added ? <Check size={13} /> : <Plus size={13} />}
                  </span>
                  <span className="component-text">
                    <span className="mono">{c.id}</span>
                    <span className="component-desc">{c.description}</span>
                  </span>
                </button>
                <Link
                  to="/catalog/components/$id"
                  params={{ id: c.id }}
                  className="icon-btn sm"
                  title={`About ${c.id}`}
                >
                  <Info size={14} />
                  <span className="sr-only">About {c.id}</span>
                </Link>
              </li>
            )
          })}
          {components && filtered.length === 0 && <li className="muted small pad">No matches.</li>}
        </ul>
      </section>

      {skipped.length > 0 && (
        <details className="skipped">
          <summary>
            <FileWarning size={14} aria-hidden /> {skipped.length} file{skipped.length > 1 ? 's' : ''} skipped
          </summary>
          <ul role="list">
            {skipped.map((s) => (
              <li key={s} className="mono">
                {s}
              </li>
            ))}
          </ul>
        </details>
      )}
    </aside>
  )
}
