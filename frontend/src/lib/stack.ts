// Pure builder logic: turning what the user sees and edits into a
// GenerateRequest. It mirrors cli/wizard.go: an emptied scalar falls back to
// the preset value or default, and only changed values are sent.

import type { ComponentDef, ComponentParam, GenerateRequest, ParamValue, Preset } from '@/api/types'

export type EditValue = string | string[]
/** component id → param name → the user's edit */
export type Values = Record<string, Record<string, EditValue>>

export interface BuilderSearch {
  /** preset mode: the selected preset */
  preset?: string
  /** custom mode: component ids in stack order */
  services?: string[]
  /** non-secret edits; secrets never reach the URL */
  values?: Values
  /** the preset a custom stack was derived from, for display only */
  base?: string
}

export interface StackEntry {
  id: string
  def: ComponentDef
  /** preset values for this component (preset mode only) */
  presetParams?: Record<string, ParamValue>
}

const isString = (v: unknown): v is string => typeof v === 'string'

export function parseBuilderSearch(raw: Record<string, unknown>): BuilderSearch {
  const out: BuilderSearch = {}
  if (isString(raw.preset) && raw.preset) out.preset = raw.preset
  if (Array.isArray(raw.services)) out.services = [...new Set(raw.services.filter(isString))]
  if (isString(raw.base) && raw.base) out.base = raw.base
  if (raw.values && typeof raw.values === 'object' && !Array.isArray(raw.values)) {
    const values: Values = {}
    for (const [comp, params] of Object.entries(raw.values)) {
      if (!params || typeof params !== 'object' || Array.isArray(params)) continue
      const clean: Record<string, EditValue> = {}
      for (const [name, v] of Object.entries(params)) {
        if (isString(v)) clean[name] = v
        else if (Array.isArray(v)) clean[name] = v.filter(isString)
      }
      if (Object.keys(clean).length) values[comp] = clean
    }
    if (Object.keys(values).length) out.values = values
  }
  return out
}

/** Secret-looking params are masked in the UI and kept out of the URL (same rule as the TUI). */
export const isSecret = (name: string) => /PASSWORD|SECRET|TOKEN/i.test(name)

export const toText = (v: ParamValue | undefined): string =>
  v == null ? '' : Array.isArray(v) ? v.map(String).join(', ') : String(v)

export const toList = (v: ParamValue | undefined): string[] =>
  v == null ? [] : Array.isArray(v) ? v.map(String) : [String(v)]

/** The value a field starts with: the preset value, else the default. */
export function prefillOf(param: ComponentParam, presetParams?: Record<string, ParamValue>): EditValue {
  const pv = presetParams?.[param.name]
  if (param.type === 'list') return toList(pv)
  return pv != null ? toText(pv) : (param.default ?? '')
}

export function effectiveValue(entry: StackEntry, param: ComponentParam, edits: Values): EditValue {
  return edits[entry.id]?.[param.name] ?? prefillOf(param, entry.presetParams)
}

const sameList = (a: string[], b: string[]) => a.length === b.length && a.every((x, i) => x === b[i])

/** Whether the user's edit differs from what the field would get anyway. */
export function isEdited(entry: StackEntry, param: ComponentParam, edits: Values): boolean {
  const edit = edits[entry.id]?.[param.name]
  if (edit === undefined) return false
  const prefill = prefillOf(param, entry.presetParams)
  if (Array.isArray(edit) || Array.isArray(prefill)) return !sameList(toList(edit), toList(prefill))
  return edit.trim() !== '' && edit !== prefill
}

export function buildRequest(presetId: string | undefined, entries: StackEntry[], edits: Values): GenerateRequest {
  const changed = (entry: StackEntry) => {
    const params: Record<string, ParamValue> = {}
    for (const p of entry.def.params) {
      if (isEdited(entry, p, edits)) params[p.name] = edits[entry.id]![p.name]!
    }
    return params
  }
  if (presetId) {
    const overrides: Record<string, Record<string, ParamValue>> = {}
    for (const e of entries) {
      const params = changed(e)
      if (Object.keys(params).length) overrides[e.id] = params
    }
    return Object.keys(overrides).length ? { preset: presetId, overrides } : { preset: presetId }
  }
  return {
    services: entries.map((e) => {
      const params = changed(e)
      return Object.keys(params).length ? { component: e.id, params } : { component: e.id }
    }),
  }
}

/** Required params that are still empty; the server would reject the request. */
export function missingRequired(entries: StackEntry[], edits: Values) {
  const missing: { component: string; param: string }[] = []
  for (const e of entries) {
    for (const p of e.def.params) {
      if (!p.required) continue
      const v = effectiveValue(e, p, edits)
      if (Array.isArray(v) ? v.length === 0 : v.trim() === '') missing.push({ component: e.id, param: p.name })
    }
  }
  return missing
}

/** Dependencies declared by the template that are not part of the stack. */
export function missingDependencies(entry: StackEntry, stackIds: string[]): string[] {
  return Object.keys(entry.def.service.depends_on ?? {}).filter((d) => !stackIds.includes(d))
}

export function splitSecrets(values: Values): { visible: Values; secret: Values } {
  const visible: Values = {}
  const secret: Values = {}
  for (const [comp, params] of Object.entries(values)) {
    for (const [name, v] of Object.entries(params)) {
      const into = isSecret(name) ? secret : visible
      ;(into[comp] ??= {})[name] = v
    }
  }
  return { visible, secret }
}

export function mergeValues(...layers: (Values | undefined)[]): Values {
  const out: Values = {}
  for (const layer of layers) {
    for (const [comp, params] of Object.entries(layer ?? {})) out[comp] = { ...out[comp], ...params }
  }
  return out
}

/** Preset values as edits, so a preset can become an editable custom stack. */
export function presetAsValues(preset: Preset): Values {
  const out: Values = {}
  for (const s of preset.services) {
    for (const [name, v] of Object.entries(s.params ?? {})) {
      if (v == null) continue
      ;(out[s.component] ??= {})[name] = Array.isArray(v) ? v.map(String) : String(v)
    }
  }
  return out
}

export function withoutComponent(values: Values | undefined, id: string): Values | undefined {
  if (!values?.[id]) return values
  const { [id]: _removed, ...rest } = values
  return Object.keys(rest).length ? rest : undefined
}

/** JSON with sorted keys, for cheap structural comparison. */
export function stableStringify(v: unknown): string {
  return JSON.stringify(v, (_k, val: unknown) =>
    val && typeof val === 'object' && !Array.isArray(val)
      ? Object.fromEntries(Object.entries(val).sort(([a], [b]) => a.localeCompare(b)))
      : val,
  )
}

/** Params whose current value mentions a conflicting host port, e.g. "8080/tcp". */
export function fieldsUsingPort(entry: StackEntry, edits: Values, port: string): string[] {
  const host = port.split('/')[0] ?? port
  return entry.def.params
    .filter((p) => {
      const v = effectiveValue(entry, p, edits)
      return Array.isArray(v) ? v.some((item) => item.split(':')[0] === host) : v === host
    })
    .map((p) => p.name)
}
