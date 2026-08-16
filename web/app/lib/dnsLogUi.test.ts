import assert from 'node:assert/strict'
import test from 'node:test'

import {
  canClearDNSLog,
  canConfirmDNSLogClear,
  canStartDNSLog,
  isAliasableDNSLogClient,
  isDNSLogClientToken,
  normalizeDNSLogStatus
} from './dnsLogUi.ts'

const token = `h1c_${'A'.repeat(43)}`

test('normalizes an incomplete DNS-log status response', () => {
  assert.deepEqual(normalizeDNSLogStatus({ enabled: true, queued_events: 3 }), {
    enabled: true,
    domains_pseudonymized: false,
    clients_pseudonymized: false,
    key_configured: false,
    queued_events: 3,
    reason: '',
    requires_clear: false
  })
})

test('recognizes client pseudonyms and aliasable addresses', () => {
  assert.equal(isDNSLogClientToken(token), true)
  assert.equal(isDNSLogClientToken('h1c_too-short'), false)
  assert.equal(isAliasableDNSLogClient(token), true)
  assert.equal(isAliasableDNSLogClient('192.0.2.10'), true)
  assert.equal(isAliasableDNSLogClient('2001:db8::10'), true)
  assert.equal(isAliasableDNSLogClient('office'), false)
})

test('enforces lifecycle and destructive confirmation rules', () => {
  const running = normalizeDNSLogStatus({ enabled: true })
  const incompatible = normalizeDNSLogStatus({ requires_clear: true })
  const stopped = normalizeDNSLogStatus({ enabled: false })

  assert.equal(canStartDNSLog(incompatible), false)
  assert.equal(canStartDNSLog(stopped), true)
  assert.equal(canClearDNSLog(running), false)
  assert.equal(canClearDNSLog(stopped), true)
  assert.equal(canConfirmDNSLogClear(stopped, 'delete'), false)
  assert.equal(canConfirmDNSLogClear(stopped, ' DELETE '), true)
})

