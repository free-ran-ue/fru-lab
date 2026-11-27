import type { ReactNode } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import AppLayout from '../../components/layout/AppLayout'
import TopologySchematic from '../../lab/TopologySchematic'
import { StatusSwatch, StatusTag } from '../../lab/StatusTag'
import { getStatusMeta } from '../../lab/statusMeta'
import { SLICE1_SD, SLICE2_SD, templateLabel, useLab } from '../../lab/useLab'
import type { NodeStatus } from '../../lab/types'
import styles from '../../lab/lab.module.css'

const LEGEND: NodeStatus[] = ['running', 'deploying', 'stopped', 'unhealthy']
// how many UE rows the free-ue column lists before "+N more"
const UE_PREVIEW = 8

const NODE_PAGE = { core: '/core-network', gnb: '/free-ran', ue: '/free-ue' } as const

interface ColumnProps {
  title: string
  to: string
  status: NodeStatus
  statusText?: string
  rows: { k: string; v: ReactNode; mono?: boolean }[]
  children?: ReactNode
}

// One read-only column per lab page: its status, key facts, a short list,
// and a link to the page where it is deployed.
function Column({ title, to, status, statusText, rows, children }: ColumnProps) {
  return (
    <section className={styles.panel} aria-label={title}>
      <div className={styles.ph}>
        <h2>{title}</h2>
        <StatusTag status={status} text={statusText} />
        <Link to={to} className={styles.open}>Open {title} →</Link>
      </div>
      <dl className={styles.kv}>
        {rows.map((row) => (
          <div key={row.k} className={styles.kvRow}>
            <dt>{row.k}</dt>
            <dd className={row.mono ? 'num' : undefined}>{row.v}</dd>
          </div>
        ))}
      </dl>
      {children}
    </section>
  )
}

// DashboardPage is the lab at a glance. It deploys nothing: the topology
// and the three columns follow the Core Network, free-ran and free-ue
// pages, and each links to its page.
export default function DashboardPage() {
  const navigate = useNavigate()
  const lab = useLab()
  const { free5gc, nodes, ue, ueCounts } = lab
  const template = lab.runningTemplate ?? 'basic'
  const nfs = free5gc.networkFunctions

  const gnbRows = lab.isTwoSlice
    ? [
      { name: 'slice 1', sd: SLICE1_SD, status: lab.gnbSlice1.node.status },
      { name: 'slice 2', sd: SLICE2_SD, status: lab.gnbSlice2.node.status },
    ]
    : [{ name: 'gNB', sd: '—', status: lab.gnb.node.status }]

  // up UEs first, then the rest
  const ueRows = [...ue.rows].sort((a, b) => Number(a.status === 'stopped') - Number(b.status === 'stopped'))

  return (
    <AppLayout crumbs={[{ label: 'Dashboard' }]}>
      <section className={styles.pageHead}>
        <h1 className={styles.title}>Lab dashboard</h1>
        <span className={styles.subtitle}>free5GC core network and the free-ran-ue gNB / UE simulator</span>
      </section>

      <section className={styles.meta} aria-label="Lab status">
        <div><span>Core Network</span><b><StatusTag status={nodes.core.status} /><em>{lab.runningTemplate ? templateLabel(template) : 'not deployed'}</em></b></div>
        <div><span>free-ran</span><b><StatusTag status={nodes.gnb.status} /><em>{lab.isTwoSlice ? 'slice1 + slice2' : 'gNB'}</em></b></div>
        <div><span>free-ue</span><b><StatusTag status={nodes.ue.status} text={`${ueCounts.running} / ${ueCounts.total}`} /><em>running / subscribers</em></b></div>
        <div><span>Healthy NFs</span><b><span className="num">{nfs.length ? `${lab.healthyNfs} / ${nfs.length}` : '—'}</span></b></div>
      </section>

      <section className={styles.panel} aria-label="Network topology">
        <div className={styles.ph}>
          <h2>Network topology</h2>
          {LEGEND.map((status) => (
            <span key={status} className={styles.legend}><StatusSwatch status={status} />{getStatusMeta(status).text}</span>
          ))}
          <span className={styles.aux}>click a node to open its page</span>
        </div>
        <TopologySchematic
          template={template}
          templateLabel={templateLabel(template)}
          coreStatus={nodes.core.status}
          coreNfs={nfs}
          gnbStatus={nodes.gnb.status}
          gnbTemplate={lab.isTwoSlice ? 'ulcl-2slice template' : 'basic template'}
          gnbSlices={lab.gnbSlices}
          ueStatus={nodes.ue.status}
          ueRows={ue.rows}
          selection={null}
          onSelect={(selection) => navigate(NODE_PAGE[selection.node])}
        />
      </section>

      <div className={styles.trio}>
        <Column
          title="Core Network"
          to="/core-network"
          status={nodes.core.status}
          rows={[
            { k: 'Template', v: lab.runningTemplate ? templateLabel(template) : '—' },
            { k: 'Network functions', v: nfs.length ? `${lab.healthyNfs} / ${nfs.length} healthy` : '—', mono: true },
            { k: 'Subscribers', v: <Link to="/core-network/subscribers">{ueCounts.total}</Link>, mono: true },
            { k: 'Last deployed', v: nodes.core.lastDeployed, mono: true },
          ]}
        >
          {nfs.length > 0 && (
            <table className={styles.tbl}>
              <thead><tr><th>NF</th><th>Status</th></tr></thead>
              <tbody>
                {nfs.map((nf) => (
                  <tr key={nf.name}><td className="num">{nf.name}</td><td><StatusTag status={nf.status} /></td></tr>
                ))}
              </tbody>
            </table>
          )}
        </Column>

        <Column
          title="free-ran"
          to="/free-ran"
          status={nodes.gnb.status}
          rows={[
            { k: 'Template', v: lab.isTwoSlice ? 'ulcl-2slice' : 'basic' },
            { k: 'UEs attached', v: String(lab.upUes.length), mono: true },
            { k: 'Last deployed', v: nodes.gnb.lastDeployed, mono: true },
          ]}
        >
          <table className={styles.tbl}>
            <thead><tr><th>gNB</th><th>SD</th><th>Status</th></tr></thead>
            <tbody>
              {gnbRows.map((row) => (
                <tr key={row.name}><td>{row.name}</td><td className="num">{row.sd}</td><td><StatusTag status={row.status} /></td></tr>
              ))}
            </tbody>
          </table>
        </Column>

        <Column
          title="free-ue"
          to="/free-ue"
          status={nodes.ue.status}
          statusText={`${ueCounts.running} / ${ueCounts.total}`}
          rows={[
            { k: 'Running', v: String(ueCounts.running), mono: true },
            { k: 'Starting', v: String(ueCounts.starting), mono: true },
            { k: 'Unhealthy', v: String(ueCounts.unhealthy), mono: true },
          ]}
        >
          {ueRows.length === 0 ? (
            <p className={styles.more}>No subscribers yet.</p>
          ) : (
            <>
              <table className={styles.tbl}>
                <thead><tr><th>UE ID</th><th>Status</th></tr></thead>
                <tbody>
                  {ueRows.slice(0, UE_PREVIEW).map((row) => (
                    <tr key={row.ueId}><td className="num">{row.ueId}</td><td><StatusTag status={row.status} /></td></tr>
                  ))}
                </tbody>
              </table>
              {ueRows.length > UE_PREVIEW && <p className={styles.more}>+{ueRows.length - UE_PREVIEW} more on the free-ue page</p>}
            </>
          )}
        </Column>
      </div>
    </AppLayout>
  )
}
