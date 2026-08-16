export interface DNSLogStatus {
  enabled: boolean
  domains_pseudonymized: boolean
  clients_pseudonymized: boolean
  key_configured: boolean
  queued_events: number
  reason: string
  requires_clear: boolean
}

export const EMPTY_DNS_LOG_STATUS: DNSLogStatus = {
  enabled: false,
  domains_pseudonymized: false,
  clients_pseudonymized: false,
  key_configured: false,
  queued_events: 0,
  reason: '',
  requires_clear: false
}

export function normalizeDNSLogStatus(value: Partial<DNSLogStatus> | undefined): DNSLogStatus {
  return {
    enabled: value?.enabled ?? false,
    domains_pseudonymized: value?.domains_pseudonymized ?? false,
    clients_pseudonymized: value?.clients_pseudonymized ?? false,
    key_configured: value?.key_configured ?? false,
    queued_events: value?.queued_events ?? 0,
    reason: value?.reason ?? '',
    requires_clear: value?.requires_clear ?? false
  }
}

export function isDNSLogClientToken(value: string): boolean {
  return /^h1c_[A-Za-z0-9_-]{43}$/.test(value.trim())
}

export function isAliasableDNSLogClient(value: string): boolean {
  const candidate = value.trim()
  if (isDNSLogClientToken(candidate)) {
    return true
  }
  if (/^(?:\d{1,3}\.){3}\d{1,3}$/.test(candidate)) {
    return candidate.split('.').every(part => Number(part) <= 255)
  }
  return candidate.includes(':') && /^[0-9a-f:.]+$/i.test(candidate)
}

export function canStartDNSLog(status: DNSLogStatus): boolean {
  return !status.requires_clear
}

export function canClearDNSLog(status: DNSLogStatus): boolean {
  return !status.enabled
}

export function canConfirmDNSLogClear(status: DNSLogStatus, confirmation: string): boolean {
  return canClearDNSLog(status) && confirmation.trim() === 'DELETE'
}

