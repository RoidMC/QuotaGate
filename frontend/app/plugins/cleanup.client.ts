export default defineNuxtPlugin({
  name: 'cleanup-nuxt-state',
  hooks: {
    'app:mounted': () => {
      setTimeout(() => {
        document.querySelectorAll('script').forEach((script) => {
          const text = script.textContent || ''
          if (text.includes('window.__NUXT__') || text.includes('window.__NUXT_SITE_CONFIG__')) {
            script.remove()
            //console.log('[Cleanup] Removed Nuxt hydration script tag from DOM')
          }
          if (script.type === 'application/json' && script.getAttribute('data-nuxt-data')) {
            script.remove()
            //console.log('[Cleanup] Removed nuxt-data script tag from DOM')
          }
        })
      }, 100)
    }
  }
})
