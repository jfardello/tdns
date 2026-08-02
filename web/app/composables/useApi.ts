import type { components } from '~/generated/api'

type Schema<Name extends keyof components['schemas']> = components['schemas'][Name]

export type DnsLogItem = Required<Schema<'api.LogDetails'>>
export type DnsLogClientCandidate = Required<Schema<'api.ClientCandidate'>>
export type DashboardSummary = Required<Schema<'api.DashboardSummary'>>
export type DashboardHourlyPoint = Required<Schema<'api.DashboardHourlyPoint'>>
export type BlacklistState = Required<Schema<'api.BlacklistStatus'>>
export type CacheState = Required<Schema<'api.CacheStatus'>>
export type HostEntry = Required<Schema<'api.HostEntry'>>
export type TagMember = Required<Schema<'api.TagMember'>>
export type KnownHostCandidate = Required<Schema<'api.KnownHost'>>
export type ZenModeState = Required<Schema<'api.ZenModeStatus'>>
export type StaticResponseState = Required<Schema<'api.StaticResponseStatus'>>
export type StubResolverState = Required<Schema<'api.StubResolverStatus'>>

function normalizeStringArray(value: string[] | undefined): string[] {
  return value ?? []
}

function normalizeDnsLogItem(item: Schema<'api.LogDetails'>): DnsLogItem {
  return {
    domain: item.domain ?? '',
    counter: item.counter ?? 0,
    host: item.host ?? ''
  }
}

function normalizeDnsLogClient(item: Schema<'api.ClientCandidate'>): DnsLogClientCandidate {
  return {
    address: item.address ?? '',
    host: item.host ?? ''
  }
}

function normalizeDashboardSummary(summary: Schema<'api.DashboardSummary'> | undefined): DashboardSummary {
  return {
    total_queries: summary?.total_queries ?? 0,
    blocked_queries: summary?.blocked_queries ?? 0,
    allowed_queries: summary?.allowed_queries ?? 0,
    cache_hits: summary?.cache_hits ?? 0,
    cache_misses: summary?.cache_misses ?? 0
  }
}

function normalizeDashboardHourlyPoint(point: Schema<'api.DashboardHourlyPoint'>): DashboardHourlyPoint {
  return {
    hour_bucket: point.hour_bucket ?? 0,
    hour_start: point.hour_start ?? '',
    total_queries: point.total_queries ?? 0,
    blocked_queries: point.blocked_queries ?? 0,
    allowed_queries: point.allowed_queries ?? 0
  }
}

function normalizeBlacklistState(state: Schema<'api.BlacklistStatus'> | undefined): BlacklistState {
  return {
    enabled: state?.enabled ?? false,
    file: state?.file ?? '',
    external_file: state?.external_file ?? '',
    external_repo: state?.external_repo ?? '',
    external_repo_branch: state?.external_repo_branch ?? '',
    external_pull_period: state?.external_pull_period ?? '',
    excludes: normalizeStringArray(state?.excludes),
    persisted_excludes: normalizeStringArray(state?.persisted_excludes),
    persisted_hosts: normalizeStringArray(state?.persisted_hosts),
    runtime_whitelist: normalizeStringArray(state?.runtime_whitelist),
    blockfile_total_entries: state?.blockfile_total_entries ?? 0
  }
}

function normalizeCacheState(state: Schema<'api.CacheStatus'> | undefined): CacheState {
  return {
    enabled: state?.enabled ?? false,
    ttl: state?.ttl ?? 0,
    excludes: normalizeStringArray(state?.excludes),
    hits: state?.hits ?? 0,
    misses: state?.misses ?? 0
  }
}

function normalizeHostEntry(entry: Schema<'api.HostEntry'>): HostEntry {
  return {
    domain: entry.domain ?? '',
    address: entry.address ?? ''
  }
}

function normalizeTagMember(entry: Schema<'api.TagMember'>): TagMember {
  return {
    address: entry.address ?? '',
    host: entry.host ?? '',
    has_host_alias: entry.has_host_alias ?? false
  }
}

function normalizeKnownHost(entry: Schema<'api.KnownHost'>): KnownHostCandidate {
  return {
    address: entry.address ?? '',
    host: entry.host ?? ''
  }
}

function normalizeZenModeState(state: Schema<'api.ZenModeStatus'> | undefined): ZenModeState {
  return {
    enabled: state?.enabled ?? false,
    file: state?.file ?? '',
    duration_minutes: state?.duration_minutes ?? 0,
    configured_domains: normalizeStringArray(state?.configured_domains),
    persisted_domains: normalizeStringArray(state?.persisted_domains),
    configured_excludes: normalizeStringArray(state?.configured_excludes),
    persisted_excludes: normalizeStringArray(state?.persisted_excludes),
    labels: normalizeStringArray(state?.labels),
    runtime_domains: normalizeStringArray(state?.runtime_domains),
    started_at: state?.started_at ?? '',
    ends_at: state?.ends_at ?? '',
    remaining_seconds: state?.remaining_seconds ?? 0
  }
}

function normalizeStaticResponseState(state: Schema<'api.StaticResponseStatus'> | undefined): StaticResponseState {
  return {
    enabled: state?.enabled ?? false,
    file: state?.file ?? '',
    labels: normalizeStringArray(state?.labels),
    configured_hosts: (state?.configured_hosts ?? []).map(normalizeHostEntry),
    persisted_hosts: (state?.persisted_hosts ?? []).map(normalizeHostEntry),
    runtime_hosts: (state?.runtime_hosts ?? []).map(normalizeHostEntry)
  }
}

function normalizeStubResolverState(state: Schema<'api.StubResolverStatus'> | undefined): StubResolverState {
  return {
    enabled: state?.enabled ?? false,
    configured_stubs: normalizeStringArray(state?.configured_stubs),
    runtime_stubs: normalizeStringArray(state?.runtime_stubs)
  }
}

export function useApi() {
  const { client, execute } = useApiClient()

  async function getStubResolver() {
    const response = await execute(client.GET('/api/stub-resolver'))
    return response && { ...response, stub_resolver: normalizeStubResolverState(response.stub_resolver) }
  }

  async function toggleStubResolver(action: 'start' | 'stop') {
    const response = await execute(client.POST('/api/stub-resolver/{action}', {
      params: { path: { action } }
    }))
    return response && { ...response, stub_resolver: normalizeStubResolverState(response.stub_resolver) }
  }

  async function replaceStubResolvers(stubs: string[]) {
    const response = await execute(client.POST('/api/stub-resolver', { body: { stubs } }))
    return response && { ...response, stub_resolver: normalizeStubResolverState(response.stub_resolver) }
  }

  async function getBlacklist() {
    const response = await execute(client.GET('/api/blacklist'))
    return response && { ...response, blacklist: normalizeBlacklistState(response.blacklist) }
  }

  async function toggleBlacklist(action: 'start' | 'stop') {
    const response = await execute(client.POST('/api/blacklist/{action}', {
      params: { path: { action } }
    }))
    return response && { ...response, blacklist: normalizeBlacklistState(response.blacklist) }
  }

  async function addBlacklistRuntimeWhitelist(domains: string[]) {
    const response = await execute(client.POST('/api/blacklist/whitelist', { body: { domains } }))
    return response && { ...response, blacklist: normalizeBlacklistState(response.blacklist) }
  }

  async function replaceBlacklistPersistedHosts(hosts: string[]) {
    const response = await execute(client.POST('/api/blacklist/persisted/hosts', { body: { hosts } }))
    return response && { ...response, blacklist: normalizeBlacklistState(response.blacklist) }
  }

  async function replaceBlacklistPersistedExcludes(excludes: string[]) {
    const response = await execute(client.POST('/api/blacklist/persisted/excludes', { body: { excludes } }))
    return response && { ...response, blacklist: normalizeBlacklistState(response.blacklist) }
  }

  async function getStaticResponse() {
    const response = await execute(client.GET('/api/static-response'))
    return response && { ...response, static_response: normalizeStaticResponseState(response.static_response) }
  }

  async function toggleStaticResponse(action: 'start' | 'stop') {
    const response = await execute(client.POST('/api/static-response/{action}', {
      params: { path: { action } }
    }))
    return response && { ...response, static_response: normalizeStaticResponseState(response.static_response) }
  }

  async function replaceStaticResponseHosts(hosts: string[]) {
    const response = await execute(client.POST('/api/static-response', { body: { hosts } }))
    return response && { ...response, static_response: normalizeStaticResponseState(response.static_response) }
  }

  async function replaceStaticResponsePersistedHosts(hosts: string[]) {
    const response = await execute(client.POST('/api/static-response/persisted', { body: { hosts } }))
    return response && { ...response, static_response: normalizeStaticResponseState(response.static_response) }
  }

  async function getDnsLogTop(top: number, options: {
    since?: string
    status?: 'blocked' | 'allowed' | ''
    client?: string
    client_mode?: 'host' | 'ip' | ''
  } = {}) {
    const response = await execute(client.GET('/api/dns-log/top/{top}', {
      params: {
        path: { top },
        query: {
          since: options.since || undefined,
          status: options.status || undefined,
          client: options.client || undefined,
          client_mode: options.client_mode || undefined
        }
      }
    }))
    return response && {
      ...response,
      log_items: (response.log_items ?? []).map(normalizeDnsLogItem)
    }
  }

  async function searchDnsLogClients(search = '', limit = 20) {
    const response = await execute(client.GET('/api/dns-log/clients', {
      params: { query: { search: search.trim() || undefined, limit } }
    }))
    return response && {
      ...response,
      clients: (response.clients ?? []).map(normalizeDnsLogClient)
    }
  }

  async function getDnsDashboard(hours = 24) {
    const response = await execute(client.GET('/api/dns-log/dashboard', {
      params: { query: { hours } }
    }))
    return response && {
      ...response,
      window_hours: response.window_hours ?? hours,
      summary: normalizeDashboardSummary(response.summary),
      hourly: (response.hourly ?? []).map(normalizeDashboardHourlyPoint)
    }
  }

  async function getDnsDashboardHistory() {
    const response = await execute(client.GET('/api/dns-log/dashboard/history'))
    return response && {
      ...response,
      window_hours: response.window_hours ?? 23,
      summary: normalizeDashboardSummary(response.summary),
      hourly: (response.hourly ?? []).map(normalizeDashboardHourlyPoint)
    }
  }

  async function getDnsDashboardCurrent() {
    const response = await execute(client.GET('/api/dns-log/dashboard/current'))
    return response && {
      ...response,
      window_hours: response.window_hours ?? 1,
      summary: normalizeDashboardSummary(response.summary),
      hourly: (response.hourly ?? []).map(normalizeDashboardHourlyPoint)
    }
  }

  async function rotateDnsLog(since: string) {
    return execute(client.POST('/api/dns-log/rotate', {
      params: { query: { since } }
    }))
  }

  async function setDnsLogAlias(name: string, addr: string) {
    return execute(client.POST('/api/dns-log/alias', { body: { name, addr } }))
  }

  async function getZenMode() {
    const response = await execute(client.GET('/api/zen-mode'))
    return response && { ...response, zen_mode: normalizeZenModeState(response.zen_mode) }
  }

  async function startZenMode() {
    const response = await execute(client.POST('/api/zen-mode/start'))
    return response && { ...response, zen_mode: normalizeZenModeState(response.zen_mode) }
  }

  async function replaceZenDomains(zen_domains: string[]) {
    const response = await execute(client.POST('/api/zen-mode', { body: { zen_domains } }))
    return response && { ...response, zen_mode: normalizeZenModeState(response.zen_mode) }
  }

  async function replaceZenPersistedDomains(zen_domains: string[]) {
    const response = await execute(client.POST('/api/zen-mode/persisted/domains', { body: { zen_domains } }))
    return response && { ...response, zen_mode: normalizeZenModeState(response.zen_mode) }
  }

  async function replaceZenPersistedExcludes(excludes: string[]) {
    const response = await execute(client.POST('/api/zen-mode/persisted/excludes', { body: { excludes } }))
    return response && { ...response, zen_mode: normalizeZenModeState(response.zen_mode) }
  }

  async function getCache() {
    const response = await execute(client.GET('/api/cache'))
    return response && { ...response, cache: normalizeCacheState(response.cache) }
  }

  async function clearCache() {
    const response = await execute(client.DELETE('/api/cache'))
    return response && { ...response, cache: normalizeCacheState(response.cache) }
  }

  async function toggleCache(action: 'start' | 'stop') {
    const response = await execute(client.POST('/api/cache/{action}', {
      params: { path: { action } }
    }))
    return response && { ...response, cache: normalizeCacheState(response.cache) }
  }

  async function replaceCacheExcludes(excludes: string[]) {
    const response = await execute(client.POST('/api/cache/excludes', { body: { excludes } }))
    return response && { ...response, cache: normalizeCacheState(response.cache) }
  }

  async function getTags() {
    return execute(client.GET('/api/tagger/tags'))
  }

  async function createTag(name: string) {
    return execute(client.POST('/api/tagger/tags', { body: { name } }))
  }

  async function deleteTag(tagName: string) {
    return execute(client.DELETE('/api/tagger/tags/{tagName}', {
      params: { path: { tagName } }
    }))
  }

  async function getTagMembers(tagName: string) {
    const response = await execute(client.GET('/api/tagger/tags/{tagName}', {
      params: { path: { tagName } }
    }))
    return response && {
      ...response,
      tag_members: (response.tag_members ?? []).map(normalizeTagMember)
    }
  }

  async function addTagMembers(tagName: string, members: string[]) {
    const response = await execute(client.POST('/api/tagger/tags/{tagName}', {
      params: { path: { tagName } },
      body: { members }
    }))
    return response && {
      ...response,
      tag_members: (response.tag_members ?? []).map(normalizeTagMember)
    }
  }

  async function removeTagMember(tagName: string, address: string) {
    return execute(client.DELETE('/api/tagger/tags/{tagName}/{address}', {
      params: { path: { tagName, address } }
    }))
  }

  async function getKnownHosts(search = '', limit = 20) {
    const response = await execute(client.GET('/api/tagger/hosts', {
      params: { query: { search: search.trim() || undefined, limit } }
    }))
    return response && {
      ...response,
      known_hosts: (response.known_hosts ?? []).map(normalizeKnownHost)
    }
  }

  async function setAddressLabels(address: string, tags: string[]) {
    return execute(client.POST('/api/tagger/address', { body: { address, tags } }))
  }

  async function replaceAddressLabels(address: string, tags: string[]) {
    return execute(client.PUT('/api/tagger/address/{address}', {
      params: { path: { address } },
      body: { tags }
    }))
  }

  return {
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
    getDnsDashboardHistory,
    getDnsDashboardCurrent,
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
