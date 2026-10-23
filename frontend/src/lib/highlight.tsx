import type { ReactNode } from 'react'

// A deliberately tiny highlighter for the two formats we render. It works
// line by line, which is all compose.yaml and .env need.

const VAR = /(\$\{[^}]*\})/g

function value(text: string, key: string): ReactNode[] {
  if (!text) return []
  if (/^(true|false|null|~)$/.test(text.trim()) || /^-?\d+(\.\d+)?$/.test(text.trim())) {
    return [
      <span key={key} className="tok-num">
        {text}
      </span>,
    ]
  }
  if (text.trim() === '{}' || text.trim() === '[]') {
    return [
      <span key={key} className="tok-punct">
        {text}
      </span>,
    ]
  }
  return text.split(VAR).map((part, i) =>
    part.startsWith('${') ? (
      <span key={`${key}-${i}`} className="tok-var">
        {part}
      </span>
    ) : part ? (
      <span key={`${key}-${i}`} className="tok-str">
        {part}
      </span>
    ) : null,
  )
}

function yamlLine(line: string): ReactNode[] {
  if (/^\s*#/.test(line)) return [<span key="c" className="tok-comment">{line}</span>]
  const m = /^(\s*)(- )?(?:([^\s:'"][^:]*?)(:)(?=\s|$))?(\s*)(.*)$/.exec(line)
  if (!m) return [line]
  const [, indent = '', dash, k, colon, gap = '', rest = ''] = m
  const out: ReactNode[] = [indent]
  if (dash) out.push(<span key="d" className="tok-punct">- </span>)
  if (k) {
    out.push(<span key="k" className="tok-key">{k}</span>, <span key=":" className="tok-punct">{colon}</span>)
  }
  out.push(gap, ...value(rest, 'v'))
  return out
}

function envLine(line: string, mask?: (key: string) => boolean): ReactNode[] {
  if (/^\s*#/.test(line)) return [<span key="c" className="tok-comment">{line}</span>]
  const i = line.indexOf('=')
  if (i < 0) return [line]
  const k = line.slice(0, i)
  const v = line.slice(i + 1)
  return [
    <span key="k" className="tok-key">{k}</span>,
    <span key="=" className="tok-punct">=</span>,
    mask?.(k) && v ? (
      <span key="v" className="tok-masked" title="Hidden — use Reveal to show">
        {'•'.repeat(Math.min(Math.max(v.length, 8), 16))}
      </span>
    ) : (
      <span key="v" className="tok-str">{v}</span>
    ),
  ]
}

export function highlight(text: string, lang: 'yaml' | 'env', mask?: (key: string) => boolean): ReactNode[][] {
  const lines = text.replace(/\n$/, '').split('\n')
  return lines.map((l) => (lang === 'yaml' ? yamlLine(l) : envLine(l, mask)))
}

/** Renders plain JSON-like data as YAML for read-only display. */
export function toYaml(data: unknown, indent = 0): string {
  const pad = '  '.repeat(indent)
  const scalar = (v: unknown) => {
    if (typeof v === 'string') return /^[\w./${}-][^:#]*$/.test(v) && !/^(true|false|null|\d+)$/.test(v) ? v : JSON.stringify(v)
    return String(v)
  }
  if (Array.isArray(data)) {
    if (!data.length) return `${pad}[]\n`
    return data
      .map((item) =>
        item && typeof item === 'object'
          ? `${pad}-\n${toYaml(item, indent + 1)}`
          : `${pad}- ${scalar(item)}\n`,
      )
      .join('')
  }
  if (data && typeof data === 'object') {
    const entries = Object.entries(data).filter(([, v]) => v !== undefined && v !== null)
    if (!entries.length) return `${pad}{}\n`
    return entries
      .map(([k, v]) => {
        if (v && typeof v === 'object') {
          const empty = Array.isArray(v) ? v.length === 0 : Object.keys(v).length === 0
          return empty ? `${pad}${k}: ${Array.isArray(v) ? '[]' : '{}'}\n` : `${pad}${k}:\n${toYaml(v, indent + 1)}`
        }
        return `${pad}${k}: ${scalar(v)}\n`
      })
      .join('')
  }
  return `${pad}${scalar(data)}\n`
}
