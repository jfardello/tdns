import type { DNSLogStatus } from '~/lib/dnsLogUi'
import { EMPTY_DNS_LOG_STATUS } from '~/lib/dnsLogUi'

export function useDnsLog() {
  const { clearDnsLog, getDnsLogStatus, toggleDnsLog, updateDnsLogPrivacy } = useApi()

  const dnsLogStatus = useState<DNSLogStatus>('dns-log-status', () => ({ ...EMPTY_DNS_LOG_STATUS }))
  const initialized = useState<boolean>('dns-log-initialized', () => false)
  const refreshing = useState<boolean>('dns-log-refreshing', () => false)
  const toggleLoading = useState<boolean>('dns-log-toggle-loading', () => false)
  const clearLoading = useState<boolean>('dns-log-clear-loading', () => false)
  const privacyLoading = useState<boolean>('dns-log-privacy-loading', () => false)
  const dataRevision = useState<number>('dns-log-data-revision', () => 0)

  function applyStatus(status: DNSLogStatus | undefined) {
    if (!status) {
      return
    }
    dnsLogStatus.value = status
    initialized.value = true
  }

  async function refresh(force = false): Promise<void> {
    if (refreshing.value || (initialized.value && !force)) {
      return
    }
    refreshing.value = true
    try {
      const response = await getDnsLogStatus()
      applyStatus(response?.dns_log)
    } finally {
      refreshing.value = false
    }
  }

  async function setEnabled(nextEnabled: boolean) {
    toggleLoading.value = true
    try {
      const response = await toggleDnsLog(nextEnabled ? 'start' : 'stop')
      applyStatus(response?.dns_log)
      if (response?.dns_log) {
        dataRevision.value++
      }
      return response
    } finally {
      toggleLoading.value = false
    }
  }

  async function clear() {
    clearLoading.value = true
    try {
      const response = await clearDnsLog()
      applyStatus(response?.dns_log)
      if (response?.dns_log) {
        dataRevision.value++
      }
      return response
    } finally {
      clearLoading.value = false
    }
  }

  async function setPseudonymization(domains: boolean, clients: boolean) {
    privacyLoading.value = true
    try {
      const response = await updateDnsLogPrivacy(domains, clients)
      applyStatus(response?.dns_log)
      if (response?.dns_log) {
        dataRevision.value++
      }
      return response
    } finally {
      privacyLoading.value = false
    }
  }

  return {
    dnsLogStatus,
    initialized,
    refreshing,
    toggleLoading,
    clearLoading,
    privacyLoading,
    dataRevision,
    refresh,
    setEnabled,
    setPseudonymization,
    clear
  }
}
