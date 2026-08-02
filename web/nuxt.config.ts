export default defineNuxtConfig({
  devtools: { enabled: process.env.NODE_ENV === 'development' },
  modules: ['@nuxt/ui', 'nuxt-charts'],
  css: ['~/assets/css/main.css'],
  ssr: false,
  app: {
    head: {
      meta: [
        { name: 'viewport', content: 'width=device-width, initial-scale=1' }
      ]
    }
  },
  icon: {
    provider: 'none',
    fallbackToApi: false,
    clientBundle: {
      scan: true
    }
  },
  nitro: {
    devProxy: process.env.TDNS_API_PROXY_TARGET
      ? {
          '/api': {
            target: process.env.TDNS_API_PROXY_TARGET,
            changeOrigin: false
          }
        }
      : undefined
  },
  runtimeConfig: {
    public: {
      apiBaseUrl: ''
    }
  }
})
