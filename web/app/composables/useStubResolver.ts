import type { StubResolverState } from '~/composables/useApi'

const EMPTY_STUB_RESOLVER: StubResolverState = {
  enabled: false,
  configured_stubs: [],
  runtime_stubs: []
}

export function useStubResolver() {
  const { getStubResolver, replaceStubResolvers, toggleStubResolver } = useApi()

  const stubResolver = useState<StubResolverState>('stub-resolver-state', () => ({ ...EMPTY_STUB_RESOLVER }))
  const initialized = useState<boolean>('stub-resolver-initialized', () => false)
  const refreshing = useState<boolean>('stub-resolver-refreshing', () => false)
  const toggleLoading = useState<boolean>('stub-resolver-toggle-loading', () => false)
  const runtimeStubsLoading = useState<boolean>('stub-resolver-runtime-stubs-loading', () => false)

  async function refresh(force = false) {
    if (refreshing.value) {
      return stubResolver.value
    }
    if (initialized.value && !force) {
      return stubResolver.value
    }

    refreshing.value = true
    const response = await getStubResolver()
    if (response?.stub_resolver) {
      stubResolver.value = response.stub_resolver
      initialized.value = true
    }
    refreshing.value = false

    return response
  }

  async function setEnabled(nextEnabled: boolean) {
    toggleLoading.value = true
    const response = await toggleStubResolver(nextEnabled ? 'start' : 'stop')
    if (response?.stub_resolver) {
      stubResolver.value = response.stub_resolver
      initialized.value = true
    }
    toggleLoading.value = false

    return response
  }

  async function replaceRuntimeStubs(stubs: string[]) {
    runtimeStubsLoading.value = true
    const response = await replaceStubResolvers(stubs)
    if (response?.stub_resolver) {
      stubResolver.value = response.stub_resolver
      initialized.value = true
    }
    runtimeStubsLoading.value = false

    return response
  }

  return {
    stubResolver,
    initialized,
    refreshing,
    toggleLoading,
    runtimeStubsLoading,
    refresh,
    setEnabled,
    replaceRuntimeStubs
  }
}
