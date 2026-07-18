import { createManagementApiClient } from '~/lib/apiClient'

export function useApiClient() {
  const config = useRuntimeConfig()
  const { token, clearToken } = useAuth()
  const toast = useToast()

  return createManagementApiClient(config.public.apiBaseUrl, {
    getToken: () => token.value,
    async onUnauthorized() {
      clearToken()
      toast.add({
        title: 'Session expired',
        description: 'Please login again',
        color: 'error',
        icon: 'i-lucide-alert-circle'
      })
      await navigateTo('/login')
    },
    onError(description) {
      toast.add({
        title: 'API Error',
        description,
        color: 'error',
        icon: 'i-lucide-alert-circle'
      })
    }
  })
}
