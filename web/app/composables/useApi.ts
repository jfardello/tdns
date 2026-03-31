export interface DnsLogItem {
  domain: string
  counter: number
  host: string
}

interface ApiResponse {
  kind?: string
  message: string
  current_status?: string
  items?: string[]
  log_items?: DnsLogItem[]
}

interface DnsLogApiItem {
  Domain?: unknown
  Counter?: unknown
  Host?: unknown
  domain?: unknown
  counter?: unknown
  host?: unknown
}

function normalizeDnsLogItem(item: DnsLogApiItem): DnsLogItem {
  return {
    domain: String(item.domain ?? item.Domain ?? ''),
    counter: Number(item.counter ?? item.Counter ?? 0),
    host: String(item.host ?? item.Host ?? '')
  }
}

export function useApi() {
  const config = useRuntimeConfig()
  const { getAuthHeaders, clearToken } = useAuth()
  const toast = useToast()
  const apiBaseUrl = config.public.apiBaseUrl.replace(/\/$/, '')

  async function apiCall<T = ApiResponse>(
    endpoint: string,
    options: {
      method?: 'GET' | 'POST' | 'PUT' | 'DELETE'
      body?: unknown
    } = {}
  ): Promise<T | null> {
    try {
      const response = await $fetch<T>(`${apiBaseUrl}${endpoint}`, {
        method: options.method || 'GET',
        headers: {
          'Content-Type': 'application/json',
          ...getAuthHeaders()
        },
        body: options.body ? JSON.stringify(options.body) : undefined
      })
      return response
    } catch (error: unknown) {
      const fetchError = error as { status?: number; statusText?: string; message?: string }
      if (fetchError.status === 401) {
        clearToken()
        toast.add({
          title: 'Session expired',
          description: 'Please login again',
          color: 'error',
          icon: 'i-lucide-alert-circle'
        })
        navigateTo('/login')
      } else {
        toast.add({
          title: 'API Error',
          description: fetchError.statusText || fetchError.message || 'An error occurred',
          color: 'error',
          icon: 'i-lucide-alert-circle'
        })
      }
      return null
    }
  }

  // Stub Resolver
  async function toggleStubResolver(action: 'start' | 'stop') {
    return apiCall(`/api/stub-resolver/${action}`, { method: 'POST' })
  }

  async function replaceStubResolvers(stubs: string[]) {
    return apiCall('/api/stub-resolver', { method: 'POST', body: { stubs } })
  }

  // Blacklist
  async function toggleBlacklist(action: 'start' | 'stop') {
    return apiCall(`/api/blacklist/${action}`, { method: 'POST' })
  }

  // Static Response
  async function toggleStaticResponse(action: 'start' | 'stop') {
    return apiCall(`/api/static-response/${action}`, { method: 'POST' })
  }

  // DNS Log
  async function getDnsLogTop(top: number, since?: string): Promise<ApiResponse | null> {
    const query = since ? `?since=${since}` : ''
    const response = await apiCall<ApiResponse & { log_items?: DnsLogApiItem[] }>(`/api/dns-log/top/${top}${query}`)
    if (!response) {
      return null
    }

    return {
      ...response,
      log_items: Array.isArray(response.log_items)
        ? response.log_items.map(normalizeDnsLogItem)
        : []
    }
  }

  async function rotateDnsLog(since: string) {
    return apiCall(`/api/dns-log/rotate?since=${since}`)
  }

  async function setDnsLogAlias(name: string, addr: string) {
    return apiCall('/api/dns-log/alias', { method: 'POST', body: { name, addr } })
  }

  // Zen Mode
  async function toggleZenMode() {
    return apiCall('/api/zen-mode/start', { method: 'POST' })
  }

  async function replaceZenDomains(zen_domains: string[]) {
    return apiCall('/api/zen-mode', { method: 'POST', body: { zen_domains } })
  }

  // Cache
  async function clearCache() {
    return apiCall('/api/cache', { method: 'DELETE' })
  }

  // Tagger
  async function getTags() {
    return apiCall('/api/tagger/tags')
  }

  async function createTag(name: string) {
    return apiCall('/api/tagger/tags', { method: 'POST', body: { name } })
  }

  async function deleteTag(tagName: string) {
    return apiCall(`/api/tagger/tags/${tagName}`, { method: 'DELETE' })
  }

  async function getTagMembers(tagName: string) {
    return apiCall(`/api/tagger/tags/${tagName}`)
  }

  async function addTagMembers(tagName: string, members: string[]) {
    return apiCall(`/api/tagger/tags/${tagName}`, { method: 'POST', body: { members } })
  }

  async function removeTagMember(tagName: string, address: string) {
    return apiCall(`/api/tagger/tags/${tagName}/${address}`, { method: 'DELETE' })
  }

  async function setAddressLabels(address: string, tags: string[]) {
    return apiCall('/api/tagger/address', { method: 'POST', body: { address, tags } })
  }

  async function replaceAddressLabels(address: string, tags: string[]) {
    return apiCall(`/api/tagger/address/${address}`, { method: 'PUT', body: { tags } })
  }

  return {
    apiCall,
    toggleStubResolver,
    replaceStubResolvers,
    toggleBlacklist,
    toggleStaticResponse,
    getDnsLogTop,
    rotateDnsLog,
    setDnsLogAlias,
    toggleZenMode,
    replaceZenDomains,
    clearCache,
    getTags,
    createTag,
    deleteTag,
    getTagMembers,
    addTagMembers,
    removeTagMember,
    setAddressLabels,
    replaceAddressLabels
  }
}
