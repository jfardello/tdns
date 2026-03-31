import type { NavigationMenuItem } from '@nuxt/ui'

export function useDashboardNavigation() {
  const navigationGroups = computed<NavigationMenuItem[][]>(() => [
    [
      {
        label: 'Overview',
        type: 'label'
      },
      {
        label: 'Dashboard',
        icon: 'i-lucide-layout-dashboard',
        to: '/dashboard'
      },
      {
        label: 'Top Domains',
        icon: 'i-lucide-bar-chart-3',
        to: '/dashboard/top-domains'
      }
    ],
    [
      {
        label: 'Controls',
        type: 'label'
      },
      {
        label: 'Plugins',
        icon: 'i-lucide-plug',
        to: '/dashboard/plugins'
      },
      {
        label: 'Blacklist',
        icon: 'i-lucide-shield-ban',
        to: '/dashboard/blacklist'
      },
      {
        label: 'Tag Management',
        icon: 'i-lucide-tags',
        to: '/dashboard/tags'
      }
    ]
  ])

  return {
    navigationGroups
  }
}
