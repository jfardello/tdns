<script setup lang="ts">
import { z } from 'zod'
import type { FormSubmitEvent } from '@nuxt/ui'

definePageMeta({ layout: 'dashboard', middleware: 'auth' })

const toast = useToast()
const {
  stubResolver,
  initialized,
  refreshing,
  toggleLoading,
  runtimeStubsLoading,
  refresh,
  setEnabled,
  replaceRuntimeStubs
} = useStubResolver()

const runtimeStubsSchema = z.object({
  stubs: z.string().min(1, 'At least one stub rule is required')
})

const runtimeStubsState = reactive({
  stubs: ''
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
    title: `Stub Resolver ${nextEnabled ? 'enabled' : 'disabled'}`,
    description: response.message || `Stub resolution is now ${nextEnabled ? 'active' : 'inactive'}`,
    color: 'success',
    icon: 'i-lucide-check-circle'
  })
}

async function handleReplaceStubs(event: FormSubmitEvent<z.output<typeof runtimeStubsSchema>>) {
  const stubs = splitLines(event.data.stubs)
  const response = await replaceRuntimeStubs(stubs)
  if (!response) {
    return
  }

  runtimeStubsState.stubs = ''
  toast.add({
    title: 'Runtime stubs updated',
    description: `${stubs.length} runtime stub rule${stubs.length === 1 ? '' : 's'} loaded`,
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
      <UDashboardNavbar title="Stub Resolver">
        <template #right>
          <div class="flex items-center gap-2">
            <UBadge
              :label="stubResolver.enabled ? 'Active' : 'Inactive'"
              :color="stubResolver.enabled ? 'success' : 'neutral'"
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
          <UCard>
            <div class="flex items-start justify-between gap-4">
              <div class="space-y-2">
                <p class="text-sm text-muted">Middleware status</p>
                <h3 class="text-lg font-semibold">
                  {{ stubResolver.enabled ? 'Stub resolution is active' : 'Stub resolution is disabled' }}
                </h3>
                <p class="text-sm text-muted">
                  Runtime stub rules: {{ stubResolver.runtime_stubs.length }}
                </p>
              </div>
              <USwitch
                :model-value="stubResolver.enabled"
                :loading="toggleLoading"
                @update:model-value="handleToggle"
              />
            </div>
          </UCard>

          <div class="grid grid-cols-1 gap-6 xl:grid-cols-2">
            <UCard>
              <template #header>
                <div>
                  <h3 class="font-semibold">Configured Stub Rules</h3>
                  <p class="text-sm text-muted">Rules loaded from the persisted configuration.</p>
                </div>
              </template>

              <div v-if="stubResolver.configured_stubs.length === 0" class="py-6">
                <UEmpty
                  icon="i-lucide-file-text"
                  title="No configured stub rules"
                  description="Add stub_resolver.stubs entries in the YAML file to make them permanent."
                />
              </div>

              <div v-else class="space-y-2">
                <div
                  v-for="entry in stubResolver.configured_stubs"
                  :key="entry"
                  class="rounded-lg bg-muted/40 p-3"
                >
                  <p class="break-all font-mono text-sm">{{ entry }}</p>
                </div>
              </div>
            </UCard>

            <UCard>
              <template #header>
                <div>
                  <h3 class="font-semibold">Runtime Stub Rules</h3>
                  <p class="text-sm text-muted">Replace the active stub rules for the current process only.</p>
                </div>
              </template>

              <UForm
                :schema="runtimeStubsSchema"
                :state="runtimeStubsState"
                class="space-y-4"
                @submit="handleReplaceStubs"
              >
                <UFormField
                  name="stubs"
                  label="Runtime stub rules"
                  description="Use one rule per line, e.g. example.com,udp://8.8.8.8,udp://8.8.4.4"
                >
                  <UTextarea
                    v-model="runtimeStubsState.stubs"
                    :rows="6"
                    placeholder="example.com,udp://8.8.8.8,udp://8.8.4.4&#10;corp.local,tcp://10.0.0.10"
                  />
                </UFormField>

                <div class="flex justify-end">
                  <UButton
                    type="submit"
                    icon="i-lucide-save"
                    label="Replace Runtime Stubs"
                    :loading="runtimeStubsLoading"
                  />
                </div>
              </UForm>

              <div class="mt-6">
                <p class="text-sm font-medium">Current runtime rules</p>
                <div v-if="stubResolver.runtime_stubs.length === 0" class="py-6">
                  <UEmpty
                    icon="i-lucide-list-plus"
                    title="No runtime stub rules"
                    description="Set runtime stub rules above to change the active resolver mapping."
                  />
                </div>
                <div v-else class="mt-3 space-y-2">
                  <div
                    v-for="entry in stubResolver.runtime_stubs"
                    :key="`runtime-${entry}`"
                    class="rounded-lg bg-muted/40 p-3"
                  >
                    <p class="break-all font-mono text-sm">{{ entry }}</p>
                  </div>
                </div>
              </div>
            </UCard>
          </div>

          <UAlert
            icon="i-lucide-info"
            color="warning"
            title="About Stub Resolver"
            description="Changes made here only affect the running process. Replacing runtime stub rules does not persist them to the YAML file. To make changes permanent, edit stub_resolver.stubs in your configuration."
          />
        </template>
      </div>
    </template>
  </UDashboardPanel>
</template>
