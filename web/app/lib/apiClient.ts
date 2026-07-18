import createClient, { type Client } from 'openapi-fetch'
import type { paths } from '../generated/api'

interface ApiCallbacks {
  getToken: () => string | null
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

function errorMessage(error: unknown, response?: Response): string {
  if (typeof error === 'string' && error.trim()) {
    return error
  }
  if (error && typeof error === 'object' && 'message' in error) {
    const message = (error as { message?: unknown }).message
    if (typeof message === 'string' && message.trim()) {
      return message
    }
  }
  return response?.statusText || 'An error occurred'
}

export function createManagementApiClient(
  baseUrl: string,
  callbacks: ApiCallbacks,
  fetchFn?: (input: Request) => Promise<Response>
): ManagementApiClient {
  const client = createClient<paths>({
    baseUrl: baseUrl.replace(/\/$/, ''),
    fetch: fetchFn
  })

  client.use({
    onRequest({ request }) {
      const token = callbacks.getToken()
      if (token) {
        request.headers.set('Authorization', `Bearer ${token}`)
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
