import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import AppLayout from '../../components/layout/AppLayout'
import { useNotifications } from '../../hooks/useNotifications'
import { extractErrorMessage } from '../../apiClient'
import LabCanvas from '../../lab/canvas/LabCanvas'
import { ID, buildGraph } from '../../lab/canvas/graph'
import { useEvents } from '../../lab/console/EventsContext'
import { useLab } from '../../lab/LabContext'
import { Actions, Button, Dot, Hint, InspectorHead, KV, Section, Segmented, ui } from '../../lab/LabUi'
import { FREE5GC_TEMPLATE_OPTIONS, type Free5gcTemplate } from '../../lab/free5gcTemplate'
import { templateLabel } from '../../lab/useLab'
import { StatusTag } from '../../lab/StatusTag'
import Workbench from '../../lab/workbench/Workbench'
import { logsHref } from '../../lab/workbench/logSources'
import NodeInspector from '../../lab/workbench/NodeInspector'

const SHORT: Record<Free5gcTemplate, string> = { basic: 'Basic', ulcl: 'ULCL' }

// CoreNetworkPage deploys and stops free5GC. The topology expands the core
// into its NFs (a preview of the picked template while it is stopped); the
// inspector holds the deploy controls and each NF's status; the console
// follows the selected NF's log.
export default function CoreNetworkPage() {
  const navigate = useNavigate()
  const lab = useLab()
  const { ueLive } = useEvents()
  const { free5gc, nodes } = lab
  const { addError, addSuccess } = useNotifications()
  const [picked, setPicked] = useState<Free5gcTemplate>('basic')
  const [selected, setSelected] = useState<string | null>(null)

  const coreStopped = nodes.core.status === 'stopped'
  const blocked = coreStopped ? undefined : lab.blocked.coreStop
  const graph = buildGraph(lab, 'core', picked, ueLive)
  const nfs = free5gc.networkFunctions
  const template = lab.runningTemplate ?? picked

  async function toggleCore() {
    try {
      if (coreStopped) {
        await free5gc.deploy(picked)
        addSuccess(`free5GC ${SHORT[picked]} is starting`)
      } else {
        await free5gc.stop()
        addSuccess('free5GC stopped')
      }
    } catch (error) {
      addError(extractErrorMessage(error, 'Core network action failed'))
    }
  }

  const primary = (
    <Button kind={coreStopped ? 'primary' : 'danger'} busy={free5gc.isActionPending} disabled={Boolean(blocked)} title={blocked} onClick={toggleCore}>
      {coreStopped ? `Deploy ${SHORT[picked]}` : 'Stop core'}
    </Button>
  )

  const showLogs = (id: string) => {
    navigate(logsHref(id))
  }

  const coreInspector = (
    <>
      <InspectorHead icon="core" name="free5GC" sub="5G core network" status={nodes.core.status} />
      <Section title="Deployment">
        {coreStopped && (
          <div style={{ marginBottom: '0.625rem' }}>
            <Segmented value={picked} options={FREE5GC_TEMPLATE_OPTIONS.map((o) => ({ value: o.value, label: SHORT[o.value] }))} onChange={setPicked} disabled={free5gc.isActionPending} />
          </div>
        )}
        <KV rows={[
          { k: 'Template', v: `${templateLabel(template)}${coreStopped ? ' (preview)' : ''}` },
          { k: 'NFs healthy', v: nfs.length ? `${lab.healthyNfs} / ${nfs.length}` : '—' },
          { k: 'Last deployed', v: nodes.core.lastDeployed },
          { k: 'Subscribers', v: <a href="/core-network/subscribers" onClick={(e) => { e.preventDefault(); navigate('/core-network/subscribers') }}>open</a> },
        ]} />
        {blocked && <Hint>{blocked}</Hint>}
        <Actions>{primary}{!coreStopped && <Button small onClick={() => showLogs(ID.core)}>Logs (all NFs)</Button>}</Actions>
      </Section>
      {nfs.length > 0 && (
        <Section title="Network functions" aside={<span>{lab.healthyNfs}/{nfs.length}</span>}>
          <ul className={ui.list}>
            {nfs.map((nf) => (
              <li key={nf.name}>
                <button type="button" onClick={() => setSelected(ID.nf(nf.name))}><Dot status={nf.status} />{nf.name}<span style={{ marginLeft: 'auto' }}><StatusTag status={nf.status} /></span></button>
              </li>
            ))}
          </ul>
        </Section>
      )}
    </>
  )

  const inspector = !selected || selected === ID.core
    ? coreInspector
    : <NodeInspector id={selected} onLogs={() => showLogs(selected)} extra={selected.startsWith('nf:') ? <Section><Button small onClick={() => setSelected(null)}>Back to free5GC</Button></Section> : undefined} />

  return (
    <AppLayout crumbs={[{ label: 'Network functions' }]} fill>
      <Workbench
        title="Core Network"
        status={<><span>free5GC <b>{nodes.core.status}</b></span>{nfs.length > 0 && <span>NFs <b>{lab.healthyNfs}/{nfs.length}</b></span>}</>}
        canvas={(
          <LabCanvas
            label="The free5GC core: control-plane NFs on the SBI bus, AMF, SMF and the UPFs."
            {...graph}
            selectedId={selected}
            onSelect={setSelected}
          />
        )}
        inspector={inspector}
      />
    </AppLayout>
  )
}
