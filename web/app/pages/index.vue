<script setup lang="ts">
const { isAuthenticated } = useAuth()

// Redirect based on auth state
if (isAuthenticated.value) {
  navigateTo('/dashboard')
} else {
  navigateTo('/login')
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center">
    <UIcon name="i-lucide-loader-2" class="size-8 animate-spin text-primary" />
  </div>
</template>
