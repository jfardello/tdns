import type { CacheState } from '~/composables/useApi'

const EMPTY_CACHE: CacheState = {
  enabled: false,
  ttl: 0,
  excludes: [],
  hits: 0,
  misses: 0
}

export function useCache() {
  const { clearCache, getCache, replaceCacheExcludes, toggleCache } = useApi()

  const cacheState = useState<CacheState>('cache-state', () => ({ ...EMPTY_CACHE }))
  const initialized = useState<boolean>('cache-initialized', () => false)
  const refreshing = useState<boolean>('cache-refreshing', () => false)
  const toggleLoading = useState<boolean>('cache-toggle-loading', () => false)
  const excludesLoading = useState<boolean>('cache-excludes-loading', () => false)
  const clearLoading = useState<boolean>('cache-clear-loading', () => false)

  async function refresh(force = false): Promise<void> {
    if (refreshing.value) {
      return
    }
    if (initialized.value && !force) {
      return
    }

    refreshing.value = true
    const response = await getCache()
    if (response?.cache) {
      cacheState.value = response.cache
      initialized.value = true
    }
    refreshing.value = false
  }

  async function setEnabled(nextEnabled: boolean) {
    toggleLoading.value = true
    const response = await toggleCache(nextEnabled ? 'start' : 'stop')
    if (response?.cache) {
      cacheState.value = response.cache
      initialized.value = true
    }
    toggleLoading.value = false
    return response
  }

  async function setExcludes(excludes: string[]) {
    excludesLoading.value = true
    const response = await replaceCacheExcludes(excludes)
    if (response?.cache) {
      cacheState.value = response.cache
      initialized.value = true
    }
    excludesLoading.value = false
    return response
  }

  async function clear() {
    clearLoading.value = true
    const response = await clearCache()
    if (response?.cache) {
      cacheState.value = response.cache
      initialized.value = true
    }
    clearLoading.value = false
    return response
  }

  return {
    cacheState,
    initialized,
    refreshing,
    toggleLoading,
    excludesLoading,
    clearLoading,
    refresh,
    setEnabled,
    setExcludes,
    clear
  }
}
