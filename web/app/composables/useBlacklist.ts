import type { BlacklistState } from '~/composables/useApi'

const EMPTY_BLACKLIST: BlacklistState = {
  enabled: false,
  file: '',
  external_file: '',
  external_repo: '',
  external_repo_branch: '',
  external_pull_period: '',
  excludes: [],
  runtime_whitelist: [],
  blockfile_total_entries: 0
}

export function useBlacklist() {
  const { addBlacklistRuntimeWhitelist, getBlacklist, toggleBlacklist } = useApi()

  const blacklist = useState<BlacklistState>('blacklist-state', () => ({ ...EMPTY_BLACKLIST }))
  const initialized = useState<boolean>('blacklist-initialized', () => false)
  const refreshing = useState<boolean>('blacklist-refreshing', () => false)
  const toggleLoading = useState<boolean>('blacklist-toggle-loading', () => false)
  const runtimeWhitelistLoading = useState<boolean>('blacklist-runtime-whitelist-loading', () => false)

  async function refresh(force = false) {
    if (refreshing.value) {
      return blacklist.value
    }
    if (initialized.value && !force) {
      return blacklist.value
    }

    refreshing.value = true
    const response = await getBlacklist()
    if (response?.blacklist) {
      blacklist.value = response.blacklist
      initialized.value = true
    }
    refreshing.value = false

    return response
  }

  async function setEnabled(nextEnabled: boolean) {
    toggleLoading.value = true
    const response = await toggleBlacklist(nextEnabled ? 'start' : 'stop')
    if (response?.blacklist) {
      blacklist.value = response.blacklist
      initialized.value = true
    }
    toggleLoading.value = false

    return response
  }

  async function addRuntimeWhitelistEntries(domains: string[]) {
    runtimeWhitelistLoading.value = true
    const response = await addBlacklistRuntimeWhitelist(domains)
    if (response?.blacklist) {
      blacklist.value = response.blacklist
      initialized.value = true
    }
    runtimeWhitelistLoading.value = false

    return response
  }

  return {
    blacklist,
    initialized,
    refreshing,
    toggleLoading,
    runtimeWhitelistLoading,
    refresh,
    setEnabled,
    addRuntimeWhitelistEntries
  }
}
