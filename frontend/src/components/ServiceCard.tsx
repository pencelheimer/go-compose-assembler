import { Link } from '@tanstack/react-router'
import { ChevronDown, Info, Plus, TriangleAlert, X } from 'lucide-react'
import { useState } from 'react'
import type { ComponentParam } from '@/api/types'
import { effectiveValue, isEdited, type EditValue, type StackEntry, type Values } from '@/lib/stack'
import { ParamField, type Origin } from './ParamField'

interface ServiceCardProps {
  entry: StackEntry
  edits: Values
  fieldErrors: Record<string, string>
  flaggedFields: Set<string>
  issues: string[]
  missingDeps: string[]
  incomplete: boolean
  onChange: (param: string, v: EditValue) => void
  onReset: (param: string) => void
  onRemove: () => void
  onAdd: (id: string) => void
}

export function ServiceCard(props: ServiceCardProps) {
  const { entry, edits, fieldErrors, flaggedFields, issues, missingDeps, incomplete } = props
  const [collapsed, setCollapsed] = useState(false)
  const [showOptional, setShowOptional] = useState(false)

  const key = (p: ComponentParam) => `${entry.id}.${p.name}`
  const origin = (p: ComponentParam): Origin => {
    if (isEdited(entry, p, edits)) return 'edited'
    if (entry.presetParams?.[p.name] != null) return 'preset'
    return p.default ? 'default' : 'none'
  }
  const needsAttention = (p: ComponentParam) => Boolean(fieldErrors[key(p)]) || flaggedFields.has(key(p))

  const required = entry.def.params.filter((p) => p.required)
  const optional = entry.def.params.filter((p) => !p.required)
  const optionalOpen =
    showOptional || optional.some((p) => needsAttention(p) || origin(p) === 'edited' || origin(p) === 'preset')
  const hasProblem = issues.length > 0 || entry.def.params.some((p) => fieldErrors[key(p)])
  const status = hasProblem ? 'error' : incomplete ? 'pending' : 'ok'
  const open = !collapsed || hasProblem

  const field = (p: ComponentParam) => (
    <ParamField
      key={p.name}
      componentId={entry.id}
      param={p}
      value={effectiveValue(entry, p, edits)}
      origin={origin(p)}
      error={fieldErrors[key(p)]}
      flagged={flaggedFields.has(key(p))}
      onChange={(v) => props.onChange(p.name, v)}
      onReset={() => props.onReset(p.name)}
    />
  )

  return (
    <article className="card service" data-status={status} aria-labelledby={`svc-${entry.id}`}>
      <header className="service-head">
        <button
          type="button"
          className="service-toggle"
          aria-expanded={open}
          onClick={() => setCollapsed((c) => !c)}
        >
          <span className="status-dot" data-status={status} aria-hidden />
          <span className="service-title">
            <span id={`svc-${entry.id}`} className="mono service-name">
              {entry.id}
            </span>
            <span className="service-desc">{entry.def.description}</span>
          </span>
          <ChevronDown size={16} className="chevron" data-open={open || undefined} aria-hidden />
        </button>
        <Link
          to="/catalog/components/$id"
          params={{ id: entry.id }}
          className="icon-btn"
          title="Component details"
        >
          <Info size={15} />
          <span className="sr-only">Details for {entry.id}</span>
        </Link>
        <button type="button" className="icon-btn" onClick={props.onRemove} title="Remove from stack">
          <X size={16} />
          <span className="sr-only">Remove {entry.id}</span>
        </button>
      </header>

      {open && (
        <div className="service-body">
          {entry.def.service.image && (
            <p className="service-image mono" title="Image">
              {entry.def.service.image}
            </p>
          )}

          {issues.length > 0 && (
            <ul className="issues" role="list">
              {issues.map((msg) => (
                <li key={msg}>
                  <TriangleAlert size={14} aria-hidden /> {msg}
                </li>
              ))}
            </ul>
          )}

          {required.length > 0 && <div className="fields">{required.map(field)}</div>}

          {optional.length > 0 && (
            <div className="optional">
              <button
                type="button"
                className="optional-toggle"
                aria-expanded={optionalOpen}
                onClick={() => setShowOptional(!optionalOpen)}
              >
                <ChevronDown size={14} className="chevron" data-open={optionalOpen || undefined} aria-hidden />
                Optional <span className="count">{optional.length}</span>
              </button>
              {optionalOpen && <div className="fields">{optional.map(field)}</div>}
            </div>
          )}

          {entry.def.params.length === 0 && <p className="muted small">No parameters.</p>}

          {missingDeps.length > 0 && (
            <div className="deps">
              <span>Depends on</span>
              {missingDeps.map((d) => (
                <button key={d} type="button" className="btn ghost sm" onClick={() => props.onAdd(d)}>
                  <Plus size={13} /> <span className="mono">{d}</span>
                </button>
              ))}
              <span className="muted">— not in the stack</span>
            </div>
          )}
        </div>
      )}
    </article>
  )
}
