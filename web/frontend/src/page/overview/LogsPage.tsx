import { useSearchParams } from 'react-router-dom'
import AppLayout from '../../components/layout/AppLayout'
import { sourceKey, type LogSource } from '../../lab/console/stream'
import LogsTab from '../../lab/workbench/LogsTab'
import Workbench from '../../lab/workbench/Workbench'
import { sourceFromQuery, useLogSources } from '../../lab/workbench/logSources'
import styles from './overview-pages.module.css'

// LogsPage is the Overview's live log view: pick a source (the core or one
// NF, a gNB, a UE) and follow it. Other pages' Logs buttons land here with
// the source already picked (?target=&instance=&service=).
export default function LogsPage() {
  const [params, setParams] = useSearchParams()
  const options = useLogSources()
  const source = sourceFromQuery(params)
  const label = source ? options.find((o) => sourceKey(o.source) === sourceKey(source))?.label : undefined

  function pick(s: LogSource) {
    const q = new URLSearchParams({ target: s.target })
    if (s.instance) q.set('instance', s.instance)
    if (s.service) q.set('service', s.service)
    setParams(q, { replace: true })
  }

  return (
    <AppLayout crumbs={[{ label: 'Logs' }]} fill>
      <Workbench
        title="Logs"
        status={<><span>sources <b>{options.length}</b></span>{label && <span>following <b>{label}</b></span>}</>}
        body={<div className={styles.panel}><LogsTab options={options} source={source} onSource={pick} /></div>}
      />
    </AppLayout>
  )
}
