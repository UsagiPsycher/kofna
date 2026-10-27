import {useEffect, useRef} from 'react'
import {useI18n} from './i18n'
import {LogLine} from './types'

const tags: Record<LogLine['level'], string> = {
  cmd: '$',
  out: '›',
  info: 'i',
  warn: '!',
  error: '×',
}

export function LogPanel({lines, open, onToggle, onClear}: {
  lines: LogLine[]
  open: boolean
  onToggle: () => void
  onClear: () => void
}) {
  const end = useRef<HTMLDivElement>(null)
  const {t} = useI18n()
  useEffect(() => {
    if (open) end.current?.scrollIntoView({block: 'end'})
  }, [lines, open])

  return (
    <section className={open ? 'log open' : 'log'}>
      <header className="log-head">
        <button className="log-toggle" onClick={onToggle} aria-expanded={open}>
          <span className="chev" aria-hidden>{open ? '▾' : '▸'}</span>
          {t.log}
          <span className="count">{lines.length}</span>
        </button>
        {open && lines.length > 0 && <button className="btn ghost small" onClick={onClear}>{t.clear}</button>}
      </header>
      {open && (
        <div className="log-body">
          {lines.length === 0 && <p className="log-empty">{t.logEmpty}</p>}
          {lines.map((l, i) => (
            <div key={i} className={`log-line ${l.level}`}>
              <span className="log-at">{l.at}</span>
              <span className="log-tag">{tags[l.level]}</span>
              <span className="log-text">{l.text}</span>
            </div>
          ))}
          <div ref={end}/>
        </div>
      )}
    </section>
  )
}
