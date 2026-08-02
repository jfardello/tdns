import assert from 'node:assert/strict'
import test from 'node:test'

import {
  clearLoginSecrets,
  createBrowserLoginFormState,
  takeCodeSubmission,
  takePasswordSubmission
} from './browserLoginForm.ts'

test('remember is opt-in and password submission clears form credentials', () => {
  const state = createBrowserLoginFormState()
  assert.equal(state.remember, false)
  Object.assign(state, { username: 'admin', password: 'secret password', remember: true })

  assert.deepEqual(takePasswordSubmission(state), {
    username: 'admin',
    password: 'secret password',
    remember: true
  })
  assert.deepEqual(state, { username: '', password: '', code: '', remember: false })
})

test('browser-code submission clears the code and resets remember', () => {
  const state = createBrowserLoginFormState()
  Object.assign(state, { code: 'one-time-code', remember: true })

  assert.deepEqual(takeCodeSubmission(state), { code: 'one-time-code', remember: true })
  assert.deepEqual(state, { username: '', password: '', code: '', remember: false })
})

test('switching login modes clears both secrets without changing the selection', () => {
  const state = {
    username: 'admin',
    password: 'secret password',
    code: 'one-time-code',
    remember: true
  }
  clearLoginSecrets(state)

  assert.deepEqual(state, { username: 'admin', password: '', code: '', remember: true })
})
