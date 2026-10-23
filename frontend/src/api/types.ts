// Mirrors the JSON shapes of internal/domain, internal/export and internal/web.

export type Scalar = string | number | boolean
export type ParamValue = Scalar | Scalar[] | null

export interface ComponentParam {
  name: string
  type: 'scalar' | 'list'
  required: boolean
  default?: string
  target?: string
  description?: string
}

export interface ComponentSummary {
  id: string
  description: string
}

export interface HealthCheck {
  test: string[]
  interval?: string
  timeout?: string
  retries?: number
  start_period?: string
}

export interface ServiceTemplate {
  image?: string
  ports?: string[]
  environment?: Record<string, string>
  volumes?: string[]
  networks?: string[]
  depends_on?: Record<string, { condition: string }>
  restart?: string
  healthcheck?: HealthCheck
  extra?: Record<string, unknown>
}

export interface ComponentDef {
  id: string
  description: string
  params: ComponentParam[]
  service: ServiceTemplate
  volumes?: { name: string }[]
  networks?: { name: string }[]
}

export interface PresetSummary {
  id: string
  description: string
}

export interface PresetService {
  component: string
  params?: Record<string, ParamValue>
}

export interface Preset {
  id: string
  description?: string
  services: PresetService[]
}

export interface ServiceSelection {
  component: string
  params?: Record<string, ParamValue>
}

export interface GenerateRequest {
  preset?: string
  overrides?: Record<string, Record<string, ParamValue>>
  services?: ServiceSelection[]
}

export interface Bundle {
  compose: string
  env: string
  warnings?: string[]
}

export interface ListResponse<T> {
  items: T[]
  skipped: string[]
}

export type ErrorKind = 'invalid_input' | 'not_found' | 'invalid_definition' | 'conflict' | 'internal'

export type ErrorType =
  | 'missing_param'
  | 'unknown_param'
  | 'param_type'
  | 'invalid_request'
  | 'component_not_found'
  | 'preset_not_found'
  | 'invalid_component'
  | 'invalid_preset'
  | 'unknown_target'
  | 'port_conflict'
  | 'volume_conflict'
  | 'env_conflict'
  | 'cyclic_dependency'
  | 'duplicate_service'
  | 'error'

export interface ErrorDetail {
  type: ErrorType
  message: string
  component?: string
  param?: string
  services?: string[]
  value?: string
  chain?: string[]
  location?: string
  expected?: string
  actual?: string
}

export interface ApiErrorBody {
  kind: ErrorKind
  message: string
  details: ErrorDetail[]
}
