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
        label: 'Plugins',
        type: 'label'
      },
      {
        label: 'Blacklist',
        icon: 'i-lucide-shield-ban',
        to: '/dashboard/blacklist'
      },
      {
        label: 'Zen Mode',
        icon: 'i-lucide-focus',
        to: '/dashboard/zen-mode'
      },
      {
        label: 'Static Response',
        icon: 'i-lucide-file-text',
        to: '/dashboard/static-response'
      },
      {
        label: 'Stub Resolver',
        icon: 'i-lucide-git-branch',
        to: '/dashboard/stub-resolver'
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
