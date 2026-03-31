<script setup lang="ts">
import { z } from 'zod'
import type { FormSubmitEvent } from '@nuxt/ui'

definePageMeta({ layout: 'dashboard', middleware: 'auth' })

const { toggleBlacklist, replaceZenDomains } = useApi()
const toast = useToast()

const blacklistEnabled = ref(false)
const toggleLoading = ref(false)

// Domains management
const showDomainsModal = ref(false)
const domainsSchema = z.object({
  domains: z.string().min(1, 'At least one domain is required')
})
const domainsState = reactive({ domains: '' })
const domainsLoading = ref(false)

// Local blacklist (simulated - would normally come from API)
const localDomains = ref<string[]>([
  'ads.example.com',
  'tracking.example.net',
  'malware.example.org'
])

const searchQuery = ref('')

const filteredDomains = computed(() => {
  if (!searchQuery.value) return localDomains.value
  return localDomains.value.filter(d => 
    d.toLowerCase().includes(searchQuery.value.toLowerCase())
  )
})

async function handleToggleBlacklist() {
  toggleLoading.value = true
  const action = blacklistEnabled.value ? 'stop' : 'start'
  const response = await toggleBlacklist(action)
  if (response) {
    blacklistEnabled.value = !blacklistEnabled.value
    toast.add({
      title: `Blacklist ${blacklistEnabled.value ? 'enabled' : 'disabled'}`,
      description: response.message || `Blacklist filtering is now ${blacklistEnabled.value ? 'active' : 'inactive'}`,
      color: 'success',
      icon: 'i-lucide-check-circle'
    })
  }
  toggleLoading.value = false
}

async function handleUpdateDomains(event: FormSubmitEvent<z.output<typeof domainsSchema>>) {
  domainsLoading.value = true
  const domains = event.data.domains.split('\n').map(d => d.trim()).filter(d => d)
  
  const response = await replaceZenDomains(domains)
  if (response) {
    localDomains.value = domains
    toast.add({
      title: 'Domains updated',
      description: `${domains.length} domains have been configured`,
      color: 'success',
      icon: 'i-lucide-check-circle'
    })
    showDomainsModal.value = false
  }
  domainsLoading.value = false
}

function openDomainsModal() {
  domainsState.domains = localDomains.value.join('\n')
  showDomainsModal.value = true
}

function removeDomain(domain: string) {
  localDomains.value = localDomains.value.filter(d => d !== domain)
  toast.add({
    title: 'Domain removed',
    description: `"${domain}" has been removed from the list`,
    color: 'success',
    icon: 'i-lucide-check-circle'
  })
}
</script>

<template>
  <UDashboardPanel>
    <template #header>
      <UDashboardNavbar title="Blacklist Management">
        <template #right>
          <div class="flex items-center gap-2">
            <UButton
              icon="i-lucide-edit"
              label="Edit Domains"
              variant="outline"
              color="neutral"
              @click="openDomainsModal"
            />
            <UButton
              :icon="blacklistEnabled ? 'i-lucide-shield-off' : 'i-lucide-shield'"
              :label="blacklistEnabled ? 'Disable' : 'Enable'"
              :color="blacklistEnabled ? 'error' : 'primary'"
              :loading="toggleLoading"
              @click="handleToggleBlacklist"
            />
          </div>
        </template>
      </UDashboardNavbar>

      <UDashboardToolbar>
        <template #left>
          <UInput
            v-model="searchQuery"
            icon="i-lucide-search"
            placeholder="Search domains..."
            class="w-64"
          />
        </template>
        <template #right>
          <UBadge
            :label="`${localDomains.length} domains`"
            color="neutral"
            variant="subtle"
          />
        </template>
      </UDashboardToolbar>
    </template>

    <template #body>
      <div class="p-6 space-y-6">
        <!-- Status Card -->
        <UCard>
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-4">
              <div
                class="p-4 rounded-full"
                :class="blacklistEnabled ? 'bg-success/10' : 'bg-muted'"
              >
                <UIcon
                  :name="blacklistEnabled ? 'i-lucide-shield-check' : 'i-lucide-shield-off'"
                  class="size-8"
                  :class="blacklistEnabled ? 'text-success' : 'text-muted'"
                />
              </div>
              <div>
                <h3 class="text-lg font-semibold">
                  Blacklist Filtering {{ blacklistEnabled ? 'Active' : 'Inactive' }}
                </h3>
                <p class="text-muted">
                  {{ blacklistEnabled 
                    ? 'DNS queries to blacklisted domains are being blocked' 
                    : 'All DNS queries are being resolved normally' 
                  }}
                </p>
              </div>
            </div>
            <USwitch
              :model-value="blacklistEnabled"
              :loading="toggleLoading"
              @update:model-value="handleToggleBlacklist"
            />
          </div>
        </UCard>

        <!-- Domains List -->
        <UCard>
          <template #header>
            <div class="flex items-center justify-between">
              <h3 class="font-semibold">Blacklisted Domains</h3>
              <UButton
                icon="i-lucide-plus"
                label="Add Domain"
                size="sm"
                @click="openDomainsModal"
              />
            </div>
          </template>

          <div v-if="filteredDomains.length === 0" class="py-8">
            <UEmpty
              icon="i-lucide-shield-ban"
              title="No domains found"
              :description="searchQuery ? 'No domains match your search' : 'Add domains to the blacklist'"
            >
              <UButton
                v-if="!searchQuery"
                icon="i-lucide-plus"
                label="Add Domains"
                @click="openDomainsModal"
              />
            </UEmpty>
          </div>

          <div v-else class="space-y-2">
            <div
              v-for="domain in filteredDomains"
              :key="domain"
              class="flex items-center justify-between p-3 rounded-lg bg-muted/50 hover:bg-muted transition-colors"
            >
              <div class="flex items-center gap-3">
                <UIcon name="i-lucide-ban" class="size-4 text-error" />
                <span class="font-mono">{{ domain }}</span>
              </div>
              <UButton
                icon="i-lucide-trash-2"
                variant="ghost"
                size="sm"
                color="error"
                @click="removeDomain(domain)"
              />
            </div>
          </div>
        </UCard>

        <!-- Info -->
        <UAlert
          icon="i-lucide-info"
          color="info"
          title="About Blacklisting"
          description="Blacklisted domains will return NXDOMAIN or be redirected based on your configuration. Use wildcards like *.example.com to block all subdomains."
        />
      </div>
    </template>
  </UDashboardPanel>

  <!-- Edit Domains Modal -->
  <UModal v-model:open="showDomainsModal" title="Edit Blacklist" description="Enter one domain per line">
    <template #body>
      <UForm :schema="domainsSchema" :state="domainsState" class="space-y-4" @submit="handleUpdateDomains">
        <UFormField name="domains" label="Domains" description="Wildcards supported (e.g., *.example.com)">
          <UTextarea
            v-model="domainsState.domains"
            placeholder="ads.example.com&#10;*.tracking.net&#10;malware.example.org"
            :rows="10"
          />
        </UFormField>
        <div class="flex justify-end gap-2">
          <UButton variant="ghost" label="Cancel" @click="showDomainsModal = false" />
          <UButton type="submit" label="Save Changes" :loading="domainsLoading" />
        </div>
      </UForm>
    </template>
  </UModal>
</template>
