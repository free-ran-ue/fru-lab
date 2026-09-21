import type { NetworkFunction } from './types'

export type Free5gcTemplate = 'basic' | 'ulcl'

export const FREE5GC_TEMPLATE_OPTIONS: { value: Free5gcTemplate; label: string }[] = [
  { value: 'basic', label: 'Basic' },
  { value: 'ulcl', label: 'ULCL' },
]

// Which template is currently deployed, detected purely from which NF
// service names are live right now - free5gc's own template choice isn't
// tracked as separate state anywhere, it's just read back off the running
// containers (I-UPF/PSA-UPF only exist under the ulcl template). Null means
// "nothing deployed yet", not "unknown template".
export function detectFree5gcTemplate(nfs: NetworkFunction[]): Free5gcTemplate | null {
  if (nfs.length === 0) return null
  return nfs.some((nf) => nf.name === 'I-UPF' || nf.name === 'PSA-UPF') ? 'ulcl' : 'basic'
}
