import { useEffect, useState } from 'react'
import type { CoreNetwork, HostInterface } from '../../api'
import { api } from '../../apiClient'
import { Button } from '../LabUi'
import { ps } from '../profiles/parts'

const IPV4 = /^(25[0-5]|2[0-4]\d|1?\d?\d)(\.(25[0-5]|2[0-4]\d|1?\d?\d)){3}$/
const NAME = /^[A-Za-z0-9][A-Za-z0-9 ._-]{0,31}$/
const DNN = /^[A-Za-z0-9][A-Za-z0-9.-]{0,62}$/

export const NEW_CORE: CoreNetwork = { name: '', n2Ip: '', n2Interface: '', n3Ip: '', n3Interface: '', dns: [{ name: 'internet' }] }

interface Props {
  // the core being edited; one without an id is new
  value: CoreNetwork
  // the other cores' names, for the unique-name check
  takenNames: string[]
  onSave: (n: CoreNetwork) => Promise<void>
}

// CoreNetworkForm adds or edits a core outside the lab: its name, the
// AMF's N2 address and the UPF's N3 address with the host interface that
// reaches each, and one or more data networks. Save sits on a bar at the
// top, like the gNB and UE config tabs.
export default function CoreNetworkForm({ value, takenNames, onSave }: Props) {
  const [draft, setDraft] = useState<CoreNetwork>(value)
  const [saving, setSaving] = useState(false)
  const [hosts, setHosts] = useState<HostInterface[]>([])
  useEffect(() => { setDraft(value) }, [value])
  useEffect(() => { api.hostInterfaceList().then((r) => setHosts(r.data)).catch(() => {}) }, [])

  const set = (patch: Partial<CoreNetwork>) => setDraft((d) => ({ ...d, ...patch }))
  const setDn = (i: number, patch: Partial<CoreNetwork['dns'][number]>) => set({ dns: draft.dns.map((d, j) => (j === i ? { ...d, ...patch } : d)) })
  const isNew = !draft.id
  const dirty = JSON.stringify(draft) !== JSON.stringify(value)

  // what is wrong, field by field
  const name = draft.name.trim()
  const problems: Record<string, string> = {}
  if (!NAME.test(name)) problems.name = '1-32 letters, digits, spaces, dots, dashes or underscores'
  else if (name.toLowerCase() === 'free5gc') problems.name = 'free5GC is the lab core\'s name'
  else if (takenNames.some((t) => t.toLowerCase() === name.toLowerCase())) problems.name = 'Another core network has this name'
  if (!IPV4.test(draft.n2Ip.trim())) problems.n2Ip = 'An IPv4 address'
  if (!IPV4.test(draft.n3Ip.trim())) problems.n3Ip = 'An IPv4 address'
  if (draft.dns.length === 0) problems.dns = 'Add at least one data network'
  const seen = new Set<string>()
  draft.dns.forEach((d, i) => {
    const n = d.name.trim().toLowerCase()
    if (!DNN.test(d.name.trim())) problems[`dn${i}`] = 'A DNN: letters, digits, dots, dashes'
    else if (seen.has(n)) problems[`dn${i}`] = 'Listed twice'
    seen.add(n)
  })
  const count = Object.keys(problems).length

  async function save() {
    setSaving(true)
    try {
      await onSave({ ...draft, dns: draft.dns.map((d) => ({ name: d.name.trim() })) })
    } catch {
      // the page shows the error
    } finally {
      setSaving(false)
    }
  }

  const field = (key: 'name' | 'n2Ip' | 'n3Ip', label: string, hint?: string) => (
    <div className={`${ps.field} ${key === 'name' ? ps.wide : ''}`}>
      <label htmlFor={`core-${key}`}>{label}</label>
      <input id={`core-${key}`} className={`${ps.input} ${problems[key] && draft[key] ? ps.inputBad : ''}`} value={draft[key] ?? ''} onChange={(e) => set({ [key]: e.target.value })} />
      {problems[key] && draft[key] ? <span className={ps.fieldHint}>{problems[key]}</span> : hint && <span className={ps.fieldHint}>{hint}</span>}
    </div>
  )
  const ifaceField = (key: 'n2Interface' | 'n3Interface', label: string) => (
    <div className={ps.field}>
      <label htmlFor={`core-${key}`}>{label}</label>
      <input id={`core-${key}`} className={ps.input} list="core-host-ifaces" placeholder="optional" value={draft[key] ?? ''} onChange={(e) => set({ [key]: e.target.value })} />
      <span className={ps.fieldHint}>The host interface that reaches it</span>
    </div>
  )

  return (
    <div className={ps.panel}>
      <div className={ps.saveBar}>
        <span className={ps.saveState}>{count ? `${count} field${count === 1 ? '' : 's'} to fix` : isNew ? 'Not saved yet' : dirty ? 'Unsaved changes' : 'Saved'}</span>
        {!isNew && dirty && <Button small onClick={() => setDraft(value)}>Discard</Button>}
        <Button kind="primary" small busy={saving} disabled={count > 0 || (!dirty && !isNew)} title={count ? 'Fix the fields first' : undefined} onClick={save}>{isNew ? 'Add network' : 'Save'}</Button>
      </div>
      <datalist id="core-host-ifaces">
        {hosts.map((h) => <option key={h.name} value={h.name}>{h.addresses.join(', ')}</option>)}
      </datalist>
      <div className={ps.section}>
        {isNew && <p className={ps.banner}>A core outside the lab. Its name is unique and gets its own page under Core Network.</p>}
        <div className={ps.grid}>
          {field('name', 'Name')}
          {field('n2Ip', 'N2 IP (AMF)')}
          {ifaceField('n2Interface', 'N2 interface')}
          {field('n3Ip', 'N3 IP (UPF)')}
          {ifaceField('n3Interface', 'N3 interface')}
        </div>
      </div>
      <div className={ps.section}>
        <p className={ps.sectionTitle}>Data networks</p>
        {problems.dns && <p className={`${ps.banner} ${ps.bannerWarn}`}>{problems.dns}</p>}
        {draft.dns.map((dn, i) => (
          <div key={i} className={ps.grid} style={{ marginBottom: '0.5rem' }}>
            <div className={ps.field}>
              <label htmlFor={`dn-name-${i}`}>DNN</label>
              <input id={`dn-name-${i}`} className={`${ps.input} ${problems[`dn${i}`] ? ps.inputBad : ''}`} value={dn.name} onChange={(e) => setDn(i, { name: e.target.value })} />
              {problems[`dn${i}`] && <span className={ps.fieldHint}>{problems[`dn${i}`]}</span>}
            </div>
            <div className={ps.field}>
              <label>&nbsp;</label>
              <Button small disabled={draft.dns.length === 1} title={draft.dns.length === 1 ? 'A core needs at least one' : undefined} onClick={() => set({ dns: draft.dns.filter((_, j) => j !== i) })}>Remove</Button>
            </div>
          </div>
        ))}
        <Button small onClick={() => set({ dns: [...draft.dns, { name: '' }] })}>+ Data network</Button>
      </div>
    </div>
  )
}
