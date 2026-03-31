export default defineNuxtRouteMiddleware((to) => {
  // Skip middleware for login page
  if (to.path === '/login') {
    return
  }
  
  // Check for token on client side
  if (import.meta.client) {
    const token = localStorage.getItem('tdns_jwt_token')
    if (!token && to.path !== '/login') {
      return navigateTo('/login')
    }
  }
})
