import type { TesterSystemInfo } from '../../api'
import { parseCpus } from './cpus'
import styles from './tester.module.css'

// The CPU views of the Status page: where each CPU sits and which side it
// is on, and how busy each one is.

// usageColor runs from green (idle) through yellow (half) to red (busy).
export function usageColor(busy: number): { background: string, borderColor: string } {
  const hue = 120 - 1.2 * Math.min(100, Math.max(0, busy))
  return { background: `hsl(${hue} 75% 82%)`, borderColor: `hsl(${hue} 60% 45%)` }
}

// nodesOf groups the host's CPUs by NUMA node.
export function nodesOf(info: TesterSystemInfo): { label: string, cpus: number[] }[] {
  const online = parseCpus(info.topology.online) ?? parseCpus(info.topology.allowed) ?? []
  return info.topology.nodes.length > 0
    ? info.topology.nodes.map((n) => ({ label: `NUMA node ${n.node}`, cpus: parseCpus(n.cpus) ?? [] }))
    : [{ label: 'CPUs', cpus: online }]
}

// CpuMap shows every CPU by NUMA node, coloured by side; clicking one
// moves it: unassigned → control plane → data plane → unassigned.
export function CpuMap({ info, cp, dp, irq, onToggle, disabled }: {
  info: TesterSystemInfo, cp: number[], dp: number[], irq: number[]
  onToggle: (cpu: number) => void, disabled: boolean
}) {
  const allowed = parseCpus(info.topology.allowed) ?? []
  const nodes = nodesOf(info)
  return (
    <div className={styles.cpuMap}>
      {nodes.map((n) => (
        <div key={n.label} className={styles.cpuNode}>
          <div className={styles.cpuNodeHead}>
            <b>{n.label}</b>
            <span className={styles.pingNote}>
              {info.topology.interfaces.filter((i) => `NUMA node ${i.numaNode}` === n.label).map((i) => i.name).join(', ')}
            </span>
          </div>
          <div className={styles.cpuGrid}>
            {n.cpus.map((c) => {
              const side = cp.includes(c) ? styles.cpuCp : dp.includes(c) ? styles.cpuDp : ''
              const usable = allowed.includes(c)
              return (
                <button key={c} type="button" disabled={disabled || !usable}
                  className={`${styles.cpu} ${side} ${usable ? '' : styles.cpuOff}`}
                  title={`CPU ${c}${usable ? '' : ' — not available to fru-tester'}${irq.includes(c) ? ' — NIC interrupts' : ''}`}
                  onClick={() => onToggle(c)}>
                  {c}{irq.includes(c) && <i className={styles.cpuIrq} />}
                </button>
              )
            })}
          </div>
        </div>
      ))}
    </div>
  )
}

// CpuUsage is the host's load per CPU, live, by NUMA node, with the
// average of each side of the saved allocation.
export function CpuUsage({ info, load, cp, dp }: {
  info: TesterSystemInfo, load: Map<number, number>, cp: number[], dp: number[]
}) {
  const avg = (cpus: number[]) => {
    const v = cpus.map((c) => load.get(c)).filter((b): b is number => b !== undefined)
    return v.length === 0 ? null : v.reduce((a, b) => a + b, 0) / v.length
  }
  const summary = [
    { label: 'All CPUs', value: avg([...load.keys()]) },
    { label: 'Control plane', value: cp.length > 0 ? avg(cp) : null, empty: 'not pinned' },
    { label: 'Data plane', value: dp.length > 0 ? avg(dp) : null, empty: 'not pinned' },
  ]
  return (
    <>
      <div className={styles.usageSummary}>
        {summary.map((s) => (
          <div key={s.label} className={styles.usageStat}>
            <span>{s.label}</span>
            <b style={s.value === null ? undefined : { color: `hsl(${120 - 1.2 * s.value} 65% 35%)` }}>
              {s.value === null ? (load.size === 0 ? '—' : s.empty ?? '—') : `${s.value.toFixed(0)}%`}
            </b>
          </div>
        ))}
      </div>
      <div className={styles.cpuMap}>
        {nodesOf(info).map((n) => (
          <div key={n.label} className={styles.cpuNode}>
            <div className={styles.cpuNodeHead}>
              <b>{n.label}</b>
              <span className={styles.pingNote}>average {avg(n.cpus)?.toFixed(0) ?? '—'}%</span>
            </div>
            <div className={styles.usageGrid}>
              {n.cpus.map((c) => {
                const busy = load.get(c)
                const side = cp.includes(c) ? 'CP' : dp.includes(c) ? 'DP' : ''
                return (
                  <div key={c} className={styles.usageCpu} style={busy === undefined ? undefined : usageColor(busy)}
                    title={`CPU ${c}${side ? ` (${side === 'CP' ? 'control plane' : 'data plane'})` : ''}: ${busy === undefined ? 'no data' : `${busy.toFixed(1)}% busy`}`}>
                    <span className={styles.usageNum}>{c}{side && <em>{side}</em>}</span>
                    <b>{busy === undefined ? '—' : `${Math.round(busy)}%`}</b>
                  </div>
                )
              })}
            </div>
          </div>
        ))}
      </div>
    </>
  )
}
