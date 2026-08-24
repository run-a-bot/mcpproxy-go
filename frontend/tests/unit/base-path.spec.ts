import { describe, expect, it } from 'vitest'

import { externalBasePath } from '@/utils/basePath'

describe('externalBasePath', () => {
  it('keeps direct dashboard URLs at origin root', () => {
    expect(externalBasePath('/ui/servers')).toBe('')
  })

  it('detects a reverse-proxy prefix', () => {
    expect(externalBasePath('/api/v1/addons/u1010a5/mcpproxy/proxy/ui/servers')).toBe(
      '/api/v1/addons/u1010a5/mcpproxy/proxy',
    )
  })
})
