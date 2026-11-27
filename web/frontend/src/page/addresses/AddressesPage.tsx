import { useState } from 'react'
import AppLayout from '../../components/layout/AppLayout'
import { useLab } from '../../lab/LabContext'
import { Dot, Hint, KV, Section, ui } from '../../lab/LabUi'
import { SortTh, useSort } from '../../lab/sort'
import DetailHeader from '../../lab/workbench/DetailHeader'
import ItemList from '../../lab/workbench/ItemList'
import Split from '../../lab/workbench/Split'
import Workbench from '../../lab/workbench/Workbench'
import type { IpamLease } from '../../api'
import styles from './addresses.module.css'

// AddressesPage shows the backend's address plan (GET /api/ipam): every
// pool fru-lab manages - each core's cn-ran and SBI networks, the shared
// ran-ue - with its reserved addresses, its dynamic ranges and who holds
// which address, plus any warning (a stray holder, an overlapping host
// network).
export default function AddressesPage() {
  const lab = useLab()
  const { pools, warnings } = lab.ipam
  const [picked, setPicked] = useState<string | null>(null)
  const pool = pools.find((p) => p.id === picked) ?? pools[0]
  const coreName = (id?: string) => (id ? lab.coreNetworks.networks.find((n) => n.id === id)?.name ?? id : 'free5GC')

  const { sorted: leases, sort, toggle } = useSort<IpamLease>(pool?.leases ?? [], {
    addr: (l) => l.addr, owner: (l) => l.owner, role: (l) => l.role, live: (l) => (l.live ? 0 : 1), since: (l) => l.since,
  }, { key: 'addr', dir: 'asc' })

  const list = (
    <ItemList
      label="Address pools"
      columns={['Pool', 'Subnet', 'Core', 'Leases']}
      template="minmax(0, 1fr) minmax(0, 1fr) minmax(0, 1fr) 3.5rem"
      rows={pools.map((p) => ({
        id: p.id,
        status: p.leases.some((l) => l.live) ? 'running' : 'stopped',
        cells: [p.id, p.subnet, p.id === 'ran-ue' ? 'shared' : coreName(p.coreId), p.leases.length],
      }))}
      selectedId={pool?.id ?? null}
      onSelect={setPicked}
      defaultSort={{ key: '0', dir: 'asc' }}
      head={warnings.length > 0 ? <div className={styles.warnings}>{warnings.map((w) => <Hint key={w}>{w}</Hint>)}</div> : undefined}
      empty={lab.ipam.view ? 'No address pools.' : 'Loading the address plan…'}
    />
  )

  const detail = pool
    ? (
      <>
        <DetailHeader icon="nf" name={pool.id} sub={`${pool.network} · ${pool.subnet}`} />
        <Section title="Pool">
          <KV rows={[
            { k: 'Purpose', v: pool.purpose },
            { k: 'Docker network', v: pool.network },
            { k: 'Subnet', v: pool.subnet },
            { k: 'Core', v: pool.id === 'ran-ue' ? 'shared by every gNB and UE' : coreName(pool.coreId) },
          ]} />
        </Section>
        <Section title="Ranges" aside={<span>{pool.ranges.length}</span>}>
          <table className={styles.tbl}>
            <thead><tr><th>Range</th><th>First</th><th>Last</th></tr></thead>
            <tbody>{pool.ranges.map((r) => <tr key={r.name}><td>{r.name}</td><td>{r.first}</td><td>{r.last}</td></tr>)}</tbody>
          </table>
        </Section>
        <Section title="Reserved" aside={<span>{pool.reserved.length}</span>}>
          <table className={styles.tbl}>
            <thead><tr><th>Address</th><th>Role</th></tr></thead>
            <tbody>{pool.reserved.map((r) => <tr key={r.addr + r.role}><td>{r.addr}</td><td>{r.role}</td></tr>)}</tbody>
          </table>
        </Section>
        <Section title="Leases" aside={<span>{pool.leases.length}</span>}>
          {pool.leases.length === 0
            ? <p className={ui.empty} style={{ padding: 0 }}>Nothing holds an address here.</p>
            : (
              <table className={styles.tbl}>
                <thead>
                  <tr>
                    <SortTh k="addr" sort={sort} onSort={toggle}>Address</SortTh>
                    <SortTh k="owner" sort={sort} onSort={toggle}>Owner</SortTh>
                    <SortTh k="role" sort={sort} onSort={toggle}>Role</SortTh>
                    <SortTh k="live" sort={sort} onSort={toggle}>State</SortTh>
                    <SortTh k="since" sort={sort} onSort={toggle}>Since</SortTh>
                  </tr>
                </thead>
                <tbody>
                  {leases.map((l) => (
                    <tr key={l.addr + l.owner}>
                      <td>{l.addr}</td>
                      <td>{l.owner}</td>
                      <td>{l.role || '—'}</td>
                      <td><Dot status={l.live ? 'running' : 'stopped'} /> {l.live ? 'in use' : 'kept'}</td>
                      <td>{l.since ? new Date(l.since).toLocaleString() : '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          <p className={ui.empty} style={{ padding: '0.5rem 0 0' }}>A kept lease belongs to a stopped gNB: it gets the same address back on its next deploy.</p>
        </Section>
      </>
    )
    : <p className={ui.empty}>No pool selected.</p>

  return (
    <AppLayout crumbs={[{ label: 'Addresses' }]} fill>
      <Workbench
        title="Addresses"
        status={<><span>pools <b>{pools.length}</b></span><span>leases <b>{pools.reduce((n, p) => n + p.leases.length, 0)}</b></span>{warnings.length > 0 && <span>warnings <b>{warnings.length}</b></span>}</>}
        body={<Split storageKey="frulab-split-addresses" defaultRatio={0.45} left={list} right={detail} leftLabel="Address pools" rightLabel="Pool detail" />}
      />
    </AppLayout>
  )
}
