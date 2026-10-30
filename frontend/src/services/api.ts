import { API_ENDPOINTS } from '../types/api-types'
import type {
  FocusResponse,
  HealthResponse,
  NamespaceListResponse,
  NamespaceMapResponse,
  OverviewResponse
} from '../types/api-types'

async function getJSON<T>(endpoint: string): Promise<T> {
  const response = await fetch(endpoint)
  if (!response.ok) {
    let detail = ''
    try {
      const body = await response.json()
      if (body.error) detail = `: ${body.error}`
    } catch {
      // not JSON, so the status alone describes the failure
    }
    throw new Error(`HTTP ${response.status}${detail}`)
  }
  return response.json()
}

export const healthCheck = () => getJSON<HealthResponse>(API_ENDPOINTS.HEALTH)
export const getOverview = () => getJSON<OverviewResponse>(API_ENDPOINTS.OVERVIEW)
export const getNamespaces = () => getJSON<NamespaceListResponse>(API_ENDPOINTS.NAMESPACES)

export const getNamespaceMap = (namespace: string) =>
  getJSON<NamespaceMapResponse>(`/api/namespaces/${encodeURIComponent(namespace)}/map`)

export const getWorkloadFocus = (namespace: string, kind: string, name: string) =>
  getJSON<FocusResponse>(
    `/api/namespaces/${encodeURIComponent(namespace)}/workloads/${encodeURIComponent(kind)}/${encodeURIComponent(name)}`
  )

export const handleApiError = (error: unknown) => {
  if (error instanceof Error) {
    return error.message
  }
  return 'Request failed'
}
