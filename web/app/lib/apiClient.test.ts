import assert from 'node:assert/strict'
import test from 'node:test'

import { createManagementApiClient } from './apiClient.ts'

test('injects the current bearer token', async () => {
  let received: Request | undefined
  const api = createManagementApiClient('https://tdns.example/', {
    getToken: () => 'test-token',
    onUnauthorized: () => assert.fail('unexpected unauthorized callback'),
    onError: message => assert.fail(message)
  }, async (request) => {
    received = request
    return Response.json({ message: 'Status OK' })
  })

  const response = await api.execute(api.client.GET('/api/cache'))

  assert.equal(response?.message, 'Status OK')
  assert.equal(received?.url, 'https://tdns.example/api/cache')
  assert.equal(received?.headers.get('Authorization'), 'Bearer test-token')
})

test('handles an expired session without a duplicate API error', async () => {
  let unauthorized = 0
  const errors: string[] = []
  const api = createManagementApiClient('https://tdns.example', {
    getToken: () => 'expired-token',
    onUnauthorized: () => { unauthorized++ },
    onError: message => errors.push(message)
  }, async () => Response.json('unauthorized', { status: 401 }))

  const response = await api.execute(api.client.GET('/api/cache'))

  assert.equal(response, null)
  assert.equal(unauthorized, 1)
  assert.deepEqual(errors, [])
})

test('reports API and network errors', async (t) => {
  await t.test('API response', async () => {
    const errors: string[] = []
    const api = createManagementApiClient('https://tdns.example', {
      getToken: () => null,
      onUnauthorized: () => assert.fail('unexpected unauthorized callback'),
      onError: message => errors.push(message)
    }, async () => Response.json({ message: 'invalid request' }, { status: 400 }))

    assert.equal(await api.execute(api.client.GET('/api/cache')), null)
    assert.deepEqual(errors, ['invalid request'])
  })

  await t.test('network failure', async () => {
    const errors: string[] = []
    const api = createManagementApiClient('https://tdns.example', {
      getToken: () => null,
      onUnauthorized: () => assert.fail('unexpected unauthorized callback'),
      onError: message => errors.push(message)
    }, async () => { throw new Error('network unavailable') })

    assert.equal(await api.execute(api.client.GET('/api/cache')), null)
    assert.deepEqual(errors, ['network unavailable'])
  })
})
