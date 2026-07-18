import type { ZenModeState } from '~/composables/useApi'

const EMPTY_ZEN_MODE: ZenModeState = {
  enabled: false,
  file: '',
  duration_minutes: 0,
  configured_domains: [],
  persisted_domains: [],
  configured_excludes: [],
  persisted_excludes: [],
  labels: [],
  runtime_domains: [],
  started_at: '',
  ends_at: '',
  remaining_seconds: 0
}

export function useZenMode() {
  const { getZenMode, replaceZenDomains, replaceZenPersistedDomains, replaceZenPersistedExcludes, startZenMode } = useApi()

  const zenMode = useState<ZenModeState>('zen-mode-state', () => ({ ...EMPTY_ZEN_MODE }))
  const initialized = useState<boolean>('zen-mode-initialized', () => false)
  const refreshing = useState<boolean>('zen-mode-refreshing', () => false)
  const startLoading = useState<boolean>('zen-mode-start-loading', () => false)
  const persistedDomainsLoading = useState<boolean>('zen-mode-persisted-domains-loading', () => false)
  const persistedExcludesLoading = useState<boolean>('zen-mode-persisted-excludes-loading', () => false)
  const runtimeDomainsLoading = useState<boolean>('zen-mode-runtime-domains-loading', () => false)

  async function refresh(force = false): Promise<void> {
    if (refreshing.value) {
      return
    }
    if (initialized.value && !force) {
      return
    }

    refreshing.value = true
    const response = await getZenMode()
    if (response?.zen_mode) {
      zenMode.value = response.zen_mode
      initialized.value = true
    }
    refreshing.value = false
  }

  async function startSession() {
    startLoading.value = true
    const response = await startZenMode()
    if (response?.zen_mode) {
      zenMode.value = response.zen_mode
      initialized.value = true
    }
    startLoading.value = false

    return response
  }

  async function replaceRuntimeDomains(domains: string[]) {
    runtimeDomainsLoading.value = true
    const response = await replaceZenDomains(domains)
    if (response?.zen_mode) {
      zenMode.value = response.zen_mode
      initialized.value = true
    }
    runtimeDomainsLoading.value = false

    return response
  }

  async function replacePersistedDomains(domains: string[]) {
    persistedDomainsLoading.value = true
    const response = await replaceZenPersistedDomains(domains)
    if (response?.zen_mode) {
      zenMode.value = response.zen_mode
      initialized.value = true
    }
    persistedDomainsLoading.value = false

    return response
  }

  async function replacePersistedExcludes(excludes: string[]) {
    persistedExcludesLoading.value = true
    const response = await replaceZenPersistedExcludes(excludes)
    if (response?.zen_mode) {
      zenMode.value = response.zen_mode
      initialized.value = true
    }
    persistedExcludesLoading.value = false

    return response
  }

  return {
    zenMode,
    initialized,
    refreshing,
    startLoading,
    persistedDomainsLoading,
    persistedExcludesLoading,
    runtimeDomainsLoading,
    refresh,
    startSession,
    replacePersistedDomains,
    replacePersistedExcludes,
    replaceRuntimeDomains
  }
}
