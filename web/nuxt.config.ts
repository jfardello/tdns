export default defineNuxtConfig({
  devtools: { enabled: true },
  modules: ['@nuxt/ui'],
  css: ['~/assets/css/main.css'],
  ssr: false,
  runtimeConfig: {
    public: {
      apiBaseUrl: process.env.TDNS_API_URL || ''
    }
  }
})
