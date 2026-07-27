export default defineNuxtConfig({
  devtools: { enabled: process.env.NODE_ENV === 'development' },
  modules: ['@nuxt/ui', 'nuxt-charts'],
  css: ['~/assets/css/main.css'],
  ssr: false,
  runtimeConfig: {
    public: {
      apiBaseUrl: process.env.TDNS_API_URL || ''
    }
  }
})
