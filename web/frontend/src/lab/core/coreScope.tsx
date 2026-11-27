import type { ReactNode } from 'react'
import { useParams } from 'react-router-dom'
import AppLayout from '../../components/layout/AppLayout'
import type { WebconsoleApi } from '../../webconsoleApi'
import { webconsoleClient } from '../../webconsoleApiClient'
import { useLab } from '../LabContext'
import { ui } from '../LabUi'
import { isFree5gcCore } from '../useLab'

// CoreScope is which free5GC a Subscribers page works on: the lab free5GC
// (/core-network/lab/subscribers…) or a free5GC core network
// (/core-network/n/<id>/subscribers…). Every link inside those pages is
// built on `base`, so it stays on the same core.
export interface CoreScope {
  // the core network id; undefined for the lab free5GC
  coreId?: string
  // the core's page: /core-network/lab or /core-network/n/<id>
  base: string
  // its name, for breadcrumbs and messages
  name: string
  // its webconsole
  webconsole: WebconsoleApi
}

// useCoreScope reads the scope from the route's :id. Null while the core
// networks load; a string says why there is none (no such core, or one
// outside the lab, which has no webconsole here).
export function useCoreScope(): CoreScope | string | null {
  const { id } = useParams<{ id: string }>()
  const lab = useLab()
  if (!id) return { base: '/core-network/lab', name: 'free5GC', webconsole: webconsoleClient() }
  const n = lab.coreNetworks.networks.find((c) => c.id === id)
  if (!n) return lab.coreNetworks.loaded ? 'No such core network. It may have been deleted.' : null
  if (!isFree5gcCore(n)) return `${n.name} is a core outside fru-lab: its subscribers are managed on that core, not here.`
  return { coreId: id, base: `/core-network/n/${id}`, name: n.name, webconsole: webconsoleClient(id) }
}

// WithCoreScope renders `children` with the route's scope, or why there is none.
export function WithCoreScope({ children }: { children: (scope: CoreScope) => ReactNode }) {
  const scope = useCoreScope()
  if (scope && typeof scope === 'object') return <>{children(scope)}</>
  return (
    <AppLayout crumbs={[{ label: 'Subscribers' }]}>
      <p className={ui.empty}>{scope ?? 'Loading…'}</p>
    </AppLayout>
  )
}
