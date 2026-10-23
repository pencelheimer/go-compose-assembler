import { X } from 'lucide-react'
import { useState, type ClipboardEvent, type KeyboardEvent } from 'react'

interface ListInputProps {
  id: string
  items: string[]
  placeholder?: string
  invalid?: boolean
  describedBy?: string
  onChange: (items: string[]) => void
}

/** A chip input for list params: Enter or comma adds, Backspace on empty removes. */
export function ListInput({ id, items, placeholder, invalid, describedBy, onChange }: ListInputProps) {
  const [draft, setDraft] = useState('')

  const add = (raw: string) => {
    const fresh = raw
      .split(',')
      .map((s) => s.trim())
      .filter((s) => s && !items.includes(s))
    if (fresh.length) onChange([...items, ...fresh])
    setDraft('')
  }

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault()
      if (draft.trim()) add(draft)
    } else if (e.key === 'Backspace' && !draft && items.length) {
      onChange(items.slice(0, -1))
    }
  }

  const onPaste = (e: ClipboardEvent<HTMLInputElement>) => {
    const text = e.clipboardData.getData('text')
    if (text.includes(',') || text.includes('\n')) {
      e.preventDefault()
      add(text.replace(/\n/g, ','))
    }
  }

  return (
    <div className="chips" data-invalid={invalid || undefined}>
      {items.map((item) => (
        <span key={item} className="chip">
          <span className="mono">{item}</span>
          <button
            type="button"
            className="chip-remove"
            aria-label={`Remove ${item}`}
            onClick={() => onChange(items.filter((i) => i !== item))}
          >
            <X size={12} />
          </button>
        </span>
      ))}
      <input
        id={id}
        className="chips-input mono"
        value={draft}
        placeholder={items.length ? '' : placeholder}
        aria-describedby={describedBy}
        aria-invalid={invalid || undefined}
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={onKeyDown}
        onPaste={onPaste}
        onBlur={() => draft.trim() && add(draft)}
      />
    </div>
  )
}
