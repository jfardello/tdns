<script setup lang="ts">
const { isAuthenticated, restoreSession } = useAuth()

onMounted(async () => {
  await restoreSession()
  await navigateTo(isAuthenticated.value ? '/dashboard' : '/login')
})
</script>

<template>
  <div class="min-h-screen flex items-center justify-center">
    <UIcon name="i-lucide-loader-circle" class="size-8 animate-spin text-primary" />
  </div>
</template>
