import { useQueries, useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { Layers, RotateCcw, Shuffle } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { ApiError } from '@/api/client'
import { componentQuery, componentsQuery, generateQuery, presetQuery, presetsQuery } from '@/api/queries'
import type { Bundle, ErrorDetail } from '@/api/types'
import { PreviewPanel, type PreviewStatus } from '@/components/PreviewPanel'
import { ServiceCard } from '@/components/ServiceCard'
import { Sidebar } from '@/components/Sidebar'
import { useDebouncedValue } from '@/lib/hooks'
import { secretStore, useSecrets } from '@/lib/secrets'
import {
  buildRequest,
  fieldsUsingPort,
  isSecret,
  mergeValues,
  missingDependencies,
  missingRequired,
  parseBuilderSearch,
  presetAsValues,
  splitSecrets,
  stableStringify,
  withoutComponent,
  type BuilderSearch,
  type EditValue,
  type StackEntry,
  type Values,
} from '@/lib/stack'
import { builderSearch, useUrlValues } from '@/lib/useUrlValues'

export const Route = createFileRoute('/')({
  validateSearch: parseBuilderSearch,
  loaderDeps: ({ search }) => ({ preset: search.preset }),
  loader: ({ context: { queryClient }, deps }) =>
    Promise.all([
      queryClient.prefetchQuery(componentsQuery),
      queryClient.prefetchQuery(presetsQuery),
      deps.preset && queryClient.prefetchQuery(presetQuery(deps.preset)),
    ]),
  component: Builder,
})

const GENERATE_DELAY = 350

function apiDetails(error: unknown): ErrorDetail[] {
  if (error instanceof ApiError) {
    return error.body.details.length ? error.body.details : [{ type: 'error', message: error.message }]
  }
  return [{ type: 'error', message: error instanceof Error ? error.message : String(error) }]
}

function Builder() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()

  const components = useQuery(componentsQuery)
  const presets = useQuery(presetsQuery)
  const presetQ = useQuery({ ...presetQuery(search.preset ?? ''), enabled: Boolean(search.preset) })
  const preset = search.preset ? presetQ.data : undefined
  const isPreset = Boolean(search.preset)

  const writeValues = useCallback(
    (values: Values | undefined) => void navigate({ search: (prev) => ({ ...prev, values }), replace: true }),
    [navigate],
  )
  const [values, setValues] = useUrlValues(search, writeValues)
  const secrets = useSecrets()
  const edits = useMemo(() => mergeValues(values, secrets), [values, secrets])

  const stackIds = useMemo(
    () => (isPreset ? (preset?.services.map((s) => s.component) ?? []) : (search.services ?? [])),
    [isPreset, preset, search.services],
  )
  const defs = useQueries({ queries: stackIds.map((id) => componentQuery(id)) })
  const entries: StackEntry[] = []
  defs.forEach((q, i) => {
    const id = stackIds[i]!
    if (q.data) entries.push({ id, def: q.data, presetParams: preset?.services.find((s) => s.component === id)?.params })
  })
  const defErrors = defs.flatMap((q) => (q.error ? apiDetails(q.error) : []))
  const allLoaded = entries.length === stackIds.length && (!isPreset || Boolean(preset))

  useEffect(() => {
    builderSearch.set({ ...search, values: Object.keys(values).length ? values : undefined })
  }, [search, values])

  // --- generation -----------------------------------------------------------
  const missing = allLoaded ? missingRequired(entries, edits) : []
  const ready = allLoaded && stackIds.length > 0 && missing.length === 0
  const requestKey = ready ? stableStringify(buildRequest(search.preset, entries, edits)) : ''
  const debouncedKey = useDebouncedValue(requestKey, GENERATE_DELAY)
  const request = useMemo(() => (debouncedKey ? JSON.parse(debouncedKey) : null), [debouncedKey])
  const gen = useQuery({ ...generateQuery(request ?? {}), enabled: ready && request !== null })

  const [lastGood, setLastGood] = useState<Bundle>()
  useEffect(() => {
    if (gen.data && !gen.isPlaceholderData) setLastGood(gen.data)
  }, [gen.data, gen.isPlaceholderData])
  useEffect(() => {
    if (stackIds.length === 0) setLastGood(undefined)
  }, [stackIds.length])

  const settled = ready && debouncedKey === requestKey && !gen.isFetching
  const genErrors = ready && gen.error && !gen.isFetching ? apiDetails(gen.error) : []
  const presetErrors = presetQ.error ? apiDetails(presetQ.error) : []
  const errors = [...presetErrors, ...defErrors, ...genErrors]

  let status: PreviewStatus
  if (stackIds.length === 0 && !presetQ.isLoading) status = 'empty'
  else if (!allLoaded) status = errors.length ? 'invalid' : 'loading'
  else if (missing.length) status = 'incomplete'
  else if (!settled) status = 'generating'
  else if (genErrors.length) status = 'invalid'
  else status = gen.data?.warnings?.length ? 'warnings' : 'valid'

  const fresh = settled && !genErrors.length && gen.data && !gen.isPlaceholderData ? gen.data : undefined
  const bundle = fresh ?? lastGood
  const stale = !fresh

  // --- map errors onto cards and fields ----------------------------------------
  const fieldErrors: Record<string, string> = {}
  const flagged = new Set<string>()
  const issues: Record<string, string[]> = {}
  {
    const note = (svc: string, msg: string) => (issues[svc] ??= []).push(msg)
    for (const d of genErrors) {
      if (d.component && d.param) {
        fieldErrors[`${d.component}.${d.param}`] =
          d.type === 'param_type'
            ? `Expected ${d.expected}, got ${d.actual}`
            : d.type === 'unknown_param'
              ? 'Not declared by this component'
              : d.type === 'missing_param'
                ? 'Required'
                : d.message
      } else if (d.services?.length === 2) {
        const [a, b] = d.services as [string, string]
        const what = d.type === 'port_conflict' ? `Port ${d.value}` : d.type === 'env_conflict' ? `Variable ${d.value}` : `Volume ${d.value}`
        note(a, `${what} is also used by ${b}`)
        note(b, `${what} is also used by ${a}`)
        if (d.type === 'port_conflict' && d.value) {
          for (const e of entries.filter((e) => e.id === a || e.id === b)) {
            for (const p of fieldsUsingPort(e, edits, d.value)) flagged.add(`${e.id}.${p}`)
          }
        }
      } else if (d.type === 'cyclic_dependency') {
        for (const s of d.chain ?? []) note(s, 'Part of a dependency cycle')
      } else if (d.component) {
        note(d.component, d.message)
      }
    }
  }

  // --- actions --------------------------------------------------------------
  const setValue = (component: string, param: string, v: EditValue) => {
    if (isSecret(param) && typeof v === 'string') return secretStore.set(component, param, v)
    setValues((prev) => ({ ...prev, [component]: { ...prev[component], [param]: v } }))
  }

  const resetValue = (component: string, param: string) => {
    if (isSecret(param)) return secretStore.set(component, param, undefined)
    setValues((prev) => {
      const { [param]: _dropped, ...rest } = prev[component] ?? {}
      const next = { ...prev, [component]: rest }
      if (!Object.keys(rest).length) delete next[component]
      return next
    })
  }

  /** Navigate to a new stack shape, carrying the edits typed so far. */
  const go = (next: BuilderSearch) =>
    void navigate({ search: { ...next, values: next.values && Object.keys(next.values).length ? next.values : undefined } })

  /** A preset becomes a custom stack once its service list changes. */
  const customize = (ids: string[]) => {
    if (!preset) return
    const { visible, secret } = splitSecrets(presetAsValues(preset))
    secretStore.seed(secret)
    go({ services: ids, values: mergeValues(visible, values), base: preset.id })
  }

  const addComponent = (id: string) => {
    if (stackIds.includes(id)) return
    if (isPreset) customize([...stackIds, id])
    else go({ ...search, services: [...stackIds, id], values })
  }

  const removeComponent = (id: string) => {
    const ids = stackIds.filter((s) => s !== id)
    if (isPreset) {
      customize(ids)
      return
    }
    go({ ...search, services: ids, values: withoutComponent(values, id) })
  }

  const resetAll = () => {
    for (const id of stackIds) {
      for (const p of Object.keys(secrets[id] ?? {})) secretStore.set(id, p, undefined)
    }
    setValues({})
  }

  const hasEdits = Object.keys(values).length > 0 || stackIds.some((id) => Object.keys(secrets[id] ?? {}).length > 0)
  const presetSummary = presets.data?.items.find((p) => p.id === search.preset)

  return (
    <div className="builder">
      <Sidebar
        presets={presets.data}
        components={components.data}
        activePreset={search.preset}
        custom={!isPreset}
        inStack={stackIds}
        onAdd={addComponent}
      />

      <main className="stack" aria-label="Stack">
        <div className="stack-head">
          <div>
            <p className="eyebrow">{isPreset ? 'Preset' : 'Custom stack'}</p>
            <h1 className="stack-title">
              {isPreset ? <span className="mono">{search.preset}</span> : 'Your services'}
              <span className="count">{stackIds.length}</span>
            </h1>
            <p className="muted small">
              {isPreset
                ? (presetSummary?.description ?? preset?.description)
                : search.base
                  ? <>Based on preset <Link to="/" search={{ preset: search.base }} className="mono">{search.base}</Link></>
                  : 'Add components from the catalog.'}
            </p>
          </div>
          <div className="stack-actions">
            {isPreset && preset && (
              <button type="button" className="btn ghost sm" onClick={() => customize(stackIds)} title="Edit the list of services">
                <Shuffle size={14} /> Customize services
              </button>
            )}
            <button type="button" className="btn ghost sm" onClick={resetAll} disabled={!hasEdits}>
              <RotateCcw size={14} /> Reset values
            </button>
          </div>
        </div>

        {stackIds.length === 0 && !presetQ.isLoading && !presetErrors.length && (
          <div className="empty">
            <Layers size={28} aria-hidden />
            <h2>Start a stack</h2>
            <p className="muted">Pick a preset for a ready-made set of services, or add components one by one.</p>
            <div className="empty-actions">
              {presets.data?.items.map((p) => (
                <Link key={p.id} to="/" search={{ preset: p.id }} className="btn ghost">
                  <span className="mono">{p.id}</span>
                </Link>
              ))}
            </div>
          </div>
        )}

        <div className="cards">
          {stackIds.map((id, i) => {
            const entry = entries.find((e) => e.id === id)
            if (!entry) {
              return defs[i]?.error ? null : <div key={id} className="card skeleton" aria-busy="true" />
            }
            return (
              <ServiceCard
                key={id}
                entry={entry}
                edits={edits}
                fieldErrors={fieldErrors}
                flaggedFields={flagged}
                issues={issues[id] ?? []}
                missingDeps={missingDependencies(entry, stackIds)}
                incomplete={missing.some((m) => m.component === id)}
                onChange={(p, v) => setValue(id, p, v)}
                onReset={(p) => resetValue(id, p)}
                onRemove={() => removeComponent(id)}
                onAdd={addComponent}
              />
            )
          })}
        </div>
      </main>

      <PreviewPanel status={status} bundle={bundle} stale={stale} errors={errors} missing={missing} />
    </div>
  )
}
