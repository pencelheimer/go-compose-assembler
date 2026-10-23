import { Check, CircleAlert, Copy, Download, Eye, EyeOff, FileArchive, Loader, TriangleAlert } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import type { Bundle, ErrorDetail } from '@/api/types'
import { downloadText, downloadZip } from '@/lib/download'
import { useCopy } from '@/lib/hooks'
import { isSecret } from '@/lib/stack'
import { CodeView } from './CodeView'
import { fieldId } from './ParamField'

export type PreviewStatus = 'empty' | 'loading' | 'incomplete' | 'generating' | 'invalid' | 'warnings' | 'valid'

interface PreviewPanelProps {
  status: PreviewStatus
  bundle?: Bundle
  stale: boolean
  errors: ErrorDetail[]
  missing: { component: string; param: string }[]
}

const statusText: Record<PreviewStatus, string> = {
  empty: 'Nothing to generate',
  loading: 'Loading definitions',
  incomplete: 'Needs input',
  generating: 'Generating',
  invalid: 'Invalid',
  warnings: 'Valid, with warnings',
  valid: 'Valid',
}

function focusField(component: string, param: string) {
  // The card may be rendering the field right now; wait a frame.
  requestAnimationFrame(() => {
    const el = document.getElementById(fieldId(component, param))
    el?.scrollIntoView({ block: 'center', behavior: 'smooth' })
    el?.focus({ preventScroll: true })
  })
}

function ErrorItem({ d }: { d: ErrorDetail }) {
  const target = d.component && d.param ? { component: d.component, param: d.param } : null
  let body: ReactNode = d.message
  if (d.type === 'cyclic_dependency' && d.chain?.length) {
    body = (
      <>
        Dependency cycle: <span className="mono">{[...d.chain, d.chain[0]].join(' → ')}</span>
      </>
    )
  } else if ((d.type === 'invalid_component' || d.type === 'invalid_preset') && d.location) {
    body = (
      <>
        {d.message}
        <span className="hint">Fix the YAML file and the preview refreshes on its own.</span>
      </>
    )
  }
  return (
    <li>
      <CircleAlert size={15} aria-hidden />
      <span>
        {body}
        {target && (
          <button type="button" className="link-btn" onClick={() => focusField(target.component, target.param)}>
            Go to field
          </button>
        )}
      </span>
    </li>
  )
}

export function PreviewPanel({ status, bundle, stale, errors, missing }: PreviewPanelProps) {
  const [tab, setTab] = useState<'compose' | 'env'>('compose')
  const [reveal, setReveal] = useState(false)
  const { copied, copy } = useCopy()
  const text = bundle ? (tab === 'compose' ? bundle.compose : bundle.env) : ''
  const canExport = Boolean(bundle) && !stale

  return (
    <section className="preview" aria-label="Preview">
      <div className="preview-head">
        <div className="tabs" role="tablist" aria-label="Generated file">
          {(['compose', 'env'] as const).map((t) => (
            <button
              key={t}
              type="button"
              role="tab"
              aria-selected={tab === t}
              className="tab mono"
              onClick={() => setTab(t)}
            >
              {t === 'compose' ? 'compose.yaml' : '.env'}
            </button>
          ))}
        </div>
        <span className="pill" data-status={status} role="status">
          {status === 'generating' || status === 'loading' ? (
            <Loader size={13} className="spin" aria-hidden />
          ) : (
            <span className="status-dot" data-status={status} aria-hidden />
          )}
          {statusText[status]}
        </span>
      </div>

      {errors.length > 0 && (
        <ul className="alert danger" role="list">
          {errors.map((d, i) => (
            <ErrorItem key={`${d.type}-${i}`} d={d} />
          ))}
        </ul>
      )}

      {status === 'incomplete' && missing.length > 0 && (
        <div className="alert info">
          <p>Fill in the required fields to generate:</p>
          <ul className="missing" role="list">
            {missing.map((m) => (
              <li key={`${m.component}.${m.param}`}>
                <button type="button" className="link-btn mono" onClick={() => focusField(m.component, m.param)}>
                  {m.component}.{m.param}
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}

      {!stale && bundle?.warnings && bundle.warnings.length > 0 && (
        <ul className="alert warn" role="list">
          {bundle.warnings.map((w) => (
            <li key={w}>
              <TriangleAlert size={15} aria-hidden /> <span>{w}</span>
            </li>
          ))}
        </ul>
      )}

      <div className="preview-code">
        {bundle ? (
          <>
            {stale && <span className="stale-badge">Outdated — showing last valid result</span>}
            <CodeView
              text={text}
              lang={tab === 'compose' ? 'yaml' : 'env'}
              mask={tab === 'env' && !reveal ? isSecret : undefined}
              dimmed={stale}
              label={tab === 'compose' ? 'compose.yaml' : '.env'}
            />
          </>
        ) : (
          <div className="preview-empty">
            <p>{status === 'empty' ? 'Add services to see the generated files here.' : 'The preview appears once the stack is valid.'}</p>
          </div>
        )}
      </div>

      <div className="preview-actions">
        {tab === 'env' && (
          <button type="button" className="btn ghost sm" onClick={() => setReveal((r) => !r)} disabled={!bundle}>
            {reveal ? <EyeOff size={14} /> : <Eye size={14} />} {reveal ? 'Hide secrets' : 'Reveal'}
          </button>
        )}
        <span className="spacer" />
        <button type="button" className="btn ghost sm" disabled={!canExport} onClick={() => void copy(text)}>
          {copied ? <Check size={14} /> : <Copy size={14} />} {copied ? 'Copied' : 'Copy'}
        </button>
        <button
          type="button"
          className="btn ghost sm"
          disabled={!canExport}
          onClick={() => bundle && downloadText(bundle.compose, 'compose.yaml')}
        >
          <Download size={14} /> compose.yaml
        </button>
        <button
          type="button"
          className="btn ghost sm"
          disabled={!canExport}
          onClick={() => bundle && downloadText(bundle.env, '.env')}
        >
          <Download size={14} /> .env
        </button>
        <button
          type="button"
          className="btn primary sm"
          disabled={!canExport}
          onClick={() => bundle && downloadZip({ 'compose.yaml': bundle.compose, '.env': bundle.env }, 'compose.zip')}
        >
          <FileArchive size={14} /> Download .zip
        </button>
      </div>
    </section>
  )
}
