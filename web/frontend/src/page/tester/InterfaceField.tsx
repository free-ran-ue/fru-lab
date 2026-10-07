import type { TesterFieldError, TesterInterface } from '../../api'
import styles from './tester.module.css'

interface InterfaceFieldProps {
  label: string
  path: string
  value: string
  errors: TesterFieldError[]
  // the host's interfaces; null while loading, undefined if they could not be loaded
  interfaces: TesterInterface[] | null | undefined
  onChange: (path: string, value: string) => void
  onRefresh: () => void
}

// menuOrder lists interfaces that are up first, then by name, loopback
// last. Container veths are left out (there can be dozens) unless one is
// the current choice.
function menuOrder(list: TesterInterface[], current: string): TesterInterface[] {
  const rank = (i: TesterInterface) => (i.name === 'lo' ? 2 : i.state === 'up' || i.state === 'unknown' ? 0 : 1)
  return list
    .filter((i) => i.kind !== 'veth' || i.name === current)
    .sort((a, b) => rank(a) - rank(b) || a.name.localeCompare(b.name))
}

// InterfaceField picks one of the host's interfaces from a menu. If the
// list could not be loaded it falls back to typing the name.
export default function InterfaceField({ label, path, value, errors, interfaces, onChange, onRefresh }: InterfaceFieldProps) {
  const message = errors.find((e) => e.field === path)?.message
  const className = `${styles.input} ${message ? styles.inputError : ''}`
  const known = interfaces?.some((i) => i.name === value) ?? false

  return (
    <div className={styles.field}>
      <label htmlFor={path}>{label}</label>
      {interfaces === undefined ? (
        <input id={path} className={className} type="text" value={value} onChange={(e) => onChange(path, e.target.value)} />
      ) : (
        <select id={path} className={className} value={value} disabled={interfaces === null}
          onChange={(e) => onChange(path, e.target.value)}>
          {interfaces === null && <option value={value}>Loading interfaces…</option>}
          {interfaces !== null && value === '' && <option value="">Choose an interface</option>}
          {interfaces !== null && value !== '' && !known && <option value={value}>{value} — not on this host</option>}
          {interfaces !== null && menuOrder(interfaces, value).map((i) => (
            <option key={i.name} value={i.name}>{i.name}</option>
          ))}
        </select>
      )}
      <p className={styles.fieldHint}>
        {interfaces === undefined && <>Could not list this host&apos;s interfaces; type the name. </>}
        {interfaces !== null && (
          <button type="button" className={styles.linkButton} onClick={onRefresh}>Refresh list</button>
        )}
      </p>
      {message && <p className={styles.fieldError}>{message}</p>}
    </div>
  )
}
