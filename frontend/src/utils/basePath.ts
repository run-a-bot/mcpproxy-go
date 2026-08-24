// externalBasePath returns the reverse-proxy prefix before MCPProxy's /ui route.
// Direct deployments return an empty prefix and keep existing root URLs.
export function externalBasePath(pathname = window.location.pathname): string {
  const marker = '/ui/'
  const index = pathname.indexOf(marker)
  return index >= 0 ? pathname.slice(0, index) : ''
}
