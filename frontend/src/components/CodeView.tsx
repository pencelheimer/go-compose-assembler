import { highlight } from '@/lib/highlight'

interface CodeViewProps {
  text: string
  lang: 'yaml' | 'env'
  mask?: (key: string) => boolean
  dimmed?: boolean
  label: string
}

export function CodeView({ text, lang, mask, dimmed, label }: CodeViewProps) {
  return (
    <pre className="code" data-dimmed={dimmed || undefined} aria-label={label} tabIndex={0}>
      <code>
        {highlight(text, lang, mask).map((tokens, i) => (
          <span key={i} className="code-line">
            {tokens}
            {'\n'}
          </span>
        ))}
      </code>
    </pre>
  )
}
