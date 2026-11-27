import { useLab } from '../LabContext'
import type { LogSource } from '../console/stream'
import type { SourceOption } from './LogsTab'

// logSourceFor is where a node's logs come from.
export function logSourceFor(id: string | null): LogSource | null {
  if (!id) return null
  if (id === 'core') return { target: 'free5gc' }
  if (id.startsWith('nf:')) return { target: 'free5gc', service: id.slice(3).toLowerCase() }
  // a gNB's node id is its target name, gnb-<profileId>
  if (/^gnb-[0-9a-f]{12}$/.test(id)) return { target: id as `gnb-${string}` }
  if (id.startsWith('ue:')) return { target: 'ue', instance: id.slice(3) }
  return null
}

// logsHref links to the Logs page with `id`'s logs picked.
export function logsHref(id: string): string {
  const s = logSourceFor(id)
  if (!s) return '/logs'
  const q = new URLSearchParams({ target: s.target })
  if (s.instance) q.set('instance', s.instance)
  if (s.service) q.set('service', s.service)
  return `/logs?${q}`
}

// sourceFromQuery reads the source logsHref put in the URL.
export function sourceFromQuery(q: URLSearchParams): LogSource | null {
  const target = q.get('target')
  if (!target) return null
  return { target: target as LogSource['target'], instance: q.get('instance') ?? undefined, service: q.get('service') ?? undefined }
}

// useLogSources lists every source there is something to follow from: the
// core and each NF, every running gNB, every UE deployed at least once.
export function useLogSources(): SourceOption[] {
  const lab = useLab()
  const options: SourceOption[] = []
  if (lab.coreUp) {
    options.push({ label: 'free5GC · all NFs', source: { target: 'free5gc' } })
    for (const nf of lab.free5gc.networkFunctions) options.push({ label: `free5GC · ${nf.name}`, source: { target: 'free5gc', service: nf.name.toLowerCase() } })
  }
  for (const g of lab.gnbs) options.push({ label: `gNB ${g.profileName}`, source: { target: g.id as `gnb-${string}` } })
  for (const r of lab.ue.rows) if (r.hasBeenDeployed) options.push({ label: `UE ${r.ueId}`, source: { target: 'ue', instance: r.ueId } })
  return options
}
