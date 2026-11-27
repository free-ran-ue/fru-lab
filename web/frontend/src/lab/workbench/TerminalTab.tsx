import { useEffect, useRef } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { buildUeTerminalUrl } from '../terminalSocket'
import styles from './console.module.css'

// TerminalTab is a shell inside a running UE's container.
export default function TerminalTab({ instance }: { instance: string | null }) {
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!instance || !ref.current) return
    const css = getComputedStyle(document.documentElement)
    const token = (name: string) => css.getPropertyValue(name).trim()
    const term = new Terminal({
      convertEol: true,
      fontSize: 12,
      fontFamily: "'IBM Plex Mono', ui-monospace, SFMono-Regular, Menlo, monospace",
      theme: { background: token('--chrome'), foreground: token('--chrome-text'), cursor: token('--chrome-accent') },
    })
    const fit = new FitAddon()
    term.loadAddon(fit)
    term.open(ref.current)
    fit.fit()
    term.focus()

    const socket = new WebSocket(buildUeTerminalUrl(instance))
    socket.binaryType = 'arraybuffer'
    const sendResize = () => {
      if (socket.readyState === WebSocket.OPEN) socket.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
    }
    socket.onopen = () => { fit.fit(); sendResize() }
    socket.onmessage = (e) => term.write(e.data instanceof ArrayBuffer ? new Uint8Array(e.data) : e.data)
    socket.onclose = () => term.write('\r\n[disconnected]\r\n')
    const input = term.onData((d) => { if (socket.readyState === WebSocket.OPEN) socket.send(d) })

    const ro = new ResizeObserver(() => { fit.fit(); sendResize() })
    ro.observe(ref.current)
    return () => {
      ro.disconnect()
      input.dispose()
      socket.close()
      term.dispose()
    }
  }, [instance])

  if (!instance) return <p className={styles.empty}>Select a running UE to open a shell in its container.</p>
  return <div className={styles.term} ref={ref} />
}
