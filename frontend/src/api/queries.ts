import { keepPreviousData, queryOptions } from '@tanstack/react-query'
import { api } from './client'
import type {
  Bundle,
  ComponentDef,
  ComponentSummary,
  GenerateRequest,
  ListResponse,
  Preset,
  PresetSummary,
} from './types'

// Nothing is cached on the server: definitions are re-read from disk on every
// request, so refetching on window focus picks up edited YAML files.

export const componentsQuery = queryOptions({
  queryKey: ['components'],
  queryFn: ({ signal }) => api.get<ListResponse<ComponentSummary>>('/api/components', signal),
})

export const componentQuery = (id: string) =>
  queryOptions({
    queryKey: ['components', id],
    queryFn: ({ signal }) => api.get<ComponentDef>(`/api/components/${encodeURIComponent(id)}`, signal),
  })

export const presetsQuery = queryOptions({
  queryKey: ['presets'],
  queryFn: ({ signal }) => api.get<ListResponse<PresetSummary>>('/api/presets', signal),
})

export const presetQuery = (id: string) =>
  queryOptions({
    queryKey: ['presets', id],
    queryFn: ({ signal }) => api.get<Preset>(`/api/presets/${encodeURIComponent(id)}`, signal),
  })

/** Generation is keyed by the request itself; a newer request aborts the older one. */
export const generateQuery = (req: GenerateRequest) =>
  queryOptions({
    queryKey: ['generate', req],
    queryFn: ({ signal }) => api.post<Bundle>('/api/generate', req, signal),
    placeholderData: keepPreviousData,
    retry: false,
  })
