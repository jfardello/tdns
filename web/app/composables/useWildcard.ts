import type { WildcardState } from '~/composables/useApi'

const EMPTY_WILDCARD: WildcardState = {
  enabled: false,
  primary_domain: '',
  available_extra_domains: [],
  enabled_extra_domains: [],
  allow_public_addresses: false,
  ttl: 0
}

export function useWildcard() {
  const { getWildcard, replaceWildcardDomains, toggleWildcard } = useApi()

  const wildcard = useState<WildcardState>('wildcard-state', () => ({ ...EMPTY_WILDCARD }))
  const initialized = useState<boolean>('wildcard-initialized', () => false)
  const refreshing = useState<boolean>('wildcard-refreshing', () => false)
  const toggleLoading = useState<boolean>('wildcard-toggle-loading', () => false)
  const domainsLoading = useState<boolean>('wildcard-domains-loading', () => false)
  const errorMessage = useState<string | null>('wildcard-error', () => null)

  async function refresh(force = false): Promise<void> {
    if (refreshing.value || (initialized.value && !force)) {
      return
    }

    refreshing.value = true
    const response = await getWildcard()
    if (response?.wildcard) {
      wildcard.value = response.wildcard
      initialized.value = true
      errorMessage.value = null
    } else {
      errorMessage.value = 'Unable to load wildcard DNS settings.'
    }
    refreshing.value = false
  }

  async function setEnabled(nextEnabled: boolean) {
    toggleLoading.value = true
    const response = await toggleWildcard(nextEnabled ? 'start' : 'stop')
    if (response?.wildcard) {
      wildcard.value = response.wildcard
      initialized.value = true
      errorMessage.value = null
    } else {
      errorMessage.value = 'Unable to change the wildcard DNS state.'
    }
    toggleLoading.value = false
    return response
  }

  async function setDomains(domains: string[]) {
    domainsLoading.value = true
    const response = await replaceWildcardDomains(domains)
    if (response?.wildcard) {
      wildcard.value = response.wildcard
      initialized.value = true
      errorMessage.value = null
    } else {
      errorMessage.value = 'Unable to update wildcard DNS domains.'
    }
    domainsLoading.value = false
    return response
  }

  return {
    wildcard,
    initialized,
    refreshing,
    toggleLoading,
    domainsLoading,
    errorMessage,
    refresh,
    setEnabled,
    setDomains
  }
}
