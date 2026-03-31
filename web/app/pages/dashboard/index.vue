<script setup lang="ts">
definePageMeta({ layout: 'dashboard', middleware: 'auth' })

import type { DnsLogItem } from '~/composables/useApi'

const { clearCache, getDnsLogTop } = useApi()
const toast = useToast()

const loading = ref(false)
const topDomains = ref<DnsLogItem[]>([])

async function loadTopDomains() {
  loading.value = true
  const response = await getDnsLogTop(5, '24h')
  if (response?.log_items) {
    topDomains.value = response.log_items
  } else {
    topDomains.value = []
  }
  loading.value = false
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
  }
}

onMounted(() => {
  loadTopDomains()
})
</script>

<template>
  <UDashboardPanel>
    <template #header>
      <UDashboardNavbar title="Dashboard">
        <template #right>
          <UButton
            icon="i-lucide-trash-2"
            label="Clear Cache"
            color="neutral"
            variant="outline"
            @click="handleClearCache"
          />
        </template>
      </UDashboardNavbar>
    </template>

    <template #body>
      <div class="p-6 space-y-6">
        <!-- Stats Cards -->
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
          <UCard>
            <div class="flex items-center gap-4">
              <div class="p-3 rounded-full bg-primary/10">
                <UIcon name="i-lucide-server" class="size-6 text-primary" />
              </div>
              <div>
                <p class="text-sm text-muted">DNS Server</p>
                <p class="text-xl font-semibold">Active</p>
              </div>
            </div>
          </UCard>

          <UCard>
            <div class="flex items-center gap-4">
              <div class="p-3 rounded-full bg-success/10">
                <UIcon name="i-lucide-shield-check" class="size-6 text-success" />
              </div>
              <div>
                <p class="text-sm text-muted">Protection</p>
                <p class="text-xl font-semibold">Enabled</p>
              </div>
            </div>
          </UCard>

          <UCard>
            <div class="flex items-center gap-4">
              <div class="p-3 rounded-full bg-warning/10">
                <UIcon name="i-lucide-activity" class="size-6 text-warning" />
              </div>
              <div>
                <p class="text-sm text-muted">Queries (24h)</p>
                <p class="text-xl font-semibold">{{ topDomains.reduce((sum, d) => sum + d.counter, 0) }}</p>
              </div>
            </div>
          </UCard>
        </div>

        <!-- Top Domains Quick View -->
        <UCard>
          <template #header>
            <div class="flex items-center justify-between">
              <h3 class="font-semibold">Top Domains (Last 24h)</h3>
              <UButton
                icon="i-lucide-refresh-cw"
                variant="ghost"
                size="sm"
                :loading="loading"
                @click="loadTopDomains"
              />
            </div>
          </template>

          <div v-if="loading" class="space-y-2">
            <USkeleton class="h-10 w-full" v-for="i in 5" :key="i" />
          </div>

          <div v-else-if="topDomains.length === 0" class="text-center py-8">
            <UEmpty
              icon="i-lucide-globe"
              title="No data yet"
              description="DNS query logs will appear here"
            />
          </div>

          <div v-else class="space-y-2">
            <div
              v-for="(domain, index) in topDomains"
              :key="domain.domain"
              class="flex items-center justify-between p-3 rounded-lg bg-muted/50"
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

        <!-- Quick Actions -->
        <UCard>
          <template #header>
            <h3 class="font-semibold">Quick Actions</h3>
          </template>

          <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
            <NuxtLink to="/dashboard/tags">
              <UButton
                icon="i-lucide-tags"
                label="Manage Tags"
                color="neutral"
                variant="outline"
                block
              />
            </NuxtLink>
            <NuxtLink to="/dashboard/plugins">
              <UButton
                icon="i-lucide-plug"
                label="Plugins"
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
            <NuxtLink to="/dashboard/blacklist">
              <UButton
                icon="i-lucide-shield-ban"
                label="Blacklist"
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
