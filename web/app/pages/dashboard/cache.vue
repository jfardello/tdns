<script setup lang="ts">
import { z } from 'zod'
import type { FormSubmitEvent } from '@nuxt/ui'

definePageMeta({ layout: 'dashboard', middleware: 'auth' })

const toast = useToast()
const {
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
} = useCache()

const excludesSchema = z.object({
  excludes: z.string().optional()
})

const excludesState = reactive({
  excludes: ''
})

function splitLines(value: string) {
  return value
    .split('\n')
    .map(line => line.trim())
    .filter(Boolean)
}

async function handleToggle(nextEnabled: boolean) {
  const response = await setEnabled(nextEnabled)
  if (!response) {
    return
  }

  toast.add({
    title: `Cache ${nextEnabled ? 'enabled' : 'disabled'}`,
    description: response.message || `DNS cache is now ${nextEnabled ? 'active' : 'inactive'}`,
    color: 'success',
    icon: 'i-lucide-check-circle'
  })
}

async function handleReplaceExcludes(event: FormSubmitEvent<z.output<typeof excludesSchema>>) {
  const excludes = splitLines(event.data.excludes || '')
  const response = await setExcludes(excludes)
  if (!response) {
    return
  }

  toast.add({
    title: 'Cache exclusions updated',
    description: `${excludes.length} persisted cache exclusion entr${excludes.length === 1 ? 'y' : 'ies'} saved`,
    color: 'success',
    icon: 'i-lucide-check-circle'
  })
}

async function handleClearCache() {
  const response = await clear()
  if (!response) {
    return
  }

  toast.add({
    title: 'Cache cleared',
    description: response.message || 'The DNS cache has been cleared',
    color: 'success',
    icon: 'i-lucide-check-circle'
  })
}

watch(
  () => cacheState.value.excludes,
  (excludes) => {
    excludesState.excludes = excludes.join('\n')
  },
  { immediate: true }
)

onMounted(() => {
  refresh()
})
</script>

<template>
  <UDashboardPanel>
    <template #header>
      <UDashboardNavbar title="Cache">
        <template #right>
          <div class="flex items-center gap-2">
            <UBadge
              :label="cacheState.enabled ? 'Active' : 'Inactive'"
              :color="cacheState.enabled ? 'success' : 'neutral'"
              variant="subtle"
            />
            <UButton
              icon="i-lucide-refresh-cw"
              color="neutral"
              variant="ghost"
              :loading="refreshing"
              @click="refresh(true)"
            />
          </div>
        </template>
      </UDashboardNavbar>
    </template>

    <template #body>
      <div class="space-y-6 p-6">
        <div v-if="!initialized && refreshing" class="space-y-4">
          <USkeleton class="h-28 w-full" />
          <USkeleton class="h-56 w-full" />
        </div>

        <template v-else>
          <div class="grid grid-cols-1 gap-4 xl:grid-cols-3">
            <UCard class="xl:col-span-2">
              <div class="flex items-start justify-between gap-4">
                <div class="space-y-2">
                  <p class="text-sm text-muted">Middleware status</p>
                  <h3 class="text-lg font-semibold">
                    {{ cacheState.enabled ? 'DNS cache is active' : 'DNS cache is disabled' }}
                  </h3>
                  <p class="text-sm text-muted">
                    {{ cacheState.excludes.length }} persisted exclusion entr{{ cacheState.excludes.length === 1 ? 'y' : 'ies' }}
                  </p>
                </div>
                <USwitch
                  :model-value="cacheState.enabled"
                  :loading="toggleLoading"
                  @update:model-value="handleToggle"
                />
              </div>
            </UCard>

            <UCard>
              <div class="space-y-2">
                <p class="text-sm text-muted">Runtime counters</p>
                <div class="space-y-1 text-sm">
                  <p><span class="text-muted">Hits:</span> {{ cacheState.hits.toLocaleString() }}</p>
                  <p><span class="text-muted">Misses:</span> {{ cacheState.misses.toLocaleString() }}</p>
                  <p><span class="text-muted">TTL:</span> {{ cacheState.ttl }} minute<span v-if="cacheState.ttl !== 1">s</span></p>
                </div>
              </div>
            </UCard>
          </div>

          <div class="grid grid-cols-1 gap-6 xl:grid-cols-2">
            <UCard>
              <template #header>
                <div>
                  <h3 class="font-semibold">Cache Exclusions</h3>
                  <p class="text-sm text-muted">Persist selectors that must bypass cache lookup and cache writes.</p>
                </div>
              </template>

              <UForm
                :schema="excludesSchema"
                :state="excludesState"
                class="space-y-4"
                @submit="handleReplaceExcludes"
              >
                <UFormField
                  name="excludes"
                  label="Exclude selectors"
                  description="One selector per line. Supported forms: label:family, ip:192.168.1.20, cidr:10.0.0.0/24"
                >
                  <UTextarea
                    v-model="excludesState.excludes"
                    :rows="8"
                    placeholder="label:kids&#10;ip:192.168.1.20&#10;cidr:10.0.0.0/24"
                  />
                </UFormField>

                <div class="flex justify-end">
                  <UButton
                    type="submit"
                    icon="i-lucide-save"
                    label="Save Exclusions"
                    :loading="excludesLoading"
                  />
                </div>
              </UForm>
            </UCard>

            <UCard>
              <template #header>
                <div>
                  <h3 class="font-semibold">Current Exclusions</h3>
                  <p class="text-sm text-muted">These entries are persisted in the overrides database.</p>
                </div>
              </template>

              <div v-if="cacheState.excludes.length === 0" class="py-6">
                <UEmpty
                  icon="i-lucide-database-zap"
                  title="No cache exclusions"
                  description="All requests are currently eligible for caching unless they miss the cache key."
                />
              </div>

              <div v-else class="space-y-2">
                <div
                  v-for="entry in cacheState.excludes"
                  :key="entry"
                  class="flex items-center justify-between rounded-lg bg-muted/40 p-3"
                >
                  <span class="font-mono text-sm">{{ entry }}</span>
                  <UBadge label="Persisted" color="primary" variant="subtle" />
                </div>
              </div>

              <USeparator class="my-6" />

              <div class="flex justify-end">
                <UButton
                  icon="i-lucide-trash-2"
                  label="Clear Cache Entries"
                  color="warning"
                  variant="outline"
                  :loading="clearLoading"
                  @click="handleClearCache"
                />
              </div>
            </UCard>
          </div>

          <UAlert
            icon="i-lucide-info"
            color="info"
            title="About Cache Persistence"
            description="Changes made here are persisted in the overrides database and applied on startup. They do not rewrite the YAML file. Use this page for operator-managed cache overrides without editing configuration files."
          />
        </template>
      </div>
    </template>
  </UDashboardPanel>
</template>
