<script setup lang="ts">
import { z } from 'zod'
import type { FormSubmitEvent } from '@nuxt/ui'

definePageMeta({ layout: 'dashboard', middleware: 'auth' })

const toast = useToast()
const {
  staticResponse,
  initialized,
  refreshing,
  toggleLoading,
  runtimeHostsLoading,
  refresh,
  setEnabled,
  replaceRuntimeHosts
} = useStaticResponse()

const runtimeHostsSchema = z.object({
  hosts: z.string().min(1, 'At least one host entry is required')
})

const runtimeHostsState = reactive({
  hosts: ''
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
    title: `Static Response ${nextEnabled ? 'enabled' : 'disabled'}`,
    description: response.message || `Static host answers are now ${nextEnabled ? 'active' : 'inactive'}`,
    color: 'success',
    icon: 'i-lucide-check-circle'
  })
}

async function handleReplaceHosts(event: FormSubmitEvent<z.output<typeof runtimeHostsSchema>>) {
  const hosts = splitLines(event.data.hosts)
  const response = await replaceRuntimeHosts(hosts)
  if (!response) {
    return
  }

  runtimeHostsState.hosts = ''
  toast.add({
    title: 'Runtime hosts updated',
    description: `${hosts.length} runtime host entr${hosts.length === 1 ? 'y' : 'ies'} loaded`,
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
      <UDashboardNavbar title="Static Response">
        <template #right>
          <div class="flex items-center gap-2">
            <UBadge
              :label="staticResponse.enabled ? 'Active' : 'Inactive'"
              :color="staticResponse.enabled ? 'success' : 'neutral'"
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
                  <p class="text-sm text-muted">Middleware status</p>
                  <h3 class="text-lg font-semibold">
                    {{ staticResponse.enabled ? 'Static responses are active' : 'Static responses are disabled' }}
                  </h3>
                  <p class="text-sm text-muted">
                    Runtime host entries: {{ staticResponse.runtime_hosts.length }}
                  </p>
                </div>
                <USwitch
                  :model-value="staticResponse.enabled"
                  :loading="toggleLoading"
                  @update:model-value="handleToggle"
                />
              </div>
            </UCard>

            <UCard>
              <div class="space-y-2">
                <p class="text-sm text-muted">Configured source file</p>
                <p class="break-all text-sm font-medium">{{ staticResponse.file || 'Not configured' }}</p>
              </div>
            </UCard>
          </div>

          <div class="grid grid-cols-1 gap-6 xl:grid-cols-2">
            <UCard>
              <template #header>
                <div>
                  <h3 class="font-semibold">Configured Hosts</h3>
                  <p class="text-sm text-muted">Entries loaded from the configured file.</p>
                </div>
              </template>

              <div v-if="staticResponse.configured_hosts.length === 0" class="py-6">
                <UEmpty
                  icon="i-lucide-file-text"
                  title="No configured hosts"
                  description="Add entries to the configured hosts file to make them permanent."
                />
              </div>

              <div v-else class="space-y-2">
                <div
                  v-for="entry in staticResponse.configured_hosts"
                  :key="`${entry.address}-${entry.domain}`"
                  class="flex items-center justify-between rounded-lg bg-muted/40 p-3"
                >
                  <span class="font-mono text-sm">{{ entry.domain }}</span>
                  <UBadge :label="entry.address" color="neutral" variant="subtle" />
                </div>
              </div>
            </UCard>

            <UCard>
              <template #header>
                <div>
                  <h3 class="font-semibold">Runtime Hosts</h3>
                  <p class="text-sm text-muted">Replace the active host map for the running process only.</p>
                </div>
              </template>

              <UForm
                :schema="runtimeHostsSchema"
                :state="runtimeHostsState"
                class="space-y-4"
                @submit="handleReplaceHosts"
              >
                <UFormField
                  name="hosts"
                  label="Runtime host entries"
                  description="Enter one entry per line using the format: IP_ADDRESS DOMAIN"
                >
                  <UTextarea
                    v-model="runtimeHostsState.hosts"
                    :rows="6"
                    placeholder="0.0.0.0 ads.example.com&#10;10.0.0.2 internal.example.com"
                  />
                </UFormField>

                <div class="flex justify-end">
                  <UButton
                    type="submit"
                    icon="i-lucide-save"
                    label="Replace Runtime Hosts"
                    :loading="runtimeHostsLoading"
                  />
                </div>
              </UForm>

              <div class="mt-6">
                <p class="text-sm font-medium">Current runtime hosts</p>
                <div v-if="staticResponse.runtime_hosts.length === 0" class="py-6">
                  <UEmpty
                    icon="i-lucide-list-plus"
                    title="No runtime hosts"
                    description="Set runtime host entries above to change the active host map."
                  />
                </div>
                <div v-else class="mt-3 space-y-2">
                  <div
                    v-for="entry in staticResponse.runtime_hosts"
                    :key="`runtime-${entry.address}-${entry.domain}`"
                    class="flex items-center justify-between rounded-lg bg-muted/40 p-3"
                  >
                    <span class="font-mono text-sm">{{ entry.domain }}</span>
                    <UBadge :label="entry.address" color="primary" variant="subtle" />
                  </div>
                </div>
              </div>
            </UCard>
          </div>

          <UAlert
            icon="i-lucide-info"
            color="warning"
            title="About Static Response"
            description="Changes made here only affect the running process. Replacing runtime host entries does not persist them to the YAML file or the configured hosts file. To make changes permanent, edit the static_response.file source on disk."
          />
        </template>
      </div>
    </template>
  </UDashboardPanel>
</template>
