<script setup lang="ts">
const route = useRoute()
const { logout } = useAuth()
const { navigationGroups } = useDashboardNavigation()
const toast = useToast()
const { blacklist, initialized, refreshing, toggleLoading, refresh, setEnabled } = useBlacklist()
const logoutLoading = ref(false)

const currentSection = computed(() => {
  const items = navigationGroups.value.flat().filter(item => item.type !== 'label')
  return items.find(item => item.to === route.path)?.label || 'Dashboard'
})

const blacklistLegend = computed(() => (
  blacklist.value.enabled ? 'Blacklist Active' : 'Blocking Paused'
))

async function handleLogout() {
  if (logoutLoading.value) {
    return
  }

  logoutLoading.value = true
  const loggedOut = await logout()
  logoutLoading.value = false
  if (!loggedOut) {
    toast.add({
      title: 'Unable to sign out',
      description: 'The server session is still active',
      color: 'error',
      icon: 'i-lucide-circle-alert'
    })
    return
  }
  await navigateTo('/login')
}

async function handleBlacklistToggle(nextEnabled: boolean) {
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

onMounted(() => {
  refresh()
})
</script>

<template>
  <UDashboardGroup
    class="min-h-screen bg-default"
    storage-key="dashboard-shell-v1"
    unit="px"
  >
    <UDashboardSidebar
      id="app-sidebar"
      collapsible
      resizable
      mode="slideover"
      :default-size="300"
      :min-size="220"
      :max-size="360"
      :collapsed-size="80"
    >
      <template #header="{ collapsed }">
        <div class="flex items-center gap-3 p-3">
          <div class="flex size-10 items-center justify-center rounded-xl bg-primary/10">
            <UIcon name="i-lucide-server" class="size-5 text-primary" />
          </div>
          <div v-if="!collapsed" class="min-w-0">
            <p class="truncate text-sm font-semibold">TDNS Admin</p>
            <p class="truncate text-xs text-muted">DNS control panel</p>
          </div>
        </div>
      </template>

      <template #default="{ collapsed }">
        <div class="flex h-full flex-col gap-4 px-2 pb-2">
          <UNavigationMenu
            :items="navigationGroups"
            orientation="vertical"
            highlight
            :collapsed="collapsed"
            :tooltip="collapsed ? { delayDuration: 0, content: { side: 'right' } } : false"
            :ui="{ link: collapsed ? 'justify-center' : undefined }"
          />

          <UCard v-if="!collapsed && initialized" variant="subtle">
            <div class="flex items-start justify-between gap-3">
              <div class="space-y-1">
                <p class="text-xs uppercase tracking-[0.18em] text-muted">Pause blocking</p>
                <p class="text-sm font-medium">{{ blacklistLegend }}</p>
                <p class="text-xs text-muted">
                  {{ blacklist.blockfile_total_entries.toLocaleString() }} entries in the blockfile
                </p>
              </div>
              <USwitch
                :model-value="blacklist.enabled"
                :loading="toggleLoading"
                @update:model-value="handleBlacklistToggle"
              />
            </div>
          </UCard>

          <USkeleton v-else-if="!collapsed && refreshing" class="h-24 w-full" />

          <UCard v-if="!collapsed" variant="subtle">
            <div class="space-y-1">
              <p class="text-xs uppercase tracking-[0.18em] text-muted">Current</p>
              <p class="text-sm font-medium">{{ currentSection }}</p>
              <p class="text-xs text-muted">Use the panel to move between DNS tools without leaving the dashboard shell.</p>
            </div>
          </UCard>
        </div>
      </template>

      <template #footer="{ collapsed }">
        <div class="flex items-center gap-2 p-2">
          <UDashboardSidebarCollapse
            color="neutral"
            variant="ghost"
            :class="collapsed ? 'mx-auto' : ''"
          />

          <UButton
            icon="i-lucide-log-out"
            :label="collapsed ? undefined : 'Sign out'"
            color="neutral"
            variant="ghost"
            :block="!collapsed"
            :loading="logoutLoading"
            :disabled="logoutLoading"
            @click="handleLogout"
          />
        </div>
      </template>
    </UDashboardSidebar>

    <div class="flex min-w-0 flex-1 flex-col">
      <slot />
    </div>
  </UDashboardGroup>
</template>
