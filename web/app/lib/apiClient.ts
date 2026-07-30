import createClient, { type Client } from 'openapi-fetch'
import type { paths } from '../generated/api'

interface ApiCallbacks {
  getCSRFToken: () => string | null
  onUnauthorized: () => void | Promise<void>
  onError: (message: string) => void
}

type ApiResult<T> = {
  data?: T
  error?: unknown
  response: Response
}

export interface ManagementApiClient {
  client: Client<paths>
  execute: <T>(request: Promise<ApiResult<T>>) => Promise<T | null>
}

const unsafeMethods = new Set(['POST', 'PUT', 'PATCH', 'DELETE'])

export function isUnsafeMethod(method: string): boolean {
  return unsafeMethods.has(method.toUpperCase())
}

function errorMessage(error: unknown, response?: Response): string {
  if (typeof error === 'string' && error.trim()) {
    return error
  }
  if (error && typeof error === 'object') {
    const candidate = error as { error?: unknown, message?: unknown }
    if (typeof candidate.error === 'string' && candidate.error.trim()) {
      return candidate.error
    }
    if (typeof candidate.message === 'string' && candidate.message.trim()) {
      return candidate.message
    }
  }
  return response?.statusText || 'An error occurred'
}

export function normalizeSameOriginBaseUrl(baseUrl: string, browserOrigin?: string): string {
  const normalized = baseUrl.trim().replace(/\/$/, '')
  if (!normalized || !browserOrigin) {
    return normalized
  }

  const resolved = new URL(normalized, browserOrigin)
  if (resolved.origin !== browserOrigin) {
    throw new Error('The browser management API must use the same origin as the web UI')
  }
  return resolved.pathname === '/' ? '' : resolved.pathname.replace(/\/$/, '')
}

export function createManagementApiClient(
  baseUrl: string,
  callbacks: ApiCallbacks,
  fetchFn?: (input: Request) => Promise<Response>,
  browserOrigin: string | undefined = typeof window === 'undefined' ? undefined : window.location.origin
): ManagementApiClient {
  const client = createClient<paths>({
    baseUrl: normalizeSameOriginBaseUrl(baseUrl, browserOrigin),
    credentials: 'same-origin',
    fetch: fetchFn
  })

  client.use({
    onRequest({ request }) {
      request.headers.delete('Authorization')
      const csrfToken = callbacks.getCSRFToken()
      if (csrfToken && isUnsafeMethod(request.method)) {
        request.headers.set('X-CSRF-Token', csrfToken)
      }
      return request
    }
  })

  async function execute<T>(request: Promise<ApiResult<T>>): Promise<T | null> {
    try {
      const { data, error, response } = await request
      if (response.status === 401) {
        await callbacks.onUnauthorized()
        return null
      }
      if (!response.ok || error !== undefined) {
        callbacks.onError(errorMessage(error, response))
        return null
      }
      return data ?? null
    } catch (error) {
      callbacks.onError(errorMessage(error))
      return null
    }
  }

  return { client, execute }
}
