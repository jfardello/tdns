<script setup lang="ts">
definePageMeta({ layout: 'dashboard', middleware: 'auth' })

const { toggleBlacklist, toggleStaticResponse, toggleStubResolver, toggleZenMode } = useApi()
const toast = useToast()

interface Plugin {
  id: string
  name: string
  description: string
  icon: string
  enabled: boolean
  loading: boolean
}

const plugins = ref<Plugin[]>([
  {
    id: 'blacklist',
    name: 'Blacklist Filter',
    description: 'Block DNS queries to blacklisted domains',
    icon: 'i-lucide-shield-ban',
    enabled: false,
    loading: false
  },
  {
    id: 'static-response',
    name: 'Static Response',
    description: 'Return predefined responses for specific hosts',
    icon: 'i-lucide-file-text',
    enabled: false,
    loading: false
  },
  {
    id: 'stub-resolver',
    name: 'Stub Resolver',
    description: 'Forward queries to configured upstream resolvers',
    icon: 'i-lucide-git-branch',
    enabled: false,
    loading: false
  },
  {
    id: 'zen-mode',
    name: 'Zen Mode',
    description: 'Block distracting domains during focus sessions',
    icon: 'i-lucide-focus',
    enabled: false,
    loading: false
  }
])

async function togglePlugin(plugin: Plugin) {
  plugin.loading = true
  const action = plugin.enabled ? 'stop' : 'start'
  
  let response = null
  
  switch (plugin.id) {
    case 'blacklist':
      response = await toggleBlacklist(action)
      break
    case 'static-response':
      response = await toggleStaticResponse(action)
      break
    case 'stub-resolver':
      response = await toggleStubResolver(action)
      break
    case 'zen-mode':
      response = await toggleZenMode()
      break
  }
  
  if (response) {
    plugin.enabled = !plugin.enabled
    toast.add({
      title: `${plugin.name} ${plugin.enabled ? 'enabled' : 'disabled'}`,
      description: response.message || `Plugin is now ${plugin.enabled ? 'active' : 'inactive'}`,
      color: 'success',
      icon: 'i-lucide-check-circle'
    })
  }
  
  plugin.loading = false
}
</script>

<template>
  <UDashboardPanel>
    <template #header>
      <UDashboardNavbar title="Enable Plugins">
        <template #right>
          <UBadge
            :label="`${plugins.filter(p => p.enabled).length} active`"
            color="primary"
            variant="subtle"
          />
        </template>
      </UDashboardNavbar>
    </template>

    <template #body>
      <div class="p-6">
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <UCard
            v-for="plugin in plugins"
            :key="plugin.id"
            :class="plugin.enabled ? 'border-primary' : ''"
          >
            <div class="flex items-start justify-between">
              <div class="flex items-start gap-4">
                <div
                  class="p-3 rounded-full"
                  :class="plugin.enabled ? 'bg-primary/10' : 'bg-muted'"
                >
                  <UIcon
                    :name="plugin.icon"
                    class="size-6"
                    :class="plugin.enabled ? 'text-primary' : 'text-muted'"
                  />
                </div>
                <div>
                  <h3 class="font-semibold">{{ plugin.name }}</h3>
                  <p class="text-sm text-muted mt-1">{{ plugin.description }}</p>
                </div>
              </div>
              <USwitch
                :model-value="plugin.enabled"
                :loading="plugin.loading"
                @update:model-value="togglePlugin(plugin)"
              />
            </div>
            
            <div class="mt-4 pt-4 border-t border-muted flex items-center justify-between">
              <UBadge
                :label="plugin.enabled ? 'Active' : 'Inactive'"
                :color="plugin.enabled ? 'success' : 'neutral'"
                variant="subtle"
              />
              <UButton
                v-if="plugin.enabled"
                variant="ghost"
                size="sm"
                label="Configure"
                trailing-icon="i-lucide-settings"
                disabled
              />
            </div>
          </UCard>
        </div>

        <!-- Info Card -->
        <UAlert
          class="mt-6"
          icon="i-lucide-info"
          color="info"
          title="Plugin Information"
          description="Plugins extend the functionality of your DNS server. Enable or disable them based on your needs. Some plugins may require additional configuration."
        />
      </div>
    </template>
  </UDashboardPanel>
</template>
