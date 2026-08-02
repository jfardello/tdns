import { createManagementApiClient } from '~/lib/apiClient'

export function useApiClient() {
  const config = useRuntimeConfig()
  const { csrfToken, expireSession } = useAuth()
  const toast = useToast()

  return createManagementApiClient(config.public.apiBaseUrl, {
    getCSRFToken: () => csrfToken.value,
    async onUnauthorized() {
      if (!expireSession()) {
        return
      }
      toast.add({
        title: 'Session expired',
        description: 'Sign in again to continue',
        color: 'error',
        icon: 'i-lucide-circle-alert'
      })
      await navigateTo('/login')
    },
    onError(description) {
      toast.add({
        title: 'API Error',
        description,
        color: 'error',
        icon: 'i-lucide-circle-alert'
      })
    }
  })
}
