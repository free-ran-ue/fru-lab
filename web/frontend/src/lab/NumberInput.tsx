import { useEffect, useState, type CSSProperties } from 'react'

interface Props {
  value: number
  onChange: (v: number) => void
  min: number
  max: number
  // whole numbers only
  integer?: boolean
  className?: string
  style?: CSSProperties
  id?: string
  disabled?: boolean
  'aria-label'?: string
  title?: string
}

// NumberInput is a number field that can be cleared and retyped: what is
// typed is kept as text, every valid number in [min, max] is applied at
// once, and leaving the field empty or invalid puts back the last value
// that was applied.
export default function NumberInput({ value, onChange, min, max, integer, ...rest }: Props) {
  const [text, setText] = useState(String(value))
  const [editing, setEditing] = useState(false)
  // follow the value from outside while not being edited
  useEffect(() => { if (!editing) setText(String(value)) }, [value, editing])

  const parse = (t: string): number | null => {
    if (t.trim() === '') return null
    const n = Number(t)
    if (!Number.isFinite(n) || n < min || n > max || (integer && !Number.isInteger(n))) return null
    return n
  }

  return (
    <input
      {...rest}
      type="number"
      inputMode={integer ? 'numeric' : 'decimal'}
      min={min}
      max={max}
      step={integer ? 1 : 'any'}
      value={text}
      onFocus={() => setEditing(true)}
      onChange={(e) => {
        setText(e.target.value)
        const n = parse(e.target.value)
        if (n !== null) onChange(n)
      }}
      onBlur={() => {
        setEditing(false)
        if (parse(text) === null) setText(String(value))
      }}
    />
  )
}
