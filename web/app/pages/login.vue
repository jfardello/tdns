<script setup lang="ts">
import { z } from 'zod'
import type { FormSubmitEvent, TabsItem } from '@nuxt/ui'
import type { LoginResult } from '~/composables/useAuth'
import {
  clearLoginSecrets,
  createBrowserLoginFormState,
  takeCodeSubmission,
  takePasswordSubmission
} from '~/lib/browserLoginForm'

definePageMeta({
  layout: false
})

type LoginMode = 'password' | 'code'

const { exchangeCode, loginWithPassword, isAuthenticated, isLoading, restoreSession } = useAuth()
const toast = useToast()

const passwordSchema = z.object({
  username: z.string().trim().min(1, 'Username is required'),
  password: z.string().min(1, 'Password is required')
})
const codeSchema = z.object({
  code: z.string().trim().min(1, 'Browser code is required')
})

type PasswordSchema = z.output<typeof passwordSchema>
type CodeSchema = z.output<typeof codeSchema>

const mode = ref<LoginMode>('password')
const submitting = ref(false)
const showPassword = ref(false)
const state = reactive(createBrowserLoginFormState())
const loginModes = computed<TabsItem[]>(() => [
  {
    label: 'Password',
    value: 'password',
    icon: 'i-lucide-key-round',
    disabled: submitting.value
  },
  {
    label: 'Browser code',
    value: 'code',
    icon: 'i-lucide-terminal',
    disabled: submitting.value
  }
])

watch(mode, () => {
  clearLoginSecrets(state)
  showPassword.value = false
})

onMounted(async () => {
  await restoreSession()
  if (isAuthenticated.value) {
    await navigateTo('/dashboard')
  }
})

async function completeLogin(result: LoginResult) {
  if (result === 'success') {
    toast.add({
      title: 'Signed in',
      description: 'The browser session is active',
      color: 'success',
      icon: 'i-lucide-circle-check'
    })
    await navigateTo('/dashboard')
    return
  }

  if (result === 'invalid-credentials') {
    toast.add({
      title: 'Unable to sign in',
      description: 'The username or password is incorrect',
      color: 'error',
      icon: 'i-lucide-circle-alert'
    })
    return
  }

  if (result === 'invalid-code') {
    toast.add({
      title: 'Unable to sign in',
      description: 'The browser code is invalid, expired, or already used',
      color: 'error',
      icon: 'i-lucide-circle-alert'
    })
    return
  }

  if (result === 'rate-limited') {
    toast.add({
      title: 'Too many attempts',
      description: 'Wait before trying to sign in again',
      color: 'error',
      icon: 'i-lucide-clock-alert'
    })
    return
  }

  toast.add({
    title: 'Unable to sign in',
    description: 'The server could not complete the sign-in request',
    color: 'error',
    icon: 'i-lucide-circle-alert'
  })
}

async function submitPassword(event: FormSubmitEvent<PasswordSchema>) {
  submitting.value = true
  state.username = event.data.username
  state.password = event.data.password
  const submission = takePasswordSubmission(state)
  showPassword.value = false

  try {
    await completeLogin(await loginWithPassword(
      submission.username,
      submission.password,
      submission.remember
    ))
  } finally {
    submitting.value = false
  }
}

async function submitCode(event: FormSubmitEvent<CodeSchema>) {
  submitting.value = true
  state.code = event.data.code
  const submission = takeCodeSubmission(state)

  try {
    await completeLogin(await exchangeCode(submission.code, submission.remember))
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <UApp>
    <main class="min-h-screen flex items-center justify-center bg-default p-4">
      <div v-if="isLoading" class="flex items-center justify-center" aria-label="Checking session">
        <UIcon name="i-lucide-loader-circle" class="size-8 animate-spin text-primary" />
      </div>

      <UCard v-else class="w-full min-w-0 max-w-md overflow-hidden">
        <template #header>
          <div class="flex flex-col items-center gap-3">
            <div class="flex items-center gap-2">
              <UIcon name="i-lucide-server" class="size-10 text-primary" />
              <h1 class="text-2xl font-bold">TDNS Admin</h1>
            </div>
            <p class="text-muted text-center">Sign in to manage TDNS</p>
          </div>
        </template>

        <div class="min-w-0 space-y-5">
          <UTabs
            v-model="mode"
            :items="loginModes"
            :content="false"
            class="w-full"
            :ui="{
              root: 'min-w-0',
              list: 'grid w-full min-w-0 grid-cols-2',
              trigger: 'min-w-0',
              label: 'truncate'
            }"
            aria-label="Authentication method"
          />

          <div class="min-h-72 min-w-0">
            <UForm
              v-if="mode === 'password'"
              :schema="passwordSchema"
              :state="state"
              class="space-y-4"
              @submit="submitPassword"
            >
              <UFormField name="username" label="Username">
                <UInput
                  v-model="state.username"
                  autocomplete="username"
                  icon="i-lucide-user"
                  :disabled="submitting"
                  autofocus
                  class="w-full"
                />
              </UFormField>

              <UFormField name="password" label="Password">
                <UInput
                  v-model="state.password"
                  :type="showPassword ? 'text' : 'password'"
                  autocomplete="current-password"
                  icon="i-lucide-lock-keyhole"
                  :disabled="submitting"
                  class="w-full"
                >
                  <template #trailing>
                    <UButton
                      type="button"
                      color="neutral"
                      variant="link"
                      size="sm"
                      :icon="showPassword ? 'i-lucide-eye-off' : 'i-lucide-eye'"
                      :aria-label="showPassword ? 'Hide password' : 'Show password'"
                      :disabled="submitting"
                      @click="showPassword = !showPassword"
                    />
                  </template>
                </UInput>
              </UFormField>

              <UCheckbox v-model="state.remember" label="Remember this browser" :disabled="submitting" />

              <UButton
                type="submit"
                block
                :loading="submitting"
                :disabled="submitting"
                icon="i-lucide-log-in"
              >
                Sign in
              </UButton>
            </UForm>

            <UForm
              v-else
              :schema="codeSchema"
              :state="state"
              class="space-y-4"
              @submit="submitCode"
            >
              <UFormField name="code" label="Browser code">
                <UInput
                  v-model="state.code"
                  autocomplete="one-time-code"
                  autocapitalize="off"
                  spellcheck="false"
                  icon="i-lucide-terminal"
                  placeholder="Paste the browser code"
                  :disabled="submitting"
                  autofocus
                  class="w-full"
                />
              </UFormField>

              <UCheckbox v-model="state.remember" label="Remember this browser" :disabled="submitting" />

              <UButton
                type="submit"
                block
                :loading="submitting"
                :disabled="submitting"
                icon="i-lucide-log-in"
              >
                Sign in
              </UButton>

              <p class="text-sm text-muted text-center">
                Generate a code with <code>tdns adm browser-code</code>
              </p>
            </UForm>
          </div>
        </div>
      </UCard>
    </main>
  </UApp>
</template>
