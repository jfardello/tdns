<script setup lang="ts">
definePageMeta({ layout: 'dashboard', middleware: 'auth' })

import type { DashboardHourlyPoint, DashboardSummary, DnsLogItem } from '~/composables/useApi'
import {
  dashboardSegmentsHaveGap,
  mergeDashboardSegments,
  type DashboardSegment
} from '~/lib/dashboardStats'

const { clearCache, getDnsDashboardCurrent, getDnsDashboardHistory, getDnsLogTop } = useApi()
const toast = useToast()

const EMPTY_SUMMARY: DashboardSummary = {
  total_queries: 0,
  blocked_queries: 0,
  allowed_queries: 0,
  cache_hits: 0,
  cache_misses: 0
}

const summary = ref<DashboardSummary>({ ...EMPTY_SUMMARY })
const hourly = ref<DashboardHourlyPoint[]>([])
const topDomains = ref<DnsLogItem[]>([])
const windowHours = ref(24)
const dashboardLoading = ref(false)
const currentHourLoading = ref(false)
const topDomainsLoading = ref(false)
let dashboardRequest = 0

const chartCategories = {
  allowed_queries: { name: 'Allowed', color: '#50a2ff' },
  blocked_queries: { name: 'Blocked', color: '#3a71df' }
}

const chartData = computed(() =>
  hourly.value.map(point => ({
    ...point,
    hour_label: formatHourLabel(point.hour_start)
  }))
)

function formatHourLabel(value: string) {
  const parts = value.split(' ')
  if (parts.length < 2) {
    return value
  }
  return (parts[1] ?? '').slice(0, 5)
}

function getChartPoint(index: number) {
  return chartData.value[index]
}

function formatXAxisTick(_: number, index = 0) {
  return getChartPoint(index)?.hour_label || ''
}

function getTooltipDatum(values: Record<string, unknown> | undefined) {
  const datum = values?.datum
  if (!datum || typeof datum !== 'object') {
    return undefined
  }

  return datum as DashboardHourlyPoint & { hour_label?: string }
}

function formatTooltipTitle(values: Record<string, unknown> | undefined) {
  const row = getTooltipDatum(values)
  if (!row) {
    return 'Unknown hour'
  }

  return `${row.hour_start} · Total ${row.total_queries.toLocaleString()}`
}

function formatTooltipValue(value: unknown) {
  return Number(value ?? 0).toLocaleString()
}

function getTooltipMetric(values: Record<string, unknown> | undefined, key: string) {
  return formatTooltipValue(getTooltipDatum(values)?.[key as keyof DashboardHourlyPoint])
}

function applyDashboard(history: DashboardSegment, current?: DashboardSegment) {
  const dashboard = mergeDashboardSegments(history, current)
  summary.value = dashboard.summary
  hourly.value = dashboard.hourly
  windowHours.value = dashboard.window_hours
}

async function loadCurrentDashboard(request: number, history: DashboardSegment) {
  currentHourLoading.value = true
  try {
    const current = await getDnsDashboardCurrent()
    if (request === dashboardRequest && current?.summary) {
      let completed = history
      if (dashboardSegmentsHaveGap(history, current)) {
        const refreshed = await getDnsDashboardHistory()
        if (request !== dashboardRequest) {
          return
        }
        if (refreshed?.summary) {
          completed = refreshed
        }
      }
      applyDashboard(completed, current)
    }
  } finally {
    if (request === dashboardRequest) {
      currentHourLoading.value = false
    }
  }
}

async function loadDashboard() {
  const request = ++dashboardRequest
  dashboardLoading.value = true
  currentHourLoading.value = false
  const response = await getDnsDashboardHistory()
  if (request !== dashboardRequest) {
    return
  }
  if (response?.summary) {
    applyDashboard(response)
    dashboardLoading.value = false
    void loadCurrentDashboard(request, response)
  } else {
    summary.value = { ...EMPTY_SUMMARY }
    hourly.value = []
    windowHours.value = 24
    dashboardLoading.value = false
  }
}

async function loadTopDomains() {
  topDomainsLoading.value = true
  const response = await getDnsLogTop(5, { since: '24h' })
  if (response?.log_items) {
    topDomains.value = response.log_items
  } else {
    topDomains.value = []
  }
  topDomainsLoading.value = false
}

async function loadDashboardData() {
  await Promise.all([loadDashboard(), loadTopDomains()])
}

async function handleClearCache() {
  const response = await clearCache()
  if (response) {
    toast.add({
      title: 'Cache cleared',
      description: response.message || 'DNS cache has been cleared',
      color: 'success',
      icon: 'i-lucide-check-circle'
    })
    await loadDashboard()
  }
}

onMounted(() => {
  loadDashboardData()
})
</script>

<template>
  <UDashboardPanel>
    <template #header>
      <UDashboardNavbar title="Dashboard">
        <template #right>
          <div class="flex items-center gap-2">
            <UButton
              icon="i-lucide-refresh-cw"
              color="neutral"
              variant="ghost"
              :loading="dashboardLoading || currentHourLoading || topDomainsLoading"
              @click="loadDashboardData"
            />
            <UButton
              icon="i-lucide-trash-2"
              label="Clear Cache"
              color="neutral"
              variant="outline"
              @click="handleClearCache"
            />
          </div>
        </template>
      </UDashboardNavbar>
    </template>

    <template #body>
      <div class="space-y-6 p-6">
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
          <UCard>
            <div class="flex items-start justify-between gap-4">
              <div>
                <p class="text-sm text-muted">Queries</p>
                <p class="mt-2 text-2xl font-semibold">{{ summary.total_queries.toLocaleString() }}</p>
              </div>
              <UBadge :label="`${windowHours}h`" color="primary" variant="subtle" />
            </div>
          </UCard>

          <UCard>
            <div class="flex items-start justify-between gap-4">
              <div>
                <p class="text-sm text-muted">Blocked</p>
                <p class="mt-2 text-2xl font-semibold">{{ summary.blocked_queries.toLocaleString() }}</p>
              </div>
              <UBadge label="24h filter" color="error" variant="subtle" />
            </div>
          </UCard>

          <UCard>
            <div class="flex items-start justify-between gap-4">
              <div>
                <p class="text-sm text-muted">Cache hits</p>
                <p class="mt-2 text-2xl font-semibold">{{ summary.cache_hits.toLocaleString() }}</p>
              </div>
              <UBadge label="Since start" color="success" variant="subtle" />
            </div>
          </UCard>

          <UCard>
            <div class="flex items-start justify-between gap-4">
              <div>
                <p class="text-sm text-muted">Cache misses</p>
                <p class="mt-2 text-2xl font-semibold">{{ summary.cache_misses.toLocaleString() }}</p>
              </div>
              <UBadge label="Since start" color="warning" variant="subtle" />
            </div>
          </UCard>
        </div>

        <UCard>
          <template #header>
            <div class="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
              <div>
                <h3 class="font-semibold">Traffic by Hour</h3>
                <p class="text-sm text-muted">Allowed and blocked DNS queries across the last {{ windowHours }} hours.</p>
              </div>
              <div class="flex items-center gap-2 text-sm text-muted">
                <UIcon
                  v-if="currentHourLoading"
                  name="i-lucide-loader-circle"
                  class="size-4 animate-spin"
                  aria-label="Loading current hour"
                />
                <span>Total {{ summary.total_queries.toLocaleString() }}</span>
                <span>Blocked {{ summary.blocked_queries.toLocaleString() }}</span>
              </div>
            </div>
          </template>

          <div v-if="dashboardLoading" class="space-y-3">
            <USkeleton class="h-[320px] w-full" />
          </div>

          <div v-else-if="chartData.length === 0" class="py-8">
            <UEmpty
              icon="i-lucide-bar-chart-3"
              title="No dashboard data yet"
              description="Hourly DNS traffic will appear here after queries are recorded."
            />
          </div>

          <ClientOnly v-else>
            <div class="dashboard-hourly-chart">
              <BarChart
                :data="chartData"
                :height="320"
                :categories="chartCategories"
                :x-formatter="formatXAxisTick"
                :y-axis="['allowed_queries', 'blocked_queries']"
                :stacked="true"
                :y-grid-line="true"
                :x-grid-line="false"
                :x-num-ticks="Math.min(windowHours, 8)"
                :y-num-ticks="5"
                :tooltip-title-formatter="formatTooltipTitle"
              >
                <template #tooltip="slotProps">
                  <div class="min-w-[220px]">
                    <div class="border-b border-white/10 px-4 pb-2 pt-3">
                      <p class="text-sm font-semibold text-highlighted">
                        {{ formatTooltipTitle(slotProps.values) }}
                      </p>
                    </div>

                    <div class="space-y-2 px-4 py-3 text-sm">
                      <div class="flex items-center justify-between gap-4">
                        <div class="flex items-center gap-2">
                          <span class="size-2 rounded-full" style="background-color: #00dc82;" />
                          <span class="text-toned">Allowed</span>
                        </div>
                        <span class="font-medium text-highlighted">
                          {{ getTooltipMetric(slotProps.values, 'allowed_queries') }}
                        </span>
                      </div>

                      <div class="flex items-center justify-between gap-4">
                        <div class="flex items-center gap-2">
                          <span class="size-2 rounded-full" style="background-color: #E06945;" />
                          <span class="text-toned">Blocked</span>
                        </div>
                        <span class="font-medium text-highlighted">
                          {{ getTooltipMetric(slotProps.values, 'blocked_queries') }}
                        </span>
                      </div>

                      <div class="flex items-center justify-between gap-4 border-t border-white/10 pt-2">
                        <span class="text-toned">Total</span>
                        <span class="font-semibold text-highlighted">
                          {{ getTooltipDatum(slotProps.values)?.total_queries?.toLocaleString() || '0' }}
                        </span>
                      </div>
                    </div>
                  </div>
                </template>
              </BarChart>
            </div>
          </ClientOnly>
        </UCard>

        <UCard>
          <template #header>
            <div class="flex items-center justify-between">
              <h3 class="font-semibold">Top Domains (Last 24h)</h3>
              <UButton
                icon="i-lucide-refresh-cw"
                variant="ghost"
                size="sm"
                :loading="topDomainsLoading"
                @click="loadTopDomains"
              />
            </div>
          </template>

          <div v-if="topDomainsLoading" class="space-y-2">
            <USkeleton v-for="i in 5" :key="i" class="h-10 w-full" />
          </div>

          <div v-else-if="topDomains.length === 0" class="py-8 text-center">
            <UEmpty
              icon="i-lucide-globe"
              title="No data yet"
              description="DNS query logs will appear here."
            />
          </div>

          <div v-else class="space-y-2">
            <div
              v-for="(domain, index) in topDomains"
              :key="domain.domain"
              class="flex items-center justify-between rounded-lg bg-muted/50 p-3"
            >
              <div class="flex items-center gap-3">
                <UBadge :label="String(index + 1)" color="neutral" variant="subtle" />
                <div>
                  <p class="font-medium">{{ domain.domain }}</p>
                  <p class="text-sm text-muted">{{ domain.host }}</p>
                </div>
              </div>
              <UBadge :label="`${domain.counter} queries`" color="primary" variant="subtle" />
            </div>
          </div>

          <template #footer>
            <NuxtLink to="/dashboard/top-domains">
              <UButton variant="ghost" label="View all" trailing-icon="i-lucide-arrow-right" />
            </NuxtLink>
          </template>
        </UCard>

        <UCard>
          <template #header>
            <h3 class="font-semibold">Quick Actions</h3>
          </template>

          <div class="grid grid-cols-2 gap-4 md:grid-cols-4">
            <NuxtLink to="/dashboard/tags">
              <UButton
                icon="i-lucide-tags"
                label="Manage Tags"
                color="neutral"
                variant="outline"
                block
              />
            </NuxtLink>
            <NuxtLink to="/dashboard/zen-mode">
              <UButton
                icon="i-lucide-focus"
                label="Zen Mode"
                color="neutral"
                variant="outline"
                block
              />
            </NuxtLink>
            <NuxtLink to="/dashboard/top-domains">
              <UButton
                icon="i-lucide-bar-chart-3"
                label="Top Domains"
                color="neutral"
                variant="outline"
                block
              />
            </NuxtLink>
            <NuxtLink to="/dashboard/stub-resolver">
              <UButton
                icon="i-lucide-git-branch"
                label="Stub Resolver"
                color="neutral"
                variant="outline"
                block
              />
            </NuxtLink>
          </div>
        </UCard>
      </div>
    </template>
  </UDashboardPanel>
</template>

<style scoped>
.dashboard-hourly-chart {
  --vis-tooltip-background-color: rgba(17, 19, 24, 0.96);
  --vis-tooltip-border-color: rgba(255, 255, 255, 0.08);
  --vis-tooltip-text-color: rgb(241, 245, 249);
  --vis-tooltip-box-shadow: 0 20px 40px rgba(0, 0, 0, 0.35);
  --vis-tooltip-padding: 0;
  --vis-tooltip-border-radius: 0.75rem;
}
</style>
