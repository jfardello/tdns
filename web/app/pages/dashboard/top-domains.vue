<script setup lang="ts">
import { z } from 'zod'
import type { FormSubmitEvent, TableColumn } from '@nuxt/ui'
import type { DnsLogClientCandidate, DnsLogItem } from '~/composables/useApi'
import {
  canConfirmDNSLogClear,
  canStartDNSLog,
  isAliasableDNSLogClient,
  isDNSLogClientToken
} from '~/lib/dnsLogUi'

definePageMeta({ layout: 'dashboard', middleware: 'auth' })

const { getDnsLogTop, searchDnsLogClients, rotateDnsLog, setDnsLogAlias } = useApi()
const {
  dnsLogStatus,
  initialized: dnsLogInitialized,
  refreshing: dnsLogRefreshing,
  toggleLoading: dnsLogToggleLoading,
  clearLoading: dnsLogClearLoading,
  refresh: refreshDNSLogStatus,
  setEnabled: setDNSLogEnabled,
  clear: clearAllDNSLogData
} = useDnsLog()
const toast = useToast()

const loading = ref(false)
const logs = ref<DnsLogItem[]>([])
const topCount = ref(50)
const sinceFilter = ref('24h')
const filterByStatus = ref(false)
const statusShowsBlocked = ref(true)
const filterByClient = ref(false)
const clientSearch = ref('')
const clientSearchLoading = ref(false)
const selectedClientValue = ref('')
const clientOptions = ref<Array<{ label: string, value: string, mode: 'host' | 'ip' }>>([])

interface TopDomainRow extends DnsLogItem {
  rank: number
}

const sinceOptions = [
  { label: 'Last Hour', value: '1h' },
  { label: 'Last 24 Hours', value: '24h' },
  { label: 'Last 7 Days', value: '7d' },
  { label: 'Last 30 Days', value: '30d' },
  { label: 'Last 180 Days', value: '180d' }
]

// Rotate modal
const showRotateModal = ref(false)
const rotateSchema = z.object({
  since: z.string().min(1, 'Duration is required')
})
const rotateState = reactive({ since: '30d' })

// Alias modal
const showAliasModal = ref(false)
const aliasSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  addr: z.string().min(1, 'Address is required')
})
const aliasState = reactive({ name: '', addr: '' })

// Complete deletion modal
const showClearModal = ref(false)
const clearConfirmation = ref('')
const clearConfirmationValid = computed(() => (
  canConfirmDNSLogClear(dnsLogStatus.value, clearConfirmation.value)
))
const privacyUsesKey = computed(() => (
  dnsLogStatus.value.domains_pseudonymized || dnsLogStatus.value.clients_pseudonymized
))

async function loadLogs() {
  loading.value = true
  try {
    const response = await getDnsLogTop(topCount.value, {
      since: sinceFilter.value,
      status: filterByStatus.value ? (statusShowsBlocked.value ? 'blocked' : 'allowed') : '',
      client: filterByClient.value ? selectedClientValue.value : '',
      client_mode: filterByClient.value ? selectedClientMode.value : ''
    })
    logs.value = response?.log_items ?? []
  } finally {
    loading.value = false
  }
}

function optionLabel(item: DnsLogClientCandidate, mode: 'host' | 'ip') {
  const address = isDNSLogClientToken(item.address)
    ? `${item.address} · pseudonymized token`
    : item.address
  if (!item.host) {
    return address
  }
  return mode === 'host'
    ? `${item.host} · alias (${address})`
    : `${address} (${item.host})`
}

const selectedClientMode = computed<'host' | 'ip' | ''>(() => {
  if (!filterByClient.value || !selectedClientValue.value) {
    return ''
  }
  const match = clientOptions.value.find(item => item.value === selectedClientValue.value)
  return match?.mode ?? ''
})

const activeFilterSummary = computed(() => {
  const parts: string[] = []
  if (filterByStatus.value) {
    parts.push(statusShowsBlocked.value ? 'blocked queries' : 'allowed queries')
  }
  if (filterByClient.value && selectedClientValue.value) {
    const selected = clientOptions.value.find(item => item.value === selectedClientValue.value)
    parts.push(`client ${selected?.label ?? selectedClientValue.value}`)
  }
  if (parts.length === 0) {
    return `last ${sinceFilter.value}`
  }
  return `${parts.join(' for ')} in the last ${sinceFilter.value}`
})

async function loadClientOptions() {
  clientSearchLoading.value = true
  try {
    const response = await searchDnsLogClients(clientSearch.value, 25)
    const clients = response?.clients ?? []
    clientOptions.value = clients.flatMap((item) => {
      const options: Array<{ label: string, value: string, mode: 'host' | 'ip' }> = []
      if (item.host) {
        options.push({
          label: optionLabel(item, 'host'),
          value: item.host,
          mode: 'host'
        })
      }
      options.push({
        label: optionLabel(item, 'ip'),
        value: item.address,
        mode: 'ip'
      })
      return options
    })
  } finally {
    clientSearchLoading.value = false
  }
}

async function handleRotate(event: FormSubmitEvent<z.output<typeof rotateSchema>>) {
  const response = await rotateDnsLog(event.data.since)
  if (response) {
    toast.add({
      title: 'Logs rotated',
      description: response.message || `Logs older than ${event.data.since} have been deleted`,
      color: 'success',
      icon: 'i-lucide-check-circle'
    })
    showRotateModal.value = false
    loadLogs()
  }
}

async function handleSetAlias(event: FormSubmitEvent<z.output<typeof aliasSchema>>) {
  const response = await setDnsLogAlias(event.data.name, event.data.addr)
  if (response) {
    toast.add({
      title: 'Alias set',
      description: `Alias "${event.data.name}" assigned to ${event.data.addr}`,
      color: 'success',
      icon: 'i-lucide-check-circle'
    })
    showAliasModal.value = false
    aliasState.name = ''
    aliasState.addr = ''
    await Promise.all([loadLogs(), loadClientOptions()])
  }
}

async function refreshDNSLogViews() {
  await Promise.all([
    refreshDNSLogStatus(true),
    loadLogs(),
    loadClientOptions()
  ])
}

async function handleLoggingToggle(nextEnabled: boolean) {
  if (nextEnabled && !canStartDNSLog(dnsLogStatus.value)) {
    return
  }
  const response = await setDNSLogEnabled(nextEnabled)
  if (!response) {
    return
  }
  toast.add({
    title: `DNS logging ${nextEnabled ? 'started' : 'stopped'}`,
    description: nextEnabled
      ? 'New DNS queries are now being recorded'
      : 'Accepted DNS-log events were flushed before logging stopped',
    color: 'success',
    icon: 'i-lucide-check-circle'
  })
  await Promise.all([loadLogs(), loadClientOptions()])
}

function openClearModal() {
  clearConfirmation.value = ''
  showClearModal.value = true
}

async function handleCompleteDeletion() {
  if (!clearConfirmationValid.value) {
    return
  }
  const response = await clearAllDNSLogData()
  if (!response) {
    return
  }

  logs.value = []
  clientOptions.value = []
  clientSearch.value = ''
  selectedClientValue.value = ''
  filterByClient.value = false
  clearConfirmation.value = ''
  showClearModal.value = false
  toast.add({
    title: 'DNS-log data deleted',
    description: 'Events, dashboard aggregates, aliases, queues and sequence state were deleted',
    color: 'success',
    icon: 'i-lucide-check-circle'
  })
  await Promise.all([loadLogs(), loadClientOptions()])
}

function setAliasFromHost(host: string) {
  aliasState.addr = host
  showAliasModal.value = true
}

// Table columns
const columns: TableColumn<TopDomainRow>[] = [
  { accessorKey: 'rank', header: '#' },
  { accessorKey: 'domain', header: 'Domain' },
  { accessorKey: 'host', header: 'Client' },
  { accessorKey: 'counter', header: 'Queries' },
  { id: 'actions', header: '' }
]

onMounted(() => {
  void refreshDNSLogViews()
})

watch([topCount, sinceFilter, filterByStatus, statusShowsBlocked, filterByClient, selectedClientValue], () => {
  loadLogs()
})

watch(clientSearch, () => {
  loadClientOptions()
})

watch(filterByClient, (enabled) => {
  if (!enabled) {
    clientSearch.value = ''
    selectedClientValue.value = ''
  } else {
    loadClientOptions()
  }
})
</script>

<template>
  <UDashboardPanel>
    <template #header>
      <UDashboardNavbar title="Top Domains">
        <template #right>
          <div class="flex flex-wrap items-center gap-2">
            <UButton
              icon="i-lucide-user-plus"
              label="Set Alias"
              variant="outline"
              color="neutral"
              @click="() => { showAliasModal = true }"
            />
            <UButton
              icon="i-lucide-trash-2"
              label="Rotate Logs"
              variant="outline"
              color="error"
              @click="() => { showRotateModal = true }"
            />
          </div>
        </template>
      </UDashboardNavbar>

      <UDashboardToolbar>
        <template #left>
          <div class="flex flex-wrap items-end gap-4">
            <UFormField label="Time Period">
              <USelect v-model="sinceFilter" :items="sinceOptions" value-key="value" />
            </UFormField>
            <UFormField label="Show Top">
              <UInputNumber v-model="topCount" :min="10" :max="500" :step="10" />
            </UFormField>
            <div class="flex items-center gap-3 rounded-lg border border-default px-3 py-2">
              <UCheckbox v-model="filterByStatus" />
              <span class="text-sm font-medium">Filter by status</span>
              <span class="text-sm text-muted">Allowed</span>
              <USwitch
                :model-value="statusShowsBlocked"
                :disabled="!filterByStatus"
                @update:model-value="statusShowsBlocked = $event"
              />
              <span class="text-sm text-muted">Blocked</span>
            </div>
            <div class="flex items-center gap-3 rounded-lg border border-default px-3 py-2">
              <UCheckbox v-model="filterByClient" />
              <span class="text-sm font-medium">Filter by client</span>
            </div>
            <template v-if="filterByClient">
              <UFormField
                label="Search clients"
                :description="dnsLogStatus.clients_pseudonymized ? 'Use an exact client address or full h1c token; alias text can be partial.' : undefined"
              >
                <UInput
                  v-model="clientSearch"
                  placeholder="Search by alias, address, or client token"
                  :loading="clientSearchLoading"
                />
              </UFormField>
              <UFormField label="Choose client">
                <USelect
                  v-model="selectedClientValue"
                  :items="clientOptions"
                  value-key="value"
                  placeholder="Select alias, address, or token"
                />
              </UFormField>
            </template>
          </div>
        </template>
        <template #right>
          <UButton
            icon="i-lucide-refresh-cw"
            variant="ghost"
            :loading="loading || dnsLogRefreshing"
            @click="refreshDNSLogViews"
          />
        </template>
      </UDashboardToolbar>
    </template>

    <template #body>
      <div class="space-y-6 p-6">
        <div v-if="!dnsLogInitialized && dnsLogRefreshing" class="space-y-3">
          <USkeleton class="h-32 w-full" />
        </div>

        <template v-else>
          <UAlert
            v-if="dnsLogStatus.requires_clear"
            icon="i-lucide-triangle-alert"
            color="warning"
            title="Stored DNS-log data is incompatible with the configured privacy settings"
            :description="dnsLogStatus.reason || 'Delete all DNS-log data before logging can resume.'"
          />

          <UCard>
            <div class="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
              <div class="space-y-3">
                <div class="flex flex-wrap items-center gap-2">
                  <h3 class="font-semibold">DNS logging</h3>
                  <UBadge
                    :label="dnsLogStatus.enabled ? 'Active' : 'Stopped'"
                    :color="dnsLogStatus.enabled ? 'success' : 'neutral'"
                    variant="subtle"
                  />
                </div>
                <div class="flex flex-wrap gap-2 text-sm">
                  <UBadge
                    :label="`Domains: ${dnsLogStatus.domains_pseudonymized ? 'pseudonymized' : 'plain'}`"
                    :color="dnsLogStatus.domains_pseudonymized ? 'primary' : 'neutral'"
                    variant="outline"
                  />
                  <UBadge
                    :label="`Clients: ${dnsLogStatus.clients_pseudonymized ? 'pseudonymized' : 'plain'}`"
                    :color="dnsLogStatus.clients_pseudonymized ? 'primary' : 'neutral'"
                    variant="outline"
                  />
                  <UBadge
                    v-if="privacyUsesKey"
                    :label="dnsLogStatus.key_configured ? 'Privacy key configured' : 'Privacy key unavailable'"
                    :color="dnsLogStatus.key_configured ? 'success' : 'error'"
                    variant="outline"
                  />
                  <UBadge
                    v-if="dnsLogStatus.queued_events > 0"
                    :label="`${dnsLogStatus.queued_events} queued event${dnsLogStatus.queued_events === 1 ? '' : 's'}`"
                    color="warning"
                    variant="outline"
                  />
                </div>
                <p v-if="dnsLogStatus.enabled" class="text-sm text-muted">New DNS queries are being recorded.</p>
                <p v-else class="text-sm text-muted">Historical data remains available until it is rotated or completely deleted.</p>
                <p v-if="dnsLogStatus.enabled" class="text-sm text-warning">
                  Stop DNS logging before completely deleting its stored data.
                </p>
              </div>

              <div class="flex flex-wrap items-center gap-4">
                <div class="flex items-center gap-3 rounded-lg border border-default px-3 py-2">
                  <span class="text-sm font-medium">Enabled</span>
                  <USwitch
                    :model-value="dnsLogStatus.enabled"
                    :loading="dnsLogToggleLoading"
                    :disabled="dnsLogStatus.requires_clear && !dnsLogStatus.enabled"
                    @update:model-value="handleLoggingToggle"
                  />
                </div>
                <UButton
                  icon="i-lucide-trash"
                  label="Delete All DNS-log Data"
                  color="error"
                  variant="soft"
                  :disabled="dnsLogStatus.enabled"
                  @click="openClearModal"
                />
              </div>
            </div>
          </UCard>
        </template>

        <UCard>
          <div v-if="loading" class="space-y-2">
            <USkeleton class="h-12 w-full" v-for="i in 10" :key="i" />
          </div>

          <div v-else-if="logs.length === 0" class="py-12">
            <UEmpty
              icon="i-lucide-bar-chart-3"
              title="No DNS logs"
              description="DNS query logs will appear here once there is activity"
            />
          </div>

          <UTable v-else :data="logs.map((log, i) => ({ ...log, rank: i + 1 }))" :columns="columns">
            <template #rank-cell="{ row }">
              <UBadge :label="String(row.original.rank)" color="neutral" variant="subtle" />
            </template>

            <template #domain-cell="{ row }">
              <span class="font-medium">{{ row.original.domain }}</span>
            </template>

            <template #host-cell="{ row }">
              <div class="flex min-w-0 items-center gap-2">
                <UIcon name="i-lucide-monitor" class="size-4 text-muted" />
                <span class="break-all font-mono text-sm">{{ row.original.host }}</span>
                <UBadge
                  v-if="isDNSLogClientToken(row.original.host)"
                  label="Pseudonymized"
                  color="primary"
                  variant="subtle"
                  size="sm"
                />
              </div>
            </template>

            <template #counter-cell="{ row }">
              <UBadge :label="`${row.original.counter}`" color="primary" variant="subtle" />
            </template>

            <template #actions-cell="{ row }">
              <UDropdownMenu
                v-if="isAliasableDNSLogClient(row.original.host)"
                :items="[
                  [
                    { label: 'Set Alias', icon: 'i-lucide-user-plus', onSelect: () => setAliasFromHost(row.original.host) }
                  ]
                ]"
              >
                <UButton icon="i-lucide-more-vertical" variant="ghost" size="sm" />
              </UDropdownMenu>
            </template>
          </UTable>
        </UCard>

        <!-- Summary -->
        <div class="mt-4 flex items-center justify-between text-sm text-muted">
          <span>Showing {{ logs.length }} entries for {{ activeFilterSummary }}</span>
          <span>Total queries: {{ logs.reduce((sum, l) => sum + l.counter, 0).toLocaleString() }}</span>
        </div>
      </div>
    </template>
  </UDashboardPanel>

  <!-- Rotate Modal -->
  <UModal v-model:open="showRotateModal" title="Rotate DNS Logs" description="Delete logs older than the specified duration">
    <template #body>
      <UForm :schema="rotateSchema" :state="rotateState" class="space-y-4" @submit="handleRotate">
        <UFormField name="since" label="Delete logs older than">
          <USelect
            v-model="rotateState.since"
            :items="[
              { label: '7 Days', value: '7d' },
              { label: '30 Days', value: '30d' },
              { label: '90 Days', value: '90d' },
              { label: '180 Days', value: '180d' },
              { label: '1 Year', value: '365d' }
            ]"
            value-key="value"
          />
        </UFormField>
        <UAlert
          icon="i-lucide-alert-triangle"
          color="warning"
          title="Warning"
          description="This action cannot be undone. Old logs will be permanently deleted."
        />
        <div class="flex justify-end gap-2">
          <UButton variant="ghost" label="Cancel" @click="() => { showRotateModal = false }" />
          <UButton type="submit" label="Rotate Logs" color="error" />
        </div>
      </UForm>
    </template>
  </UModal>

  <!-- Alias Modal -->
  <UModal v-model:open="showAliasModal" title="Set Host Alias" description="Assign a friendly name to a client address or pseudonymized token">
    <template #body>
      <UForm :schema="aliasSchema" :state="aliasState" class="space-y-4" @submit="handleSetAlias">
        <UFormField name="addr" label="Client address or token">
          <UInput v-model="aliasState.addr" placeholder="192.168.1.100 or h1c_…" />
        </UFormField>
        <UFormField name="name" label="Alias Name">
          <UInput v-model="aliasState.name" placeholder="e.g., Living Room TV" />
        </UFormField>
        <div class="flex justify-end gap-2">
          <UButton variant="ghost" label="Cancel" @click="() => { showAliasModal = false }" />
          <UButton type="submit" label="Set Alias" />
        </div>
      </UForm>
    </template>
  </UModal>

  <!-- Complete deletion modal -->
  <UModal
    v-model:open="showClearModal"
    title="Delete All DNS-log Data"
    description="Permanently delete DNS-log events and all derived data"
  >
    <template #body>
      <div class="space-y-4">
        <UAlert
          icon="i-lucide-triangle-alert"
          color="error"
          title="This action cannot be undone"
          description="Events, dashboard aggregates, client aliases, queued data and sequence state will be permanently deleted. Age-based log rotation is a separate operation."
        />
        <UAlert
          v-if="dnsLogStatus.enabled"
          icon="i-lucide-circle-alert"
          color="warning"
          title="DNS logging must be stopped first"
          description="Close this dialog and disable DNS logging before deleting its data."
        />
        <UFormField label="Type DELETE to confirm">
          <UInput
            v-model="clearConfirmation"
            autocomplete="off"
            placeholder="DELETE"
            :disabled="dnsLogStatus.enabled || dnsLogClearLoading"
          />
        </UFormField>
        <div class="flex flex-wrap justify-end gap-2">
          <UButton variant="ghost" label="Cancel" :disabled="dnsLogClearLoading" @click="() => { showClearModal = false }" />
          <UButton
            label="Delete All Data"
            icon="i-lucide-trash"
            color="error"
            :loading="dnsLogClearLoading"
            :disabled="!clearConfirmationValid"
            @click="handleCompleteDeletion"
          />
        </div>
      </div>
    </template>
  </UModal>
</template>
