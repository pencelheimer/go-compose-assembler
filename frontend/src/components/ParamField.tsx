import { Dices, Eye, EyeOff, RotateCcw } from 'lucide-react'
import { useState } from 'react'
import type { ComponentParam } from '@/api/types'
import { generateSecret } from '@/lib/secrets'
import { isSecret, type EditValue } from '@/lib/stack'
import { ListInput } from './ListInput'

export type Origin = 'edited' | 'preset' | 'default' | 'none'

export const fieldId = (component: string, param: string) => `f-${component}-${param}`

interface ParamFieldProps {
  componentId: string
  param: ComponentParam
  value: EditValue
  origin: Origin
  error?: string
  flagged?: boolean
  onChange: (v: EditValue) => void
  onReset: () => void
}

const originLabel: Record<Origin, string | null> = {
  edited: 'edited',
  preset: 'preset',
  default: 'default',
  none: null,
}

export function ParamField({ componentId, param, value, origin, error, flagged, onChange, onReset }: ParamFieldProps) {
  const [reveal, setReveal] = useState(false)
  const id = fieldId(componentId, param.name)
  const hintId = `${id}-hint`
  const secret = isSecret(param.name)
  const invalid = Boolean(error)
  const label = originLabel[origin]

  return (
    <div className="field" data-flagged={flagged || undefined} data-invalid={invalid || undefined}>
      <div className="field-head">
        <label htmlFor={id} className="field-label mono">
          {param.name}
          {param.required && (
            <span className="req" aria-label="required">
              *
            </span>
          )}
        </label>
        {param.type === 'list' && <span className="tag">list → {param.target}</span>}
        {label && (
          <span className="tag" data-origin={origin}>
            {label}
          </span>
        )}
        {origin === 'edited' && (
          <button type="button" className="icon-btn sm" onClick={onReset} title="Reset to inherited value">
            <RotateCcw size={13} />
            <span className="sr-only">Reset {param.name}</span>
          </button>
        )}
      </div>

      {Array.isArray(value) ? (
        <ListInput
          id={id}
          items={value}
          placeholder={param.target === 'ports' ? 'host:container, Enter to add' : 'Type and press Enter'}
          invalid={invalid}
          describedBy={hintId}
          onChange={onChange}
        />
      ) : (
        <div className="input-wrap">
          <input
            id={id}
            className="input mono"
            type={secret && !reveal ? 'password' : 'text'}
            value={value}
            placeholder={param.default ?? (param.required ? 'required' : '')}
            autoComplete={secret ? 'new-password' : 'off'}
            spellCheck={false}
            aria-invalid={invalid || undefined}
            aria-describedby={hintId}
            onChange={(e) => onChange(e.target.value)}
          />
          {secret && (
            <div className="input-actions">
              <button
                type="button"
                className="icon-btn sm"
                onClick={() => setReveal((r) => !r)}
                title={reveal ? 'Hide' : 'Show'}
              >
                {reveal ? <EyeOff size={14} /> : <Eye size={14} />}
                <span className="sr-only">{reveal ? 'Hide' : 'Show'} value</span>
              </button>
              <button
                type="button"
                className="icon-btn sm"
                onClick={() => onChange(generateSecret())}
                title="Generate a random value"
              >
                <Dices size={14} />
                <span className="sr-only">Generate a random value</span>
              </button>
            </div>
          )}
        </div>
      )}

      <p id={hintId} className={invalid ? 'field-error' : 'field-hint'}>
        {error ?? param.description}
      </p>
    </div>
  )
}
