import { createManagementApiClient } from '~/lib/apiClient'
import {
  clearLegacyBrowserCredential,
  normalizeBrowserSession,
  type BrowserSession
} from '~/lib/browserSession'
import {
  exchangeBrowserCode,
  loginWithAdministratorPassword,
  type BrowserLoginAttempt,
  type BrowserLoginOutcome
} from '~/lib/browserLogin'

export type AuthenticationStatus = 'loading' | 'authenticated' | 'anonymous'
export type LoginResult = BrowserLoginOutcome

let restorePromise: Promise<void> | null = null

export function useAuth() {
  const config = useRuntimeConfig()
  const status = useState<AuthenticationStatus>('auth_status', () => 'loading')
  const session = useState<BrowserSession | null>('auth_session', () => null)
  const csrfToken = useState<string | null>('auth_csrf_token', () => null)

  const isAuthenticated = computed(() => status.value === 'authenticated')
  const isLoading = computed(() => status.value === 'loading')

  function clearSession(nextStatus: AuthenticationStatus = 'anonymous') {
    session.value = null
    csrfToken.value = null
    status.value = nextStatus
  }

  function applySession(value: unknown): boolean {
    const normalized = normalizeBrowserSession(value)
    if (!normalized) {
      clearSession()
      return false
    }

    session.value = normalized.session
    csrfToken.value = normalized.csrfToken
    status.value = 'authenticated'
    return true
  }

  function createSessionClient() {
    return createManagementApiClient(config.public.apiBaseUrl, {
      getCSRFToken: () => csrfToken.value,
      onUnauthorized: () => {},
      onError: () => {}
    })
  }

  async function restoreSession(): Promise<void> {
    clearLegacyBrowserCredential()
    if (status.value !== 'loading') {
      return
    }
    if (restorePromise) {
      return restorePromise
    }

    restorePromise = (async () => {
      try {
        const api = createSessionClient()
        const { data, response } = await api.client.GET('/api/auth/session')
        if (response.ok && applySession(data)) {
          return
        }
      } catch {
        // Authentication restoration fails closed when the server is unavailable.
      }
      clearSession()
    })()

    try {
      await restorePromise
    } finally {
      restorePromise = null
    }
  }

  function applyLoginAttempt(attempt: BrowserLoginAttempt): LoginResult {
    if (attempt.outcome === 'success' && applySession(attempt.session)) {
      return 'success'
    }
    if (attempt.outcome === 'invalid-code' || attempt.outcome === 'invalid-credentials') {
      clearSession()
    }
    return attempt.outcome === 'success' ? 'error' : attempt.outcome
  }

  async function exchangeCode(code: string, remember = false): Promise<LoginResult> {
    clearLegacyBrowserCredential()
    const api = createSessionClient()
    return applyLoginAttempt(await exchangeBrowserCode(api.client, code, remember))
  }

  async function loginWithPassword(
    username: string,
    password: string,
    remember = false
  ): Promise<LoginResult> {
    clearLegacyBrowserCredential()
    const api = createSessionClient()
    return applyLoginAttempt(
      await loginWithAdministratorPassword(api.client, username, password, remember)
    )
  }

  async function logout(): Promise<boolean> {
    if (status.value !== 'authenticated') {
      clearSession()
      return true
    }

    try {
      const api = createSessionClient()
      const { response } = await api.client.POST('/api/auth/logout')
      if (response.status !== 204) {
        return false
      }
      clearSession()
      return true
    } catch {
      return false
    }
  }

  function expireSession(): boolean {
    if (status.value === 'anonymous') {
      return false
    }
    clearSession()
    return true
  }

  return {
    status: readonly(status),
    session: readonly(session),
    csrfToken: readonly(csrfToken),
    isAuthenticated,
    isLoading,
    restoreSession,
    exchangeCode,
    loginWithPassword,
    logout,
    expireSession
  }
}
