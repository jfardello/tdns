export interface DnsLogItem {
  domain: string
  counter: number
  host: string
}

export interface DnsLogClientCandidate {
  address: string
  host: string
}

export interface DashboardSummary {
  total_queries: number
  blocked_queries: number
  allowed_queries: number
  cache_hits: number
  cache_misses: number
}

export interface DashboardHourlyPoint {
  hour_bucket: number
  hour_start: string
  total_queries: number
  blocked_queries: number
  allowed_queries: number
}

export interface BlacklistState {
  enabled: boolean
  file: string
  external_file: string
  external_repo: string
  external_repo_branch: string
  external_pull_period: string
  excludes: string[]
  persisted_excludes: string[]
  persisted_hosts: string[]
  runtime_whitelist: string[]
  blockfile_total_entries: number
}

export interface CacheState {
  enabled: boolean
  ttl: number
  excludes: string[]
  hits: number
  misses: number
}

export interface HostEntry {
  domain: string
  address: string
}

export interface TagMember {
  address: string
  host: string
  has_host_alias: boolean
}

export interface KnownHostCandidate {
  address: string
  host: string
}

export interface ZenModeState {
  enabled: boolean
  file: string
  duration_minutes: number
  configured_domains: string[]
  persisted_domains: string[]
  configured_excludes: string[]
  persisted_excludes: string[]
  runtime_domains: string[]
  started_at: string
  ends_at: string
  remaining_seconds: number
}

export interface StaticResponseState {
  enabled: boolean
  file: string
  configured_hosts: HostEntry[]
  persisted_hosts: HostEntry[]
  runtime_hosts: HostEntry[]
}

export interface StubResolverState {
  enabled: boolean
  configured_stubs: string[]
  runtime_stubs: string[]
}

interface ApiResponse {
  kind?: string
  message: string
  current_status?: string
  window_hours?: number
  items?: string[]
  log_items?: DnsLogItem[]
  summary?: DashboardSummary
  hourly?: DashboardHourlyPoint[]
  blacklist?: BlacklistState
  cache?: CacheState
  zen_mode?: ZenModeState
  static_response?: StaticResponseState
  stub_resolver?: StubResolverState
  tag_members?: TagMember[]
  known_hosts?: KnownHostCandidate[]
  clients?: DnsLogClientCandidate[]
}

interface DnsLogApiItem {
  Domain?: unknown
  Counter?: unknown
  Host?: unknown
  domain?: unknown
  counter?: unknown
  host?: unknown
}

interface DnsLogClientApiItem {
  address?: unknown
  host?: unknown
}

interface DashboardSummaryApi {
  total_queries?: unknown
  blocked_queries?: unknown
  allowed_queries?: unknown
  cache_hits?: unknown
  cache_misses?: unknown
}

interface DashboardHourlyApiItem {
  hour_bucket?: unknown
  hour_start?: unknown
  total_queries?: unknown
  blocked_queries?: unknown
  allowed_queries?: unknown
}

interface BlacklistApiState {
  enabled?: unknown
  file?: unknown
  external_file?: unknown
  external_repo?: unknown
  external_repo_branch?: unknown
  external_pull_period?: unknown
  excludes?: unknown
  persisted_excludes?: unknown
  persisted_hosts?: unknown
  runtime_whitelist?: unknown
  blockfile_total_entries?: unknown
}

interface CacheApiState {
  enabled?: unknown
  ttl?: unknown
  excludes?: unknown
  hits?: unknown
  misses?: unknown
}

interface HostEntryApi {
  domain?: unknown
  address?: unknown
}

interface TagMemberApi {
  address?: unknown
  host?: unknown
  has_host_alias?: unknown
}

interface KnownHostApi {
  address?: unknown
  host?: unknown
}

interface ZenModeApiState {
  enabled?: unknown
  file?: unknown
  duration_minutes?: unknown
  configured_domains?: unknown
  persisted_domains?: unknown
  configured_excludes?: unknown
  persisted_excludes?: unknown
  runtime_domains?: unknown
  started_at?: unknown
  ends_at?: unknown
  remaining_seconds?: unknown
}

interface StaticResponseApiState {
  enabled?: unknown
  file?: unknown
  configured_hosts?: unknown
  persisted_hosts?: unknown
  runtime_hosts?: unknown
}

interface StubResolverApiState {
  enabled?: unknown
  configured_stubs?: unknown
  runtime_stubs?: unknown
}

function normalizeDnsLogItem(item: DnsLogApiItem): DnsLogItem {
  return {
    domain: String(item.domain ?? item.Domain ?? ''),
    counter: Number(item.counter ?? item.Counter ?? 0),
    host: String(item.host ?? item.Host ?? '')
  }
}

function normalizeDnsLogClient(item: DnsLogClientApiItem): DnsLogClientCandidate {
  return {
    address: String(item.address ?? ''),
    host: String(item.host ?? '')
  }
}

function normalizeDnsLogClients(value: unknown): DnsLogClientCandidate[] {
  if (!Array.isArray(value)) {
    return []
  }

  return value
    .filter(item => item && typeof item === 'object')
    .map(item => normalizeDnsLogClient(item as DnsLogClientApiItem))
}

function normalizeDashboardSummary(summary: DashboardSummaryApi | undefined): DashboardSummary {
  return {
    total_queries: Number(summary?.total_queries ?? 0),
    blocked_queries: Number(summary?.blocked_queries ?? 0),
    allowed_queries: Number(summary?.allowed_queries ?? 0),
    cache_hits: Number(summary?.cache_hits ?? 0),
    cache_misses: Number(summary?.cache_misses ?? 0)
  }
}

function normalizeDashboardHourlyPoint(point: DashboardHourlyApiItem): DashboardHourlyPoint {
  return {
    hour_bucket: Number(point.hour_bucket ?? 0),
    hour_start: String(point.hour_start ?? ''),
    total_queries: Number(point.total_queries ?? 0),
    blocked_queries: Number(point.blocked_queries ?? 0),
    allowed_queries: Number(point.allowed_queries ?? 0)
  }
}

function normalizeStringArray(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return []
  }

  return value.map(item => String(item ?? '')).filter(Boolean)
}

function normalizeBlacklistState(state: BlacklistApiState | undefined): BlacklistState {
  return {
    enabled: Boolean(state?.enabled ?? false),
    file: String(state?.file ?? ''),
    external_file: String(state?.external_file ?? ''),
    external_repo: String(state?.external_repo ?? ''),
    external_repo_branch: String(state?.external_repo_branch ?? ''),
    external_pull_period: String(state?.external_pull_period ?? ''),
    excludes: normalizeStringArray(state?.excludes),
    persisted_excludes: normalizeStringArray(state?.persisted_excludes),
    persisted_hosts: normalizeStringArray(state?.persisted_hosts),
    runtime_whitelist: normalizeStringArray(state?.runtime_whitelist),
    blockfile_total_entries: Number(state?.blockfile_total_entries ?? 0)
  }
}

function normalizeCacheState(state: CacheApiState | undefined): CacheState {
  return {
    enabled: Boolean(state?.enabled ?? false),
    ttl: Number(state?.ttl ?? 0),
    excludes: normalizeStringArray(state?.excludes),
    hits: Number(state?.hits ?? 0),
    misses: Number(state?.misses ?? 0)
  }
}

function normalizeHostEntry(entry: HostEntryApi): HostEntry {
  return {
    domain: String(entry.domain ?? ''),
    address: String(entry.address ?? '')
  }
}

function normalizeTagMember(entry: TagMemberApi): TagMember {
  return {
    address: String(entry.address ?? ''),
    host: String(entry.host ?? ''),
    has_host_alias: Boolean(entry.has_host_alias ?? false)
  }
}

function normalizeTagMembers(value: unknown): TagMember[] {
  if (!Array.isArray(value)) {
    return []
  }

  return value
    .filter(item => item && typeof item === 'object')
    .map(item => normalizeTagMember(item as TagMemberApi))
}

function normalizeKnownHost(entry: KnownHostApi): KnownHostCandidate {
  return {
    address: String(entry.address ?? ''),
    host: String(entry.host ?? '')
  }
}

function normalizeKnownHosts(value: unknown): KnownHostCandidate[] {
  if (!Array.isArray(value)) {
    return []
  }

  return value
    .filter(item => item && typeof item === 'object')
    .map(item => normalizeKnownHost(item as KnownHostApi))
}

function normalizeHostEntries(value: unknown): HostEntry[] {
  if (!Array.isArray(value)) {
    return []
  }

  return value
    .filter(item => item && typeof item === 'object')
    .map(item => normalizeHostEntry(item as HostEntryApi))
}

function normalizeZenModeState(state: ZenModeApiState | undefined): ZenModeState {
  return {
    enabled: Boolean(state?.enabled ?? false),
    file: String(state?.file ?? ''),
    duration_minutes: Number(state?.duration_minutes ?? 0),
    configured_domains: normalizeStringArray(state?.configured_domains),
    persisted_domains: normalizeStringArray(state?.persisted_domains),
    configured_excludes: normalizeStringArray(state?.configured_excludes),
    persisted_excludes: normalizeStringArray(state?.persisted_excludes),
    runtime_domains: normalizeStringArray(state?.runtime_domains),
    started_at: String(state?.started_at ?? ''),
    ends_at: String(state?.ends_at ?? ''),
    remaining_seconds: Number(state?.remaining_seconds ?? 0)
  }
}

function normalizeStaticResponseState(state: StaticResponseApiState | undefined): StaticResponseState {
  return {
    enabled: Boolean(state?.enabled ?? false),
    file: String(state?.file ?? ''),
    configured_hosts: normalizeHostEntries(state?.configured_hosts),
    persisted_hosts: normalizeHostEntries(state?.persisted_hosts),
    runtime_hosts: normalizeHostEntries(state?.runtime_hosts)
  }
}

function normalizeStubResolverState(state: StubResolverApiState | undefined): StubResolverState {
  return {
    enabled: Boolean(state?.enabled ?? false),
    configured_stubs: normalizeStringArray(state?.configured_stubs),
    runtime_stubs: normalizeStringArray(state?.runtime_stubs)
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
    const response = await apiCall<ApiResponse & { stub_resolver?: StubResolverApiState }>(`/api/stub-resolver/${action}`, { method: 'POST' })
    if (!response) {
      return null
    }

    return {
      ...response,
      stub_resolver: normalizeStubResolverState(response.stub_resolver)
    }
  }

  async function replaceStubResolvers(stubs: string[]) {
    const response = await apiCall<ApiResponse & { stub_resolver?: StubResolverApiState }>('/api/stub-resolver', { method: 'POST', body: { stubs } })
    if (!response) {
      return null
    }

    return {
      ...response,
      stub_resolver: normalizeStubResolverState(response.stub_resolver)
    }
  }

  async function getStubResolver() {
    const response = await apiCall<ApiResponse & { stub_resolver?: StubResolverApiState }>('/api/stub-resolver')
    if (!response) {
      return null
    }

    return {
      ...response,
      stub_resolver: normalizeStubResolverState(response.stub_resolver)
    }
  }

  // Blacklist
  async function getBlacklist(): Promise<ApiResponse | null> {
    const response = await apiCall<ApiResponse & { blacklist?: BlacklistApiState }>('/api/blacklist')
    if (!response) {
      return null
    }

    return {
      ...response,
      blacklist: normalizeBlacklistState(response.blacklist)
    }
  }

  async function toggleBlacklist(action: 'start' | 'stop'): Promise<ApiResponse | null> {
    const response = await apiCall<ApiResponse & { blacklist?: BlacklistApiState }>(`/api/blacklist/${action}`, { method: 'POST' })
    if (!response) {
      return null
    }

    return {
      ...response,
      blacklist: normalizeBlacklistState(response.blacklist)
    }
  }

  async function addBlacklistRuntimeWhitelist(domains: string[]): Promise<ApiResponse | null> {
    const response = await apiCall<ApiResponse & { blacklist?: BlacklistApiState }>('/api/blacklist/whitelist', {
      method: 'POST',
      body: { domains }
    })
    if (!response) {
      return null
    }

    return {
      ...response,
      blacklist: normalizeBlacklistState(response.blacklist)
    }
  }

  async function replaceBlacklistPersistedHosts(hosts: string[]): Promise<ApiResponse | null> {
    const response = await apiCall<ApiResponse & { blacklist?: BlacklistApiState }>('/api/blacklist/persisted/hosts', {
      method: 'POST',
      body: { hosts }
    })
    if (!response) {
      return null
    }

    return {
      ...response,
      blacklist: normalizeBlacklistState(response.blacklist)
    }
  }

  async function replaceBlacklistPersistedExcludes(excludes: string[]): Promise<ApiResponse | null> {
    const response = await apiCall<ApiResponse & { blacklist?: BlacklistApiState }>('/api/blacklist/persisted/excludes', {
      method: 'POST',
      body: { excludes }
    })
    if (!response) {
      return null
    }

    return {
      ...response,
      blacklist: normalizeBlacklistState(response.blacklist)
    }
  }

  // Static Response
  async function toggleStaticResponse(action: 'start' | 'stop') {
    const response = await apiCall<ApiResponse & { static_response?: StaticResponseApiState }>(`/api/static-response/${action}`, { method: 'POST' })
    if (!response) {
      return null
    }

    return {
      ...response,
      static_response: normalizeStaticResponseState(response.static_response)
    }
  }

  async function getStaticResponse() {
    const response = await apiCall<ApiResponse & { static_response?: StaticResponseApiState }>('/api/static-response')
    if (!response) {
      return null
    }

    return {
      ...response,
      static_response: normalizeStaticResponseState(response.static_response)
    }
  }

  async function replaceStaticResponseHosts(hosts: string[]) {
    const response = await apiCall<ApiResponse & { static_response?: StaticResponseApiState }>('/api/static-response', {
      method: 'POST',
      body: { hosts }
    })
    if (!response) {
      return null
    }

    return {
      ...response,
      static_response: normalizeStaticResponseState(response.static_response)
    }
  }

  async function replaceStaticResponsePersistedHosts(hosts: string[]) {
    const response = await apiCall<ApiResponse & { static_response?: StaticResponseApiState }>('/api/static-response/persisted', {
      method: 'POST',
      body: { hosts }
    })
    if (!response) {
      return null
    }

    return {
      ...response,
      static_response: normalizeStaticResponseState(response.static_response)
    }
  }

  // DNS Log
  async function getDnsLogTop(top: number, options: {
    since?: string
    status?: 'blocked' | 'allowed' | ''
    client?: string
    client_mode?: 'host' | 'ip' | ''
  } = {}): Promise<ApiResponse | null> {
    const query = new URLSearchParams()
    if (options.since) {
      query.set('since', options.since)
    }
    if (options.status) {
      query.set('status', options.status)
    }
    if (options.client) {
      query.set('client', options.client)
    }
    if (options.client_mode) {
      query.set('client_mode', options.client_mode)
    }
    const suffix = query.toString() ? `?${query.toString()}` : ''
    const response = await apiCall<ApiResponse & { log_items?: DnsLogApiItem[] }>(`/api/dns-log/top/${top}${suffix}`)
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

  async function searchDnsLogClients(search = '', limit = 20): Promise<ApiResponse | null> {
    const query = new URLSearchParams()
    if (search.trim()) {
      query.set('search', search.trim())
    }
    query.set('limit', String(limit))
    const response = await apiCall<ApiResponse>(`/api/dns-log/clients?${query.toString()}`)
    if (!response) {
      return null
    }

    return {
      ...response,
      clients: normalizeDnsLogClients(response.clients)
    }
  }

  async function getDnsDashboard(hours = 24): Promise<ApiResponse | null> {
    const response = await apiCall<ApiResponse & {
      summary?: DashboardSummaryApi
      hourly?: DashboardHourlyApiItem[]
    }>(`/api/dns-log/dashboard?hours=${hours}`)
    if (!response) {
      return null
    }

    return {
      ...response,
      window_hours: Number(response.window_hours ?? hours),
      summary: normalizeDashboardSummary(response.summary),
      hourly: Array.isArray(response.hourly)
        ? response.hourly.map(normalizeDashboardHourlyPoint)
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
  async function getZenMode() {
    const response = await apiCall<ApiResponse & { zen_mode?: ZenModeApiState }>('/api/zen-mode')
    if (!response) {
      return null
    }

    return {
      ...response,
      zen_mode: normalizeZenModeState(response.zen_mode)
    }
  }

  async function startZenMode() {
    const response = await apiCall<ApiResponse & { zen_mode?: ZenModeApiState }>('/api/zen-mode/start', { method: 'POST' })
    if (!response) {
      return null
    }

    return {
      ...response,
      zen_mode: normalizeZenModeState(response.zen_mode)
    }
  }

  async function replaceZenDomains(zen_domains: string[]) {
    const response = await apiCall<ApiResponse & { zen_mode?: ZenModeApiState }>('/api/zen-mode', { method: 'POST', body: { zen_domains } })
    if (!response) {
      return null
    }

    return {
      ...response,
      zen_mode: normalizeZenModeState(response.zen_mode)
    }
  }

  async function replaceZenPersistedDomains(zen_domains: string[]) {
    const response = await apiCall<ApiResponse & { zen_mode?: ZenModeApiState }>('/api/zen-mode/persisted/domains', {
      method: 'POST',
      body: { zen_domains }
    })
    if (!response) {
      return null
    }

    return {
      ...response,
      zen_mode: normalizeZenModeState(response.zen_mode)
    }
  }

  async function replaceZenPersistedExcludes(excludes: string[]) {
    const response = await apiCall<ApiResponse & { zen_mode?: ZenModeApiState }>('/api/zen-mode/persisted/excludes', {
      method: 'POST',
      body: { excludes }
    })
    if (!response) {
      return null
    }

    return {
      ...response,
      zen_mode: normalizeZenModeState(response.zen_mode)
    }
  }

  // Cache
  async function clearCache() {
    const response = await apiCall<ApiResponse & { cache?: CacheApiState }>('/api/cache', { method: 'DELETE' })
    if (!response) {
      return null
    }

    return {
      ...response,
      cache: normalizeCacheState(response.cache)
    }
  }

  async function getCache() {
    const response = await apiCall<ApiResponse & { cache?: CacheApiState }>('/api/cache')
    if (!response) {
      return null
    }

    return {
      ...response,
      cache: normalizeCacheState(response.cache)
    }
  }

  async function toggleCache(action: 'start' | 'stop') {
    const response = await apiCall<ApiResponse & { cache?: CacheApiState }>(`/api/cache/${action}`, { method: 'POST' })
    if (!response) {
      return null
    }

    return {
      ...response,
      cache: normalizeCacheState(response.cache)
    }
  }

  async function replaceCacheExcludes(excludes: string[]) {
    const response = await apiCall<ApiResponse & { cache?: CacheApiState }>('/api/cache/excludes', {
      method: 'POST',
      body: { excludes }
    })
    if (!response) {
      return null
    }

    return {
      ...response,
      cache: normalizeCacheState(response.cache)
    }
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
    const response = await apiCall<ApiResponse>(`/api/tagger/tags/${tagName}`)
    if (!response) {
      return null
    }

    return {
      ...response,
      tag_members: normalizeTagMembers(response.tag_members)
    }
  }

  async function addTagMembers(tagName: string, members: string[]) {
    const response = await apiCall<ApiResponse>(`/api/tagger/tags/${tagName}`, { method: 'POST', body: { members } })
    if (!response) {
      return null
    }

    return {
      ...response,
      tag_members: normalizeTagMembers(response.tag_members)
    }
  }

  async function removeTagMember(tagName: string, address: string) {
    return apiCall(`/api/tagger/tags/${tagName}/${address}`, { method: 'DELETE' })
  }

  async function getKnownHosts(search = '', limit = 20) {
    const query = new URLSearchParams()
    if (search.trim()) {
      query.set('search', search.trim())
    }
    query.set('limit', String(limit))

    const response = await apiCall<ApiResponse>(`/api/tagger/hosts?${query.toString()}`)
    if (!response) {
      return null
    }

    return {
      ...response,
      known_hosts: normalizeKnownHosts(response.known_hosts)
    }
  }

  async function setAddressLabels(address: string, tags: string[]) {
    return apiCall('/api/tagger/address', { method: 'POST', body: { address, tags } })
  }

  async function replaceAddressLabels(address: string, tags: string[]) {
    return apiCall(`/api/tagger/address/${address}`, { method: 'PUT', body: { tags } })
  }

  return {
    apiCall,
    getStubResolver,
    getBlacklist,
    getStaticResponse,
    getZenMode,
    toggleStubResolver,
    replaceStubResolvers,
    toggleBlacklist,
    addBlacklistRuntimeWhitelist,
    replaceBlacklistPersistedHosts,
    replaceBlacklistPersistedExcludes,
    toggleStaticResponse,
    replaceStaticResponseHosts,
    replaceStaticResponsePersistedHosts,
    getDnsDashboard,
    getDnsLogTop,
    searchDnsLogClients,
    rotateDnsLog,
    setDnsLogAlias,
    startZenMode,
    replaceZenDomains,
    replaceZenPersistedDomains,
    replaceZenPersistedExcludes,
    getCache,
    toggleCache,
    replaceCacheExcludes,
    clearCache,
    getTags,
    createTag,
    deleteTag,
    getTagMembers,
    addTagMembers,
    removeTagMember,
    getKnownHosts,
    setAddressLabels,
    replaceAddressLabels
  }
}
