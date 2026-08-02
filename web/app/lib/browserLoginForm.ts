export interface BrowserLoginFormState {
  username: string
  password: string
  code: string
  remember: boolean
}

export function createBrowserLoginFormState(): BrowserLoginFormState {
  return { username: '', password: '', code: '', remember: false }
}

export function clearLoginSecrets(state: BrowserLoginFormState): void {
  state.password = ''
  state.code = ''
}

export function takePasswordSubmission(state: BrowserLoginFormState) {
  const submission = {
    username: state.username,
    password: state.password,
    remember: state.remember
  }
  state.username = ''
  state.password = ''
  state.remember = false
  return submission
}

export function takeCodeSubmission(state: BrowserLoginFormState) {
  const submission = { code: state.code, remember: state.remember }
  state.code = ''
  state.remember = false
  return submission
}
