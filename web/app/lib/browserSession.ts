export const LEGACY_TOKEN_KEY = 'tdns_jwt_token'

export interface BrowserSession {
  subject: string
  scope: string
  expiresAt: string
}

export interface NormalizedBrowserSession {
  session: BrowserSession
  csrfToken: string
}

interface StorageLike {
  removeItem: (key: string) => void
}

export function clearLegacyBrowserCredential(
  storage: StorageLike | undefined = typeof localStorage === 'undefined' ? undefined : localStorage
): void {
  storage?.removeItem(LEGACY_TOKEN_KEY)
}

export function normalizeBrowserSession(value: unknown): NormalizedBrowserSession | null {
  if (!value || typeof value !== 'object') {
    return null
  }

  const candidate = value as Record<string, unknown>
  if (
    typeof candidate.subject !== 'string'
    || typeof candidate.scope !== 'string'
    || typeof candidate.expires_at !== 'string'
    || typeof candidate.csrf_token !== 'string'
    || !candidate.subject
    || !candidate.scope
    || !candidate.expires_at
    || !candidate.csrf_token
  ) {
    return null
  }

  return {
    session: {
      subject: candidate.subject,
      scope: candidate.scope,
      expiresAt: candidate.expires_at
    },
    csrfToken: candidate.csrf_token
  }
}
