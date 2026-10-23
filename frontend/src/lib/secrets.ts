import { useSyncExternalStore } from 'react'
import type { Values } from './stack'

// Secret values live only in memory: they survive in-app navigation but never
// reach the URL, localStorage or history.

let snapshot: Values = {}
const listeners = new Set<() => void>()

function emit(next: Values) {
  snapshot = next
  for (const l of listeners) l()
}

export const secretStore = {
  get: () => snapshot,
  set(component: string, param: string, value: string | undefined) {
    const params = { ...snapshot[component] }
    if (value === undefined) delete params[param]
    else params[param] = value
    emit({ ...snapshot, [component]: params })
  },
  /** Adds values without overwriting what the user already typed. */
  seed(values: Values) {
    const next = { ...snapshot }
    for (const [comp, params] of Object.entries(values)) next[comp] = { ...params, ...next[comp] }
    emit(next)
  },
  subscribe(listener: () => void) {
    listeners.add(listener)
    return () => listeners.delete(listener)
  },
}

export const useSecrets = () => useSyncExternalStore(secretStore.subscribe, secretStore.get)

/** A random secret from an alphabet that is safe in .env files and shells. */
export function generateSecret(length = 24): string {
  const alphabet = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789'
  const bytes = crypto.getRandomValues(new Uint8Array(length))
  return Array.from(bytes, (b) => alphabet[b % alphabet.length]).join('')
}
