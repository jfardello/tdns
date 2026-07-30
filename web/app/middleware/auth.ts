export default defineNuxtRouteMiddleware(async () => {
  const { isAuthenticated, restoreSession } = useAuth()

  await restoreSession()
  if (!isAuthenticated.value) {
    return navigateTo('/login')
  }
})
