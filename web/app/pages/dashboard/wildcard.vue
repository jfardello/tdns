<script setup lang="ts">
import { updateWildcardDomainSelection, wildcardExamples } from '~/lib/wildcardUi'

definePageMeta({ layout: 'dashboard', middleware: 'auth' })

const toast = useToast()
const {
  wildcard,
  initialized,
  refreshing,
  toggleLoading,
  domainsLoading,
  errorMessage,
  refresh,
  setEnabled,
  setDomains
} = useWildcard()

const selectedDomains = ref<string[]>([])
const examples = computed(() => wildcardExamples(wildcard.value.primary_domain))
const domainSelectionChanged = computed(() => (
  wildcard.value.available_extra_domains.some(domain => (
    selectedDomains.value.includes(domain) !== wildcard.value.enabled_extra_domains.includes(domain)
  ))
))

function updateDomain(domain: string, enabled: boolean) {
  selectedDomains.value = updateWildcardDomainSelection(
    wildcard.value.available_extra_domains,
    selectedDomains.value,
    domain,
    enabled
  )
}

async function handleToggle(nextEnabled: boolean) {
  const response = await setEnabled(nextEnabled)
  if (!response) {
    return
  }

  toast.add({
    title: `Wildcard DNS ${nextEnabled ? 'enabled' : 'disabled'}`,
    description: response.message || `Local wildcard resolution is now ${nextEnabled ? 'active' : 'inactive'}`,
    color: 'success',
    icon: 'i-lucide-check-circle'
  })
}

async function handleSaveDomains() {
  const response = await setDomains(selectedDomains.value)
  if (!response) {
    return
  }

  toast.add({
    title: 'Wildcard domains updated',
    description: `${selectedDomains.value.length} additional domain${selectedDomains.value.length === 1 ? '' : 's'} enabled`,
    color: 'success',
    icon: 'i-lucide-check-circle'
  })
}

watch(
  () => wildcard.value.enabled_extra_domains,
  domains => {
    selectedDomains.value = [...domains]
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
      <UDashboardNavbar title="Wildcard DNS">
        <template #right>
          <div class="flex items-center gap-2">
            <UBadge
              :label="wildcard.enabled ? 'Active' : 'Inactive'"
              :color="wildcard.enabled ? 'success' : 'neutral'"
              variant="subtle"
            />
            <UButton
              icon="i-lucide-refresh-cw"
              color="neutral"
              variant="ghost"
              :loading="refreshing"
              aria-label="Refresh wildcard DNS settings"
              @click="refresh(true)"
            />
          </div>
        </template>
      </UDashboardNavbar>
    </template>

    <template #body>
      <div class="space-y-6 p-6">
        <UAlert
          v-if="errorMessage"
          icon="i-lucide-circle-alert"
          color="error"
          title="Wildcard DNS request failed"
          :description="errorMessage"
        />

        <div v-if="!initialized && refreshing" class="space-y-4">
          <USkeleton class="h-32 w-full" />
          <USkeleton class="h-64 w-full" />
          <USkeleton class="h-48 w-full" />
        </div>

        <template v-else>
          <div class="grid grid-cols-1 gap-4 xl:grid-cols-3">
            <UCard class="xl:col-span-2">
              <div class="flex items-start justify-between gap-4">
                <div class="space-y-2">
                  <p class="text-sm text-muted">Middleware status</p>
                  <h3 class="text-lg font-semibold">
                    {{ wildcard.enabled ? 'Local wildcard resolution is active' : 'Local wildcard resolution is disabled' }}
                  </h3>
                  <p class="text-sm text-muted">
                    Matching names are answered directly by TDNS and do not reach an upstream resolver.
                  </p>
                </div>
                <USwitch
                  :model-value="wildcard.enabled"
                  :loading="toggleLoading"
                  aria-label="Enable wildcard DNS"
                  @update:model-value="handleToggle"
                />
              </div>
            </UCard>

            <UCard>
              <div class="space-y-3">
                <p class="text-sm text-muted">Primary local domain</p>
                <p class="break-all font-mono text-sm font-semibold">
                  {{ wildcard.primary_domain || 'tdns.home.arpa' }}
                </p>
                <div class="flex flex-wrap gap-2">
                  <UBadge :label="`TTL ${wildcard.ttl}s`" color="neutral" variant="subtle" />
                  <UBadge
                    :label="wildcard.allow_public_addresses ? 'Public addresses allowed' : 'Local addresses only'"
                    :color="wildcard.allow_public_addresses ? 'warning' : 'success'"
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
                  <h3 class="font-semibold">Hostname Examples</h3>
                  <p class="text-sm text-muted">Embed a local address in the hostname using any supported notation.</p>
                </div>
              </template>

              <div class="space-y-3">
                <div
                  v-for="example in examples"
                  :key="example.label"
                  class="rounded-lg bg-muted/40 p-3"
                >
                  <div class="flex flex-wrap items-center justify-between gap-2">
                    <p class="text-sm font-medium">{{ example.label }}</p>
                    <UBadge :label="example.address" color="neutral" variant="subtle" />
                  </div>
                  <p class="mt-2 break-all font-mono text-sm">{{ example.hostname }}</p>
                </div>
              </div>
            </UCard>

            <UCard>
              <template #header>
                <div>
                  <h3 class="font-semibold">Additional Managed Domains</h3>
                  <p class="text-sm text-muted">Select domains from the allowlist configured by the administrator.</p>
                </div>
              </template>

              <UAlert
                icon="i-lucide-triangle-alert"
                color="warning"
                title="Public DNS override"
                description="Enabling nip.io, sslip.io, xip.io or another public domain overrides its normal public DNS resolution for every client using this TDNS server. Requests are answered locally instead."
              />

              <div v-if="wildcard.available_extra_domains.length === 0" class="py-8">
                <UEmpty
                  icon="i-lucide-network"
                  title="No additional domains available"
                  description="Add domains to wildcard.available_extra_domains in the YAML configuration before enabling them here."
                />
              </div>

              <div v-else class="mt-5 space-y-3">
                <UCheckbox
                  v-for="domain in wildcard.available_extra_domains"
                  :key="domain"
                  :label="domain"
                  :description="selectedDomains.includes(domain) ? 'Will be managed locally' : 'Uses normal DNS resolution'"
                  :model-value="selectedDomains.includes(domain)"
                  :disabled="domainsLoading"
                  :aria-label="`Manage ${domain} locally`"
                  variant="card"
                  indicator="end"
                  size="sm"
                  :class="[
                    'w-full bg-muted/40',
                    domainsLoading ? 'cursor-not-allowed' : 'cursor-pointer'
                  ]"
                  :ui="{
                    root: 'items-center border-0 has-data-[state=checked]:border-0',
                    wrapper: 'min-w-0',
                    label: 'break-all font-mono text-sm font-medium',
                    description: 'text-xs text-muted'
                  }"
                  @update:model-value="value => updateDomain(domain, Boolean(value))"
                />
              </div>

              <template #footer>
                <div class="flex justify-end">
                  <UButton
                    icon="i-lucide-save"
                    label="Save Domain Selection"
                    :loading="domainsLoading"
                    :disabled="!domainSelectionChanged || domainsLoading"
                    @click="handleSaveDomains"
                  />
                </div>
              </template>
            </UCard>
          </div>

          <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
            <UAlert
              icon="i-lucide-radio"
              color="info"
              title="Avoid .local names"
              description="The .local suffix is reserved for multicast DNS (mDNS). Operating systems may send those names to mDNS instead of TDNS, so use the configured home.arpa domain for reliable unicast DNS resolution."
            />
            <UAlert
              icon="i-lucide-shield-check"
              color="info"
              title="Browser secure DNS can bypass TDNS"
              description="A browser using its own DNS-over-HTTPS provider may bypass the operating system resolver. Configure secure DNS to use the system provider, or disable it, when testing these names."
            />
          </div>

          <UAlert
            icon="i-lucide-database"
            color="neutral"
            title="Persisted operator settings"
            description="The start/stop state and additional domain selection are stored as configuration overrides and survive restarts. The primary domain and available domain allowlist remain controlled by YAML configuration."
          />
        </template>
      </div>
    </template>
  </UDashboardPanel>
</template>
