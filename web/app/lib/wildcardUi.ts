export interface WildcardExample {
  label: string
  hostname: string
  address: string
}

export function wildcardExamples(primaryDomain: string): WildcardExample[] {
  const domain = primaryDomain.trim().replace(/\.+$/, '') || 'tdns.home.arpa'
  return [
    { label: 'IPv4 dots', hostname: `app.192.168.1.20.${domain}`, address: '192.168.1.20' },
    { label: 'IPv4 dashes', hostname: `app-192-168-1-20.${domain}`, address: '192.168.1.20' },
    { label: 'IPv4 hexadecimal', hostname: `app-c0a80114.${domain}`, address: '192.168.1.20' },
    { label: 'IPv6 dashes', hostname: `fd00--20.${domain}`, address: 'fd00::20' }
  ]
}

export function updateWildcardDomainSelection(
  availableDomains: string[],
  selectedDomains: string[],
  domain: string,
  enabled: boolean
): string[] {
  const selected = new Set(selectedDomains.filter(value => availableDomains.includes(value)))
  if (availableDomains.includes(domain)) {
    if (enabled) {
      selected.add(domain)
    } else {
      selected.delete(domain)
    }
  }
  return availableDomains.filter(value => selected.has(value))
}
