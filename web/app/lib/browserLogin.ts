import type { Client } from 'openapi-fetch'
import type { components, paths } from '../generated/api'

type BrowserSessionResponse = components['schemas']['api.BrowserSessionResponse']

export type BrowserLoginOutcome =
  | 'success'
  | 'invalid-code'
  | 'invalid-credentials'
  | 'rate-limited'
  | 'error'

export interface BrowserLoginAttempt {
  outcome: BrowserLoginOutcome
  session?: BrowserSessionResponse
}

export async function exchangeBrowserCode(
  client: Client<paths>,
  code: string,
  remember = false
): Promise<BrowserLoginAttempt> {
  try {
    const { data, response } = await client.POST('/api/auth/exchange', {
      body: { code: code.trim(), remember }
    })
    if (response.ok && data) {
      return { outcome: 'success', session: data }
    }
    if (response.status === 400 || response.status === 401) {
      return { outcome: 'invalid-code' }
    }
    if (response.status === 429) {
      return { outcome: 'rate-limited' }
    }
  } catch {
    return { outcome: 'error' }
  }
  return { outcome: 'error' }
}

export async function loginWithAdministratorPassword(
  client: Client<paths>,
  username: string,
  password: string,
  remember = false
): Promise<BrowserLoginAttempt> {
  try {
    const { data, response } = await client.POST('/api/auth/login', {
      body: { username: username.trim(), password, remember }
    })
    if (response.ok && data) {
      return { outcome: 'success', session: data }
    }
    if (response.status === 400 || response.status === 401 || response.status === 503) {
      return { outcome: 'invalid-credentials' }
    }
    if (response.status === 429) {
      return { outcome: 'rate-limited' }
    }
  } catch {
    return { outcome: 'error' }
  }
  return { outcome: 'error' }
}
