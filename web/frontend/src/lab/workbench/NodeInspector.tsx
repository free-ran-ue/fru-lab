import type { ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { useLab } from '../LabContext'
import { useEvents } from '../console/EventsContext'
import { STAGE_LABEL } from '../console/events'
import { nodeInfo } from '../nodeInfo'
import { Actions, Button, Dot, InspectorHead, KV, Section, ui } from '../LabUi'

const VIEW: Record<string, string> = { '/core-network/lab': 'free5GC', '/free-ran': 'free-ran', '/free-ue': 'free-ue' }

// NodeInspector shows any canvas node read-only, with a link to the view
// that manages it and a button for its logs.
export default function NodeInspector({ id, onLogs, extra }: { id: string; onLogs?: () => void; extra?: ReactNode }) {
  const lab = useLab()
  const navigate = useNavigate()
  const { events, ueLive } = useEvents()
  const info = nodeInfo(lab, id)
  if (!info) return <p className={ui.empty}>Nothing selected.</p>
  const live = info.ueId ? ueLive[info.ueId] : undefined
  const rows = [...info.rows]
  if (live && info.status !== 'stopped') {
    if (live.stage) rows.unshift({ k: 'Stage', v: STAGE_LABEL[live.stage] })
    if (live.ip) rows.splice(1, 0, { k: 'UE IP', v: live.ip })
  }
  const recent = events.filter((e) => e.node === id || (info.ueId && e.ue?.id === info.ueId)).slice(-6).reverse()

  return (
    <>
      <InspectorHead icon={info.icon} name={info.name} sub={info.sub} status={info.status} statusText={info.statusText} />
      <Section>
        <KV rows={rows} />
        <Actions>
          {info.page && <Button small onClick={() => navigate(info.page!)}>Open in {VIEW[info.page] ?? info.name}</Button>}
          {onLogs && info.kind !== 'dn' && info.kind !== 'fleet' && info.logs !== false && <Button small onClick={onLogs}>Logs</Button>}
        </Actions>
      </Section>
      {info.services && info.services.length > 0 && (
        <Section title="Services">
          <ul className={ui.list}>{info.services.map((s) => <li key={s.name}><Dot status={s.status} />{s.name}<span>{s.status}</span></li>)}</ul>
        </Section>
      )}
      {extra}
      {recent.length > 0 && (
        <Section title="Recent events">
          <ul className={ui.list}>
            {recent.map((e) => <li key={e.id}><Dot status={e.level === 'error' ? 'unhealthy' : e.level === 'ok' ? 'running' : 'stopped'} />{e.text}<span>{new Date(e.t).toISOString().slice(11, 19)}</span></li>)}
          </ul>
        </Section>
      )}
    </>
  )
}
