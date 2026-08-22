import assert from 'node:assert/strict'
import test from 'node:test'

import { updateWildcardDomainSelection, wildcardExamples } from './wildcardUi.ts'

test('builds supported wildcard examples from the configured primary domain', () => {
  assert.deepEqual(wildcardExamples('internal.example.'), [
    { label: 'IPv4 dots', hostname: 'app.192.168.1.20.internal.example', address: '192.168.1.20' },
    { label: 'IPv4 dashes', hostname: 'app-192-168-1-20.internal.example', address: '192.168.1.20' },
    { label: 'IPv4 hexadecimal', hostname: 'app-c0a80114.internal.example', address: '192.168.1.20' },
    { label: 'IPv6 dashes', hostname: 'fd00--20.internal.example', address: 'fd00::20' }
  ])
  assert.equal(wildcardExamples('')[0]?.hostname, 'app.192.168.1.20.tdns.home.arpa')
})

test('updates extra domains as one allowlisted ordered selection', () => {
  const available = ['nip.io', 'sslip.io', 'xip.io']

  assert.deepEqual(
    updateWildcardDomainSelection(available, ['xip.io'], 'nip.io', true),
    ['nip.io', 'xip.io']
  )
  assert.deepEqual(
    updateWildcardDomainSelection(available, ['nip.io', 'xip.io'], 'nip.io', false),
    ['xip.io']
  )
  assert.deepEqual(
    updateWildcardDomainSelection(available, ['nip.io'], 'example.com', true),
    ['nip.io']
  )
})
