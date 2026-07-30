import assert from 'node:assert/strict'
import test from 'node:test'

import {
  clearLegacyBrowserCredential,
  LEGACY_TOKEN_KEY,
  normalizeBrowserSession
} from './browserSession.ts'

test('removes the legacy browser bearer credential', () => {
  const removed: string[] = []
  clearLegacyBrowserCredential({ removeItem: key => removed.push(key) })
  assert.deepEqual(removed, [LEGACY_TOKEN_KEY])
})

test('normalizes complete session responses without retaining the CSRF token in metadata', () => {
  const normalized = normalizeBrowserSession({
    subject: 'admin@tdns',
    scope: 'read-write',
    expires_at: '2026-07-30T22:00:00Z',
    csrf_token: 'csrf-token'
  })

  assert.deepEqual(normalized, {
    session: {
      subject: 'admin@tdns',
      scope: 'read-write',
      expiresAt: '2026-07-30T22:00:00Z'
    },
    csrfToken: 'csrf-token'
  })
})

test('rejects incomplete session responses', () => {
  assert.equal(normalizeBrowserSession({ subject: 'admin@tdns' }), null)
  assert.equal(normalizeBrowserSession(null), null)
})
