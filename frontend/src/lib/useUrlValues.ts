import { useEffect, useRef, useState } from 'react'
import { stableStringify, type BuilderSearch, type Values } from './stack'

const URL_WRITE_DELAY = 300

/**
 * Keeps the non-secret edits in local state (so inputs stay responsive) and
 * mirrors them into the URL with a debounce. Navigation that changes the URL
 * from outside — a preset link, back/forward — replaces the local state.
 */
export function useUrlValues(search: BuilderSearch, write: (values: Values | undefined) => void) {
  const structure = stableStringify([search.preset ?? null, search.services ?? null])
  const incoming = stableStringify(search.values ?? {})

  const [values, setValues] = useState<Values>(search.values ?? {})
  const seen = useRef({ structure, written: incoming })

  // Adopt values from the URL unless they are the echo of our own write.
  useEffect(() => {
    if (seen.current.structure !== structure || seen.current.written !== incoming) {
      seen.current = { structure, written: incoming }
      setValues(search.values ?? {})
    }
    // search.values is tracked through `incoming`
  }, [structure, incoming])

  useEffect(() => {
    const str = stableStringify(values)
    if (str === seen.current.written) return
    const forStructure = seen.current.structure
    const t = setTimeout(() => {
      if (seen.current.structure !== forStructure) return // the stack changed meanwhile
      seen.current.written = str
      write(Object.keys(values).length ? values : undefined)
    }, URL_WRITE_DELAY)
    return () => clearTimeout(t)
  }, [values, write])

  return [values, setValues] as const
}

let lastBuilderSearch: BuilderSearch = {}

/** The builder's latest state, so links back to it from other pages restore it. */
export const builderSearch = {
  get: () => lastBuilderSearch,
  set: (s: BuilderSearch) => {
    lastBuilderSearch = s
  },
}
