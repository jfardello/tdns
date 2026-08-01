import assert from 'node:assert/strict'
import test from 'node:test'

const globals = globalThis as typeof globalThis & {
  defineNuxtConfig?: <T>(config: T) => T
}
globals.defineNuxtConfig = config => config

const { default: config } = await import('../../nuxt.config.ts')

test('bundles icons locally without a runtime API fallback', () => {
  assert.equal(config.icon?.provider, 'none')
  assert.equal(config.icon?.fallbackToApi, false)
  assert.equal(config.icon?.clientBundle?.scan, true)
})
