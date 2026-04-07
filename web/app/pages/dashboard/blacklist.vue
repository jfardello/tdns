<script setup lang="ts">
import { z } from 'zod'
import type { FormSubmitEvent } from '@nuxt/ui'

definePageMeta({ layout: 'dashboard', middleware: 'auth' })

const toast = useToast()
const {
  blacklist,
  initialized,
  refreshing,
  toggleLoading,
  persistedExcludesLoading,
  persistedHostsLoading,
  runtimeWhitelistLoading,
  refresh,
  setEnabled,
  replacePersistedExcludes,
  replacePersistedHosts,
  addRuntimeWhitelistEntries
} = useBlacklist()

const runtimeWhitelistSchema = z.object({
  domains: z.string().min(1, 'At least one domain is required')
})

const runtimeWhitelistState = reactive({
  domains: ''
})

const persistedExcludesState = reactive({
  excludes: ''
})

const persistedHostsState = reactive({
  hosts: ''
})

const sourceLabel = computed(() => (
  blacklist.value.external_repo ? 'Remote GitHub source' : 'Local blockfile'
))

const statusLabel = computed(() => (
  blacklist.value.enabled ? 'Blacklist filtering is active' : 'Blacklist filtering is paused'
))

const persistedExcludes = computed(() => blacklist.value.excludes)
const persistedOverrideExcludes = computed(() => blacklist.value.persisted_excludes)
const persistedHosts = computed(() => blacklist.value.persisted_hosts)
const runtimeWhitelist = computed(() => blacklist.value.runtime_whitelist)

const configItems = computed(() => [
  { label: 'Blockfile', value: blacklist.value.file || 'Not configured' },
  { label: 'Source mode', value: sourceLabel.value },
  { label: 'External repo', value: blacklist.value.external_repo || 'Not configured' },
  { label: 'External file', value: blacklist.value.external_file || 'Not configured' },
  { label: 'External branch', value: blacklist.value.external_repo_branch || 'Not configured' },
  { label: 'Pull schedule', value: blacklist.value.external_pull_period || 'Not configured' }
])

function splitDomains(value: string) {
  return value
    .split(/[\n,]/)
    .map(domain => domain.trim())
    .filter(Boolean)
}

async function handleToggleBlacklist(nextEnabled: boolean) {
  const response = await setEnabled(nextEnabled)
  if (!response) {
    return
  }

  toast.add({
    title: `Blacklist ${nextEnabled ? 'enabled' : 'paused'}`,
    description: response.message || `Blacklist filtering is now ${nextEnabled ? 'active' : 'inactive'}`,
    color: 'success',
    icon: 'i-lucide-check-circle'
  })
}

async function handleAddRuntimeWhitelist(event: FormSubmitEvent<z.output<typeof runtimeWhitelistSchema>>) {
  const domains = splitDomains(event.data.domains)
  const response = await addRuntimeWhitelistEntries(domains)
  if (!response) {
    return
  }

  runtimeWhitelistState.domains = ''
  toast.add({
    title: 'Runtime whitelist updated',
    description: `${domains.length} suffix${domains.length === 1 ? '' : 'es'} added for the running process`,
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
    title: 'Persisted whitelist updated',
    description: `${excludes.length} persisted whitelist selector${excludes.length === 1 ? '' : 's'} stored`,
    color: 'success',
    icon: 'i-lucide-check-circle'
  })
}

async function handleReplacePersistedHosts() {
  const hosts = splitDomains(persistedHostsState.hosts)
  const response = await replacePersistedHosts(hosts)
  if (!response) {
    return
  }

  persistedHostsState.hosts = ''
  toast.add({
    title: 'Persisted blocked hosts updated',
    description: `${hosts.length} extra blocked host${hosts.length === 1 ? '' : 's'} stored`,
    color: 'success',
    icon: 'i-lucide-check-circle'
  })
}

onMounted(() => {
  refresh()
})
</script>

<template>
  <UDashboardPanel>
    <template #header>
      <UDashboardNavbar title="Blacklist Management">
        <template #right>
          <div class="flex items-center gap-2">
            <UBadge
              :label="`${blacklist.blockfile_total_entries.toLocaleString()} entries`"
              color="neutral"
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
                  <div class="flex items-center gap-3">
                    <div
                      class="flex size-11 items-center justify-center rounded-full"
                      :class="blacklist.enabled ? 'bg-success/10' : 'bg-muted'"
                    >
                      <UIcon
                        :name="blacklist.enabled ? 'i-lucide-shield-check' : 'i-lucide-pause-circle'"
                        class="size-5"
                        :class="blacklist.enabled ? 'text-success' : 'text-muted'"
                      />
                    </div>
                    <div>
                      <p class="text-sm text-muted">Blocking status</p>
                      <h3 class="text-lg font-semibold">{{ statusLabel }}</h3>
                    </div>
                  </div>
                  <p class="text-sm text-muted">
                    Toggle blacklist filtering for the running process without editing the YAML file.
                  </p>
                </div>
                <USwitch
                  :model-value="blacklist.enabled"
                  :loading="toggleLoading"
                  @update:model-value="handleToggleBlacklist"
                />
              </div>
            </UCard>

            <UCard>
              <div class="space-y-2">
                <p class="text-sm text-muted">Total entries in blockfile</p>
                <p class="text-3xl font-semibold">{{ blacklist.blockfile_total_entries.toLocaleString() }}</p>
                <p class="text-sm text-muted">
                  Raw parsed entries from <span class="font-mono">{{ blacklist.file || 'the configured blockfile' }}</span>.
                </p>
              </div>
            </UCard>
          </div>

          <UCard>
            <template #header>
              <div class="flex items-center justify-between gap-3">
                <div>
                  <h3 class="font-semibold">Blacklist Configuration</h3>
                  <p class="text-sm text-muted">Current persisted configuration loaded from the running server.</p>
                </div>
                <UBadge :label="sourceLabel" color="primary" variant="subtle" />
              </div>
            </template>

            <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
              <div
                v-for="item in configItems"
                :key="item.label"
                class="rounded-lg border border-default bg-muted/30 p-4"
              >
                <p class="text-xs uppercase tracking-[0.16em] text-muted">{{ item.label }}</p>
                <p class="mt-2 break-all text-sm font-medium">{{ item.value }}</p>
              </div>
            </div>
          </UCard>

          <div class="grid grid-cols-1 gap-6 xl:grid-cols-2">
            <UCard>
              <template #header>
                <div>
                  <h3 class="font-semibold">Persisted YAML Whitelist</h3>
                  <p class="text-sm text-muted">Entries from <span class="font-mono">blacklist.excludes</span>.</p>
                </div>
              </template>

              <div v-if="persistedExcludes.length === 0" class="py-8">
                <UEmpty
                  icon="i-lucide-file-text"
                  title="No persisted whitelist entries"
                  description="Add suffixes to blacklist.excludes in your YAML configuration to make them permanent."
                />
              </div>

              <div v-else class="flex flex-wrap gap-2">
                <UBadge
                  v-for="entry in persistedExcludes"
                  :key="entry"
                  :label="entry"
                  color="neutral"
                  variant="subtle"
                />
              </div>
            </UCard>

            <UCard>
              <template #header>
                <div>
                  <h3 class="font-semibold">Persisted Whitelist Overrides</h3>
                  <p class="text-sm text-muted">Selectors stored in the tdns overrides database.</p>
                </div>
              </template>

              <div class="space-y-4">
                <UFormField
                  name="excludes"
                  label="Persisted whitelist selectors"
                  description="Enter one domain suffix or label: selector per line."
                >
                  <UTextarea
                    v-model="persistedExcludesState.excludes"
                    :rows="6"
                    placeholder="example.com&#10;label:trusted"
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
                <div v-if="persistedOverrideExcludes.length === 0" class="py-6">
                  <UEmpty
                    icon="i-lucide-database-zap"
                    title="No persisted whitelist overrides"
                    description="Store selectors here to keep them across restarts without editing YAML."
                  />
                </div>
                <div v-else class="flex flex-wrap gap-2">
                  <UBadge
                    v-for="entry in persistedOverrideExcludes"
                    :key="`persisted-${entry}`"
                    :label="entry"
                    color="primary"
                    variant="subtle"
                  />
                </div>
              </div>
            </UCard>
          </div>

          <div class="grid grid-cols-1 gap-6 xl:grid-cols-2">
            <UCard>
              <template #header>
                <div>
                  <h3 class="font-semibold">Persisted Blocked Hosts</h3>
                  <p class="text-sm text-muted">Extra blocked suffixes stored outside the blockfile.</p>
                </div>
              </template>

              <div class="space-y-4">
                <UFormField
                  name="hosts"
                  label="Persisted blocked suffixes"
                  description="Enter one domain suffix per line."
                >
                  <UTextarea
                    v-model="persistedHostsState.hosts"
                    :rows="6"
                    placeholder="ads.example.com&#10;tracker.example.net"
                  />
                </UFormField>

                <div class="flex justify-end">
                  <UButton
                    icon="i-lucide-database"
                    label="Replace Persisted Blocked Hosts"
                    :loading="persistedHostsLoading"
                    @click="handleReplacePersistedHosts"
                  />
                </div>
              </div>

              <div class="mt-6">
                <div v-if="persistedHosts.length === 0" class="py-6">
                  <UEmpty
                    icon="i-lucide-ban"
                    title="No persisted blocked hosts"
                    description="Store extra blocked suffixes here to keep them active across restarts."
                  />
                </div>
                <div v-else class="flex flex-wrap gap-2">
                  <UBadge
                    v-for="entry in persistedHosts"
                    :key="entry"
                    :label="entry"
                    color="error"
                    variant="subtle"
                  />
                </div>
              </div>
            </UCard>

            <UCard>
              <template #header>
                <div>
                  <h3 class="font-semibold">Runtime Whitelist</h3>
                  <p class="text-sm text-muted">Add suffixes that should bypass blocking for the current process only.</p>
                </div>
              </template>

              <UForm
                :schema="runtimeWhitelistSchema"
                :state="runtimeWhitelistState"
                class="space-y-4"
                @submit="handleAddRuntimeWhitelist"
              >
                <UFormField
                  name="domains"
                  label="Whitelist suffixes"
                  description="Enter one suffix per line. Matching is suffix-based, just like blacklist.excludes."
                >
                  <UTextarea
                    v-model="runtimeWhitelistState.domains"
                    :rows="6"
                    placeholder="example.com&#10;allowed.internal&#10;tracking.net"
                  />
                </UFormField>

                <div class="flex justify-end">
                  <UButton
                    type="submit"
                    icon="i-lucide-plus"
                    label="Add Runtime Values"
                    :loading="runtimeWhitelistLoading"
                  />
                </div>
              </UForm>

              <div class="mt-6 space-y-3">
                <div class="flex items-center justify-between">
                  <p class="text-sm font-medium">Current runtime entries</p>
                  <UBadge
                    :label="`${runtimeWhitelist.length}`"
                    color="neutral"
                    variant="subtle"
                  />
                </div>

                <div v-if="runtimeWhitelist.length === 0" class="py-6">
                  <UEmpty
                    icon="i-lucide-list-plus"
                    title="No runtime whitelist entries"
                    description="Add a suffix above to bypass blacklist blocking without changing the YAML file."
                  />
                </div>

                <div v-else class="flex flex-wrap gap-2">
                  <UBadge
                    v-for="entry in runtimeWhitelist"
                    :key="entry"
                    :label="entry"
                    color="success"
                    variant="subtle"
                  />
                </div>
              </div>
            </UCard>
          </div>

          <UAlert
            icon="i-lucide-info"
            color="warning"
            title="About Blacklisting"
            description="blacklist.excludes still defines the base YAML whitelist. Persisted overrides stored here survive restarts, but they are not written back to YAML. Runtime whitelist entries still affect only the running process."
          />
        </template>
      </div>
    </template>
  </UDashboardPanel>
</template>
