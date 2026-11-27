import { useEffect, useRef } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { buildTerminalUrl } from '../terminalSocket'
import styles from './console.module.css'

// TerminalTab is a shell inside a running UE's or gNB's container
// (`instance` is the node id: ue:<imsi> or gnb-<profileId>). When the shell
// ends (exit, or the UE stops) it calls onExit, so its dock can close it.
export default function TerminalTab({ instance, onExit }: { instance: string | null; onExit?: () => void }) {
  const ref = useRef<HTMLDivElement>(null)
  const exitRef = useRef(onExit)
  exitRef.current = onExit

  useEffect(() => {
    if (!instance || !ref.current) return
    const css = getComputedStyle(document.documentElement)
    const token = (name: string) => css.getPropertyValue(name).trim()
    const term = new Terminal({
      convertEol: true,
      fontSize: 14,
      fontFamily: "'IBM Plex Mono', ui-monospace, SFMono-Regular, Menlo, monospace",
      theme: { background: token('--chrome'), foreground: token('--chrome-text'), cursor: token('--chrome-accent') },
    })
    const fit = new FitAddon()
    term.loadAddon(fit)
    term.open(ref.current)
    fit.fit()
    term.focus()

    const socket = new WebSocket(buildTerminalUrl(instance))
    socket.binaryType = 'arraybuffer'
    const sendResize = () => {
      if (socket.readyState === WebSocket.OPEN) socket.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
    }
    socket.onopen = () => { fit.fit(); sendResize() }
    socket.onmessage = (e) => term.write(e.data instanceof ArrayBuffer ? new Uint8Array(e.data) : e.data)
    // closing from here (unmount) is not the shell ending
    let ours = false
    socket.onclose = () => {
      if (ours) return
      term.write('\r\n[session ended]\r\n')
      if (exitRef.current) exitRef.current()
    }
    const input = term.onData((d) => { if (socket.readyState === WebSocket.OPEN) socket.send(d) })

    const ro = new ResizeObserver(() => { fit.fit(); sendResize() })
    ro.observe(ref.current)
    return () => {
      ro.disconnect()
      input.dispose()
      ours = true
      socket.close()
      term.dispose()
    }
  }, [instance])

  if (!instance) return <p className={styles.empty}>Select a running UE or gNB to open a shell in its container.</p>
  return <div className={styles.term} ref={ref} />
}
