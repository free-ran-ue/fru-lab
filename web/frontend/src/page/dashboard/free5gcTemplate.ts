import type { NetworkFunction } from './types'

export type Free5gcTemplate = 'basic' | 'ulcl' | 'ulcl-2slice'

export const FREE5GC_TEMPLATE_OPTIONS: { value: Free5gcTemplate; label: string }[] = [
  { value: 'basic', label: 'Basic' },
  { value: 'ulcl', label: 'ULCL' },
  { value: 'ulcl-2slice', label: 'ULCL (Two Slice)' },
]

// Which template is currently deployed, detected purely from which NF
// service names are live right now - free5gc's own template choice isn't
// tracked as separate state anywhere, it's just read back off the running
// containers (I-UPF/PSA-UPF only exist under ulcl; the -1/-2 suffixed pairs
// only exist under ulcl-2slice, which runs its own dedicated SMF per
// slice). Null means "nothing deployed yet", not "unknown template".
export function detectFree5gcTemplate(nfs: NetworkFunction[]): Free5gcTemplate | null {
  if (nfs.length === 0) return null
  if (nfs.some((nf) => nf.name === 'I-UPF-1' || nf.name === 'I-UPF-2' || nf.name === 'PSA-UPF-1' || nf.name === 'PSA-UPF-2')) {
    return 'ulcl-2slice'
  }
  if (nfs.some((nf) => nf.name === 'I-UPF' || nf.name === 'PSA-UPF')) return 'ulcl'
  return 'basic'
}
