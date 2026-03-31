<script setup lang="ts">
import { z } from 'zod'
import type { FormSubmitEvent } from '@nuxt/ui'

definePageMeta({
  layout: false
})

const { setToken, isAuthenticated } = useAuth()
const toast = useToast()

// Redirect if already authenticated
if (isAuthenticated.value) {
  navigateTo('/dashboard')
}

const schema = z.object({
  token: z.string().min(1, 'JWT token is required')
})

type Schema = z.output<typeof schema>

const state = reactive({
  token: ''
})

const loading = ref(false)

async function onSubmit(event: FormSubmitEvent<Schema>) {
  loading.value = true
  
  // Simple validation - just store the token
  // The token will be validated on the first API call
  setToken(event.data.token)
  
  toast.add({
    title: 'Token saved',
    description: 'You have been authenticated',
    color: 'success',
    icon: 'i-lucide-check-circle'
  })
  
  loading.value = false
  navigateTo('/dashboard')
}
</script>

<template>
  <UApp>
    <div class="min-h-screen flex items-center justify-center bg-default p-4">
      <UCard class="w-full max-w-md">
        <template #header>
          <div class="flex flex-col items-center gap-4">
            <div class="flex items-center gap-2">
              <UIcon name="i-lucide-server" class="size-10 text-primary" />
              <h1 class="text-2xl font-bold">TDNS Admin</h1>
            </div>
            <p class="text-muted text-center">
              Enter your JWT token to access the DNS administration panel
            </p>
          </div>
        </template>

        <UForm :schema="schema" :state="state" class="space-y-4" @submit="onSubmit">
          <UFormField name="token" label="JWT Token">
            <UTextarea
              v-model="state.token"
              placeholder="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
              :rows="4"
              autoresize
            />
          </UFormField>

          <UButton type="submit" block :loading="loading" icon="i-lucide-log-in">
            Sign In
          </UButton>
        </UForm>

        <template #footer>
          <p class="text-sm text-muted text-center">
            The token will be stored locally and used for API authentication
          </p>
        </template>
      </UCard>
    </div>
  </UApp>
</template>
