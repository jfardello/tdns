import assert from 'node:assert/strict'
import test from 'node:test'

import { createManagementApiClient } from './apiClient.ts'

test('uses generated wildcard operations with CSRF protection', async () => {
  const requests: Request[] = []
  const api = createManagementApiClient('https://tdns.example', {
    getCSRFToken: () => 'csrf-token',
    onUnauthorized: () => assert.fail('unexpected unauthorized response'),
    onError: message => assert.fail(message)
  }, async (request) => {
    requests.push(request.clone())
    return Response.json({
      message: 'Status OK',
      wildcard: {
        enabled: true,
        primary_domain: 'tdns.home.arpa',
        available_extra_domains: ['nip.io', 'sslip.io'],
        enabled_extra_domains: ['nip.io'],
        allow_public_addresses: false,
        ttl: 60
      }
    })
  })

  await api.execute(api.client.GET('/api/wildcard'))
  await api.execute(api.client.POST('/api/wildcard/{action}', {
    params: { path: { action: 'start' } }
  }))
  await api.execute(api.client.PUT('/api/wildcard/domains', {
    body: { domains: ['nip.io', 'sslip.io'] }
  }))

  assert.deepEqual(requests.map(request => request.method), ['GET', 'POST', 'PUT'])
  assert.equal(requests[0]?.headers.has('X-CSRF-Token'), false)
  assert.equal(requests[1]?.headers.get('X-CSRF-Token'), 'csrf-token')
  assert.equal(requests[2]?.headers.get('X-CSRF-Token'), 'csrf-token')
  assert.deepEqual(await requests[2]!.json(), { domains: ['nip.io', 'sslip.io'] })
})
