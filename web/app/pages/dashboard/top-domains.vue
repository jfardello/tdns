<script setup lang="ts">
import { z } from 'zod'
import type { FormSubmitEvent, TableColumn } from '@nuxt/ui'
import type { DnsLogItem } from '~/composables/useApi'

definePageMeta({ layout: 'dashboard', middleware: 'auth' })

const { getDnsLogTop, rotateDnsLog, setDnsLogAlias } = useApi()
const toast = useToast()

const loading = ref(false)
const logs = ref<DnsLogItem[]>([])
const topCount = ref(50)
const sinceFilter = ref('24h')

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

async function loadLogs() {
  loading.value = true
  const response = await getDnsLogTop(topCount.value, sinceFilter.value)
  if (response?.log_items) {
    logs.value = response.log_items
  } else {
    logs.value = []
  }
  loading.value = false
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
  }
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
  loadLogs()
})

watch([topCount, sinceFilter], () => {
  loadLogs()
})
</script>

<template>
  <UDashboardPanel>
    <template #header>
      <UDashboardNavbar title="Top Domains">
        <template #right>
          <div class="flex items-center gap-2">
            <UButton
              icon="i-lucide-user-plus"
              label="Set Alias"
              variant="outline"
              color="neutral"
              @click="showAliasModal = true"
            />
            <UButton
              icon="i-lucide-trash-2"
              label="Rotate Logs"
              variant="outline"
              color="error"
              @click="showRotateModal = true"
            />
          </div>
        </template>
      </UDashboardNavbar>

      <UDashboardToolbar>
        <template #left>
          <div class="flex items-center gap-4">
            <UFormField label="Time Period">
              <USelect v-model="sinceFilter" :items="sinceOptions" value-key="value" />
            </UFormField>
            <UFormField label="Show Top">
              <UInputNumber v-model="topCount" :min="10" :max="500" :step="10" />
            </UFormField>
          </div>
        </template>
        <template #right>
          <UButton
            icon="i-lucide-refresh-cw"
            variant="ghost"
            :loading="loading"
            @click="loadLogs"
          />
        </template>
      </UDashboardToolbar>
    </template>

    <template #body>
      <div class="p-6">
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
              <div class="flex items-center gap-2">
                <UIcon name="i-lucide-monitor" class="size-4 text-muted" />
                <span class="font-mono text-sm">{{ row.original.host }}</span>
              </div>
            </template>

            <template #counter-cell="{ row }">
              <UBadge :label="`${row.original.counter}`" color="primary" variant="subtle" />
            </template>

            <template #actions-cell="{ row }">
              <UDropdownMenu
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
          <span>Showing {{ logs.length }} entries</span>
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
          <UButton variant="ghost" label="Cancel" @click="showRotateModal = false" />
          <UButton type="submit" label="Rotate Logs" color="error" />
        </div>
      </UForm>
    </template>
  </UModal>

  <!-- Alias Modal -->
  <UModal v-model:open="showAliasModal" title="Set Host Alias" description="Assign a friendly name to a client IP address">
    <template #body>
      <UForm :schema="aliasSchema" :state="aliasState" class="space-y-4" @submit="handleSetAlias">
        <UFormField name="addr" label="IP Address">
          <UInput v-model="aliasState.addr" placeholder="192.168.1.100" />
        </UFormField>
        <UFormField name="name" label="Alias Name">
          <UInput v-model="aliasState.name" placeholder="e.g., Living Room TV" />
        </UFormField>
        <div class="flex justify-end gap-2">
          <UButton variant="ghost" label="Cancel" @click="showAliasModal = false" />
          <UButton type="submit" label="Set Alias" />
        </div>
      </UForm>
    </template>
  </UModal>
</template>
