import assert from 'node:assert/strict'
import test from 'node:test'

import {
  createManagementApiClient,
  isUnsafeMethod,
  normalizeSameOriginBaseUrl
} from './apiClient.ts'

function callbacks(csrfToken: string | null = null) {
  return {
    getCSRFToken: () => csrfToken,
    onUnauthorized: () => assert.fail('unexpected unauthorized callback'),
    onError: (message: string) => assert.fail(message)
  }
}

test('uses same-origin cookies without an Authorization header', async () => {
  let received: Request | undefined
  const api = createManagementApiClient('https://tdns.example/', callbacks(), async (request) => {
    received = request
    return Response.json({ message: 'Status OK' })
  })

  const response = await api.execute(api.client.GET('/api/cache', {
    headers: { Authorization: 'Bearer browser-visible-token' }
  }))

  assert.equal(response?.message, 'Status OK')
  assert.equal(received?.url, 'https://tdns.example/api/cache')
  assert.equal(received?.credentials, 'same-origin')
  assert.equal(received?.headers.get('Authorization'), null)
  assert.equal(received?.headers.get('X-CSRF-Token'), null)
})

test('classifies only unsafe HTTP methods for CSRF', () => {
  for (const method of ['POST', 'PUT', 'PATCH', 'DELETE']) {
    assert.equal(isUnsafeMethod(method), true, method)
  }
  for (const method of ['GET', 'HEAD', 'OPTIONS']) {
    assert.equal(isUnsafeMethod(method), false, method)
  }
})

test('adds CSRF to unsafe requests and omits it from safe requests', async () => {
  let unsafeRequest: Request | undefined
  const unsafeApi = createManagementApiClient('https://tdns.example', callbacks('csrf-token'), async (request) => {
    unsafeRequest = request
    return new Response(null, { status: 204 })
  })
  await unsafeApi.execute(unsafeApi.client.POST('/api/auth/logout'))
  assert.equal(unsafeRequest?.headers.get('X-CSRF-Token'), 'csrf-token')

  let safeRequest: Request | undefined
  const safeApi = createManagementApiClient('https://tdns.example', callbacks('csrf-token'), async (request) => {
    safeRequest = request
    return Response.json({ message: 'Status OK' })
  })
  await safeApi.execute(safeApi.client.GET('/api/cache'))
  assert.equal(safeRequest?.headers.get('X-CSRF-Token'), null)
})

test('handles an expired session without a duplicate API error', async () => {
  let unauthorized = 0
  const errors: string[] = []
  const api = createManagementApiClient('https://tdns.example', {
    getCSRFToken: () => 'csrf-token',
    onUnauthorized: () => { unauthorized++ },
    onError: message => errors.push(message)
  }, async () => Response.json({ error: 'unauthorized' }, { status: 401 }))

  const response = await api.execute(api.client.GET('/api/cache'))

  assert.equal(response, null)
  assert.equal(unauthorized, 1)
  assert.deepEqual(errors, [])
})

test('reports API and network errors', async (t) => {
  await t.test('API response', async () => {
    const errors: string[] = []
    const api = createManagementApiClient('https://tdns.example', {
      getCSRFToken: () => null,
      onUnauthorized: () => assert.fail('unexpected unauthorized callback'),
      onError: message => errors.push(message)
    }, async () => Response.json({ error: 'invalid_request' }, { status: 400 }))

    assert.equal(await api.execute(api.client.GET('/api/cache')), null)
    assert.deepEqual(errors, ['invalid_request'])
  })

  await t.test('network failure', async () => {
    const errors: string[] = []
    const api = createManagementApiClient('https://tdns.example', {
      getCSRFToken: () => null,
      onUnauthorized: () => assert.fail('unexpected unauthorized callback'),
      onError: message => errors.push(message)
    }, async () => { throw new Error('network unavailable') })

    assert.equal(await api.execute(api.client.GET('/api/cache')), null)
    assert.deepEqual(errors, ['network unavailable'])
  })
})

test('rejects a cross-origin browser API base URL', () => {
  assert.throws(
    () => normalizeSameOriginBaseUrl('https://api.example', 'https://tdns.example'),
    /same origin/
  )
  assert.equal(
    normalizeSameOriginBaseUrl('https://tdns.example/management/', 'https://tdns.example'),
    '/management'
  )
})
