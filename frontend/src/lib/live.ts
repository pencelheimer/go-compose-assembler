import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'

export type LiveStatus = 'connecting' | 'live' | 'offline'
export type Changed = 'components' | 'presets'

const EVENTS: Record<string, Changed> = {
  components_changed: 'components',
  presets_changed: 'presets',
}

/**
 * Subscribes to GET /api/events. A change event invalidates the matching
 * queries (and the current generation), so open pages refetch on their own.
 * EventSource reconnects by itself; after an outage everything is refetched,
 * since events may have been missed meanwhile.
 */
export function useLiveUpdates(onChange?: (what: Changed) => void): LiveStatus {
  const queryClient = useQueryClient()
  const [status, setStatus] = useState<LiveStatus>('connecting')
  const onChangeRef = useRef(onChange)
  onChangeRef.current = onChange

  useEffect(() => {
    if (typeof EventSource === 'undefined') {
      setStatus('offline')
      return
    }
    const es = new EventSource('/api/events')
    let wasDown = false

    es.onopen = () => {
      setStatus('live')
      if (wasDown) void queryClient.invalidateQueries()
      wasDown = false
    }
    es.onerror = () => {
      wasDown = true
      // CLOSED means the server refused the stream (e.g. no /api/events): give up.
      setStatus(es.readyState === EventSource.CLOSED ? 'offline' : 'connecting')
    }
    for (const [name, what] of Object.entries(EVENTS)) {
      es.addEventListener(name, () => {
        void queryClient.invalidateQueries({ queryKey: [what] })
        void queryClient.invalidateQueries({ queryKey: ['generate'] })
        onChangeRef.current?.(what)
      })
    }
    return () => es.close()
  }, [queryClient])

  return status
}
