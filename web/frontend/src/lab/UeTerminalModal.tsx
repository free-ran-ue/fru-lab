import { useEffect, useRef } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { buildUeTerminalUrl } from './terminalSocket'
import styles from './terminal-modal.module.css'

interface UeTerminalModalProps {
  instance: string | null
  onClose: () => void
}

export default function UeTerminalModal({ instance, onClose }: UeTerminalModalProps) {
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!instance || !containerRef.current) return

    // xterm paints to a canvas, so it needs concrete colours rather than
    // CSS variables - read the chrome token, falling back to its value.
    const rootStyle = getComputedStyle(document.documentElement)
    const chrome = rootStyle.getPropertyValue('--chrome').trim() || '#13122B'
    const term = new Terminal({
      convertEol: true,
      fontSize: Math.round(13 * ((parseFloat(rootStyle.fontSize) || 16) / 16)),
      fontFamily: "'IBM Plex Mono', ui-monospace, SFMono-Regular, Menlo, monospace",
      theme: { background: chrome, foreground: '#D9D7F0', cursor: '#D9D7F0' },
    })
    const fitAddon = new FitAddon()
    term.loadAddon(fitAddon)
    term.open(containerRef.current)
    fitAddon.fit()

    const socket = new WebSocket(buildUeTerminalUrl(instance))
    socket.binaryType = 'arraybuffer'

    function sendResize() {
      if (socket.readyState !== WebSocket.OPEN) return
      socket.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
    }

    socket.onopen = () => {
      fitAddon.fit()
      sendResize()
    }
    socket.onmessage = (event) => {
      if (event.data instanceof ArrayBuffer) {
        term.write(new Uint8Array(event.data))
      } else {
        term.write(event.data)
      }
    }
    socket.onerror = () => {
      term.write('\r\n[connection error]\r\n')
    }
    socket.onclose = () => {
      term.write('\r\n[disconnected]\r\n')
    }

    const dataDisposable = term.onData((data) => {
      if (socket.readyState === WebSocket.OPEN) {
        socket.send(data)
      }
    })

    function handleResize() {
      fitAddon.fit()
      sendResize()
    }
    window.addEventListener('resize', handleResize)

    return () => {
      window.removeEventListener('resize', handleResize)
      dataDisposable.dispose()
      socket.close()
      term.dispose()
    }
  }, [instance])

  if (!instance) return null

  return (
    <div className={styles.overlay}>
      <div className={styles.modal} role="dialog" aria-modal="true" aria-label={`Terminal — ${instance}`}>
        <div className={styles.header}>
          <span className={styles.title}>Terminal <span className={styles.instance}>{instance}</span></span>
          <button type="button" className={styles.closeButton} onClick={onClose} aria-label="Close terminal">✕</button>
        </div>
        <div className={styles.body}>
          <div ref={containerRef} className={styles.terminal} />
        </div>
      </div>
    </div>
  )
}
