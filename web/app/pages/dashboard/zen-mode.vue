<script setup lang="ts">
import { z } from 'zod'
import type { FormSubmitEvent } from '@nuxt/ui'

definePageMeta({ layout: 'dashboard', middleware: 'auth' })

const toast = useToast()
const {
  zenMode,
  initialized,
  refreshing,
  startLoading,
  persistedDomainsLoading,
  persistedExcludesLoading,
  runtimeDomainsLoading,
  refresh,
  startSession,
  replacePersistedDomains,
  replacePersistedExcludes,
  replaceRuntimeDomains
} = useZenMode()

const runtimeDomainsSchema = z.object({
  domains: z.string().min(1, 'At least one domain is required')
})

const runtimeDomainsState = reactive({
  domains: ''
})

const persistedDomainsState = reactive({
  domains: ''
})

const persistedExcludesState = reactive({
  excludes: ''
})

const now = ref(Date.now())
let timer: ReturnType<typeof setInterval> | undefined

const remainingLabel = computed(() => {
  if (!zenMode.value.ends_at) {
    return 'Not running'
  }

  const endsAt = new Date(zenMode.value.ends_at).getTime()
  const remainingMs = Math.max(0, endsAt - now.value)
  const totalSeconds = Math.floor(remainingMs / 1000)
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  return `${minutes}m ${String(seconds).padStart(2, '0')}s`
})

function splitDomains(value: string) {
  return value
    .split(/[\n,]/)
    .map(domain => domain.trim())
    .filter(Boolean)
}

async function handleStart() {
  const response = await startSession()
  if (!response) {
    return
  }

  toast.add({
    title: 'Zen Mode started',
    description: response.message || `Focus session started for ${zenMode.value.duration_minutes} minutes`,
    color: 'success',
    icon: 'i-lucide-check-circle'
  })
}

async function handleReplaceDomains(event: FormSubmitEvent<z.output<typeof runtimeDomainsSchema>>) {
  const domains = splitDomains(event.data.domains)
  const response = await replaceRuntimeDomains(domains)
  if (!response) {
    return
  }

  runtimeDomainsState.domains = ''
  toast.add({
    title: 'Runtime domains updated',
    description: `${domains.length} runtime domain pattern${domains.length === 1 ? '' : 's'} loaded for Zen Mode`,
    color: 'success',
    icon: 'i-lucide-check-circle'
  })
}

async function handleReplacePersistedDomains(event: FormSubmitEvent<z.output<typeof runtimeDomainsSchema>>) {
  const domains = splitDomains(event.data.domains)
  const response = await replacePersistedDomains(domains)
  if (!response) {
    return
  }

  persistedDomainsState.domains = ''
  toast.add({
    title: 'Persisted domains updated',
    description: `${domains.length} persisted domain pattern${domains.length === 1 ? '' : 's'} stored`,
    color: 'success',
    icon: 'i-lucide-check-circle'
  })
}

async function handleReplacePersistedExcludes() {
  const excludes = splitDomains(persistedExcludesState.excludes)
  const response = await replacePersistedExcludes(excludes)
  if (!response) {
    return
  }

  persistedExcludesState.excludes = ''
  toast.add({
    title: 'Persisted Zen whitelist updated',
    description: `${excludes.length} persisted whitelist selector${excludes.length === 1 ? '' : 's'} stored`,
    color: 'success',
    icon: 'i-lucide-check-circle'
  })
}

onMounted(() => {
  refresh()
  timer = setInterval(() => {
    now.value = Date.now()
  }, 1000)
})

onUnmounted(() => {
  if (timer) {
    clearInterval(timer)
  }
})
</script>

<template>
  <UDashboardPanel>
    <template #header>
      <UDashboardNavbar title="Zen Mode">
        <template #right>
          <div class="flex items-center gap-2">
            <UBadge
              :label="zenMode.enabled ? 'Session running' : 'Idle'"
              :color="zenMode.enabled ? 'success' : 'neutral'"
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
          <USkeleton class="h-48 w-full" />
          <USkeleton class="h-56 w-full" />
        </div>

        <template v-else>
          <div class="grid grid-cols-1 gap-4 xl:grid-cols-3">
            <UCard class="xl:col-span-2">
              <div class="flex items-start justify-between gap-4">
                <div class="space-y-2">
                  <p class="text-sm text-muted">Current session</p>
                  <h3 class="text-lg font-semibold">
                    {{ zenMode.enabled ? 'Zen Mode is active' : 'Zen Mode is waiting to start' }}
                  </h3>
                  <p class="text-sm text-muted">
                    Remaining time: {{ remainingLabel }}
                  </p>
                  <p class="text-sm text-muted">
                    Configured focus window: {{ zenMode.duration_minutes }} minutes
                  </p>
                </div>
                <UButton
                  icon="i-lucide-play"
                  label="Start Session"
                  :disabled="zenMode.enabled"
                  :loading="startLoading"
                  @click="handleStart"
                />
              </div>
            </UCard>

            <UCard>
              <div class="space-y-2">
                <p class="text-sm text-muted">Session timing</p>
                <p class="text-sm font-medium">Started: {{ zenMode.started_at || 'Not started' }}</p>
                <p class="text-sm font-medium">Ends: {{ zenMode.ends_at || 'Not scheduled' }}</p>
              </div>
            </UCard>
          </div>

          <UCard>
            <template #header>
              <div>
                <h3 class="font-semibold">Zen Mode Configuration</h3>
                <p class="text-sm text-muted">Persisted settings loaded from the running server configuration.</p>
              </div>
            </template>

            <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
              <div class="rounded-lg border border-default bg-muted/30 p-4">
                <p class="text-xs uppercase tracking-[0.16em] text-muted">Source file</p>
                <p class="mt-2 break-all text-sm font-medium">{{ zenMode.file || 'Not configured' }}</p>
              </div>
              <div class="rounded-lg border border-default bg-muted/30 p-4">
                <p class="text-xs uppercase tracking-[0.16em] text-muted">Configured duration</p>
                <p class="mt-2 text-sm font-medium">{{ zenMode.duration_minutes }} minutes</p>
              </div>
            </div>

            <div class="mt-6">
              <p class="text-sm font-medium">Configured domains</p>
              <div v-if="zenMode.configured_domains.length === 0" class="py-6">
                <UEmpty
                  icon="i-lucide-file-text"
                  title="No configured domains"
                  description="Add zen_mode.domains or zen_mode.file entries in the YAML configuration to make them permanent."
                />
              </div>
              <div v-else class="mt-3 flex flex-wrap gap-2">
                <UBadge
                  v-for="domain in zenMode.configured_domains"
                  :key="domain"
                  :label="domain"
                  color="neutral"
                  variant="subtle"
                />
              </div>
            </div>

            <div class="mt-6">
              <p class="text-sm font-medium">Configured whitelist selectors</p>
              <div v-if="zenMode.configured_excludes.length === 0" class="py-6">
                <UEmpty
                  icon="i-lucide-shield-off"
                  title="No configured whitelist selectors"
                  description="Add zen_mode.excludes in YAML to define permanent base exclusions."
                />
              </div>
              <div v-else class="mt-3 flex flex-wrap gap-2">
                <UBadge
                  v-for="entry in zenMode.configured_excludes"
                  :key="entry"
                  :label="entry"
                  color="neutral"
                  variant="subtle"
                />
              </div>
            </div>
          </UCard>

          <div class="grid grid-cols-1 gap-6 xl:grid-cols-2">
            <UCard>
              <template #header>
                <div>
                  <h3 class="font-semibold">Persisted Domains</h3>
                  <p class="text-sm text-muted">Stored in the tdns overrides database and loaded on startup.</p>
                </div>
              </template>

              <UForm
                :schema="runtimeDomainsSchema"
                :state="persistedDomainsState"
                class="space-y-4"
                @submit="handleReplacePersistedDomains"
              >
                <UFormField
                  name="domains"
                  label="Persisted domain patterns"
                  description="Enter one domain or regex-style pattern per line."
                >
                  <UTextarea
                    v-model="persistedDomainsState.domains"
                    :rows="6"
                    placeholder="x.com&#10;www.facebook.com&#10;.*instagram.com"
                  />
                </UFormField>

                <div class="flex justify-end">
                  <UButton
                    type="submit"
                    icon="i-lucide-database"
                    label="Replace Persisted Domains"
                    :loading="persistedDomainsLoading"
                  />
                </div>
              </UForm>

              <div class="mt-6">
                <p class="text-sm font-medium">Persisted domains</p>
                <div v-if="zenMode.persisted_domains.length === 0" class="py-6">
                  <UEmpty
                    icon="i-lucide-database-zap"
                    title="No persisted domains"
                    description="Store domains here to keep them across restarts without editing YAML."
                  />
                </div>
                <div v-else class="mt-3 flex flex-wrap gap-2">
                  <UBadge
                    v-for="domain in zenMode.persisted_domains"
                    :key="domain"
                    :label="domain"
                    color="primary"
                    variant="subtle"
                  />
                </div>
              </div>
            </UCard>

            <UCard>
              <template #header>
                <div>
                  <h3 class="font-semibold">Persisted Zen Whitelist</h3>
                  <p class="text-sm text-muted">Selectors that bypass Zen Mode and survive restarts.</p>
                </div>
              </template>

              <div class="space-y-4">
                <UFormField
                  name="excludes"
                  label="Persisted whitelist selectors"
                  description="Use domains or label: selectors, one value per line."
                >
                  <UTextarea
                    v-model="persistedExcludesState.excludes"
                    :rows="6"
                    placeholder="example.com&#10;label:nozen"
                  />
                </UFormField>

                <div class="flex justify-end">
                  <UButton
                    icon="i-lucide-database"
                    label="Replace Persisted Whitelist"
                    :loading="persistedExcludesLoading"
                    @click="handleReplacePersistedExcludes"
                  />
                </div>
              </div>

              <div class="mt-6">
                <p class="text-sm font-medium">Persisted whitelist selectors</p>
                <div v-if="zenMode.persisted_excludes.length === 0" class="py-6">
                  <UEmpty
                    icon="i-lucide-list-x"
                    title="No persisted whitelist selectors"
                    description="Store domains or labels here to keep them outside Zen Mode after restart."
                  />
                </div>
                <div v-else class="mt-3 flex flex-wrap gap-2">
                  <UBadge
                    v-for="entry in zenMode.persisted_excludes"
                    :key="entry"
                    :label="entry"
                    color="primary"
                    variant="subtle"
                  />
                </div>
              </div>
            </UCard>
          </div>

          <UCard>
            <template #header>
              <div>
                <h3 class="font-semibold">Runtime Domains</h3>
                <p class="text-sm text-muted">Replace the active Zen Mode domain patterns for the current process only.</p>
              </div>
            </template>

            <UForm
              :schema="runtimeDomainsSchema"
              :state="runtimeDomainsState"
              class="space-y-4"
              @submit="handleReplaceDomains"
            >
              <UFormField
                name="domains"
                label="Runtime domain patterns"
                description="Enter one domain or regex-style pattern per line."
              >
                <UTextarea
                  v-model="runtimeDomainsState.domains"
                  :rows="6"
                  placeholder="x.com&#10;www.facebook.com&#10;.*instagram.com"
                />
              </UFormField>

              <div class="flex justify-end">
                <UButton
                  type="submit"
                  icon="i-lucide-save"
                  label="Replace Runtime Domains"
                  :loading="runtimeDomainsLoading"
                />
              </div>
            </UForm>

            <div class="mt-6">
              <p class="text-sm font-medium">Current runtime domains</p>
              <div v-if="zenMode.runtime_domains.length === 0" class="py-6">
                <UEmpty
                  icon="i-lucide-list-plus"
                  title="No runtime domains"
                  description="Set runtime domain patterns above to change the active Zen Mode match list."
                />
              </div>
              <div v-else class="mt-3 flex flex-wrap gap-2">
                <UBadge
                  v-for="domain in zenMode.runtime_domains"
                  :key="domain"
                  :label="domain"
                  color="primary"
                  variant="subtle"
                />
              </div>
            </div>
          </UCard>

          <UAlert
            icon="i-lucide-info"
            color="warning"
            title="About Zen Mode"
            description="Configured domains and excludes still come from zen_mode.file, zen_mode.domains, and zen_mode.excludes. Persisted overrides stored here survive restarts, but they are not written back to YAML. Runtime domain replacements still affect only the running process."
          />
        </template>
      </div>
    </template>
  </UDashboardPanel>
</template>
