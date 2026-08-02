import assert from 'node:assert/strict'
import test from 'node:test'

import { createManagementApiClient } from './apiClient.ts'
import {
  exchangeBrowserCode,
  loginWithAdministratorPassword
} from './browserLogin.ts'

function clientWith(fetchFn: (request: Request) => Promise<Response>) {
  return createManagementApiClient('https://tdns.example', {
    getCSRFToken: () => null,
    onUnauthorized: () => assert.fail('raw login requests do not execute callbacks'),
    onError: message => assert.fail(message)
  }, fetchFn).client
}

function sessionResponse() {
  return Response.json({
    subject: 'admin',
    scope: 'tdns.kubewire.net:rw',
    expires_at: '2026-08-11T12:00:00Z',
    csrf_token: 'csrf-token'
  })
}

test('password login uses the generated same-origin endpoint and remember choice', async () => {
  let received: Request | undefined
  const client = clientWith(async (request) => {
    received = request
    return sessionResponse()
  })

  const attempt = await loginWithAdministratorPassword(
    client,
    ' admin ',
    'secret password',
    true
  )

  assert.equal(attempt.outcome, 'success')
  assert.equal(received?.url, 'https://tdns.example/api/auth/login')
  assert.equal(received?.credentials, 'same-origin')
  assert.equal(received?.headers.get('Authorization'), null)
  assert.deepEqual(await received?.json(), {
    username: 'admin',
    password: 'secret password',
    remember: true
  })
})

test('browser-code login trims the code and keeps remember disabled by default', async () => {
  let received: Request | undefined
  const client = clientWith(async (request) => {
    received = request
    return sessionResponse()
  })

  const attempt = await exchangeBrowserCode(client, ' browser-code ')

  assert.equal(attempt.outcome, 'success')
  assert.equal(received?.url, 'https://tdns.example/api/auth/exchange')
  assert.deepEqual(await received?.json(), { code: 'browser-code', remember: false })
})

test('login failures use bounded, mode-specific outcomes', async (t) => {
  await t.test('password failures remain generic', async () => {
    for (const status of [400, 401, 503]) {
      const client = clientWith(async () => Response.json({ error: 'unauthorized' }, { status }))
      const attempt = await loginWithAdministratorPassword(client, 'admin', 'wrong password')
      assert.equal(attempt.outcome, 'invalid-credentials', String(status))
    }
  })

  await t.test('invalid code', async () => {
    const client = clientWith(async () => Response.json({ error: 'unauthorized' }, { status: 401 }))
    assert.equal((await exchangeBrowserCode(client, 'invalid')).outcome, 'invalid-code')
  })

  await t.test('rate limit', async () => {
    const client = clientWith(async () => Response.json({ error: 'too_many_requests' }, { status: 429 }))
    assert.equal((await loginWithAdministratorPassword(client, 'admin', 'password')).outcome, 'rate-limited')
    assert.equal((await exchangeBrowserCode(client, 'code')).outcome, 'rate-limited')
  })

  await t.test('network error', async () => {
    const client = clientWith(async () => { throw new Error('network unavailable') })
    assert.equal((await loginWithAdministratorPassword(client, 'admin', 'password')).outcome, 'error')
  })
})

test('login requests never read browser storage', async (t) => {
  const names = ['localStorage', 'sessionStorage'] as const
  const originals = new Map<string, PropertyDescriptor | undefined>()
  let reads = 0
  for (const name of names) {
    originals.set(name, Object.getOwnPropertyDescriptor(globalThis, name))
    Object.defineProperty(globalThis, name, {
      configurable: true,
      get() {
        reads++
        throw new Error(`${name} must not be accessed`)
      }
    })
  }
  t.after(() => {
    for (const name of names) {
      const original = originals.get(name)
      if (original) {
        Object.defineProperty(globalThis, name, original)
      } else {
        delete (globalThis as Record<string, unknown>)[name]
      }
    }
  })

  const client = clientWith(async () => sessionResponse())
  assert.equal((await loginWithAdministratorPassword(client, 'admin', 'password')).outcome, 'success')
  assert.equal((await exchangeBrowserCode(client, 'code')).outcome, 'success')
  assert.equal(reads, 0)
})
