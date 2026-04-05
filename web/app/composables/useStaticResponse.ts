import type { StaticResponseState } from '~/composables/useApi'

const EMPTY_STATIC_RESPONSE: StaticResponseState = {
  enabled: false,
  file: '',
  configured_hosts: [],
  runtime_hosts: []
}

export function useStaticResponse() {
  const { getStaticResponse, replaceStaticResponseHosts, toggleStaticResponse } = useApi()

  const staticResponse = useState<StaticResponseState>('static-response-state', () => ({ ...EMPTY_STATIC_RESPONSE }))
  const initialized = useState<boolean>('static-response-initialized', () => false)
  const refreshing = useState<boolean>('static-response-refreshing', () => false)
  const toggleLoading = useState<boolean>('static-response-toggle-loading', () => false)
  const runtimeHostsLoading = useState<boolean>('static-response-runtime-hosts-loading', () => false)

  async function refresh(force = false) {
    if (refreshing.value) {
      return staticResponse.value
    }
    if (initialized.value && !force) {
      return staticResponse.value
    }

    refreshing.value = true
    const response = await getStaticResponse()
    if (response?.static_response) {
      staticResponse.value = response.static_response
      initialized.value = true
    }
    refreshing.value = false

    return response
  }

  async function setEnabled(nextEnabled: boolean) {
    toggleLoading.value = true
    const response = await toggleStaticResponse(nextEnabled ? 'start' : 'stop')
    if (response?.static_response) {
      staticResponse.value = response.static_response
      initialized.value = true
    }
    toggleLoading.value = false

    return response
  }

  async function replaceRuntimeHosts(hosts: string[]) {
    runtimeHostsLoading.value = true
    const response = await replaceStaticResponseHosts(hosts)
    if (response?.static_response) {
      staticResponse.value = response.static_response
      initialized.value = true
    }
    runtimeHostsLoading.value = false

    return response
  }

  return {
    staticResponse,
    initialized,
    refreshing,
    toggleLoading,
    runtimeHostsLoading,
    refresh,
    setEnabled,
    replaceRuntimeHosts
  }
}
