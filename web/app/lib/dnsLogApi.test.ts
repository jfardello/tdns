import assert from 'node:assert/strict'
import test from 'node:test'

import { createManagementApiClient } from './apiClient.ts'

const token = `h1c_${'A'.repeat(43)}`

function dnsLogClient(requests: Request[]) {
  return createManagementApiClient('https://tdns.example', {
    getCSRFToken: () => 'csrf-token',
    onUnauthorized: () => {},
    onError: message => assert.fail(message)
  }, async (request) => {
    requests.push(request.clone())
    return Response.json({
      message: 'Status OK',
      dns_log: {
        enabled: request.method === 'POST',
        domains_pseudonymized: true,
        clients_pseudonymized: true,
        key_configured: true,
        queued_events: 0,
        requires_clear: false
      }
    })
  })
}

test('uses the generated lifecycle operations and CSRF protection', async () => {
  const requests: Request[] = []
  const api = dnsLogClient(requests)

  await api.execute(api.client.GET('/api/dns-log'))
  await api.execute(api.client.POST('/api/dns-log/{action}', {
    params: { path: { action: 'stop' } }
  }))
  await api.execute(api.client.DELETE('/api/dns-log'))
  await api.execute(api.client.PUT('/api/dns-log/privacy', {
    body: { domains_pseudonymized: false, clients_pseudonymized: true }
  }))

  assert.deepEqual(requests.map(request => request.method), ['GET', 'POST', 'DELETE', 'PUT'])
  assert.equal(requests[0]?.headers.has('X-CSRF-Token'), false)
  assert.equal(requests[1]?.headers.get('X-CSRF-Token'), 'csrf-token')
  assert.equal(requests[2]?.headers.get('X-CSRF-Token'), 'csrf-token')
  assert.equal(requests[3]?.headers.get('X-CSRF-Token'), 'csrf-token')
  assert.deepEqual(await requests[3]!.json(), {
    domains_pseudonymized: false,
    clients_pseudonymized: true
  })
})

test('preserves client pseudonyms in search, filters, and alias requests', async () => {
  const requests: Request[] = []
  const api = dnsLogClient(requests)

  await api.execute(api.client.GET('/api/dns-log/clients', {
    params: { query: { search: token, limit: 25 } }
  }))
  await api.execute(api.client.GET('/api/dns-log/top/{top}', {
    params: {
      path: { top: 50 },
      query: { client: token, client_mode: 'ip' }
    }
  }))
  await api.execute(api.client.POST('/api/dns-log/alias', {
    body: { name: 'office', addr: token }
  }))

  assert.equal(new URL(requests[0]!.url).searchParams.get('search'), token)
  assert.equal(new URL(requests[1]!.url).searchParams.get('client'), token)
  assert.equal(new URL(requests[1]!.url).searchParams.get('client_mode'), 'ip')
  assert.deepEqual(await requests[2]!.json(), { name: 'office', addr: token })
})
