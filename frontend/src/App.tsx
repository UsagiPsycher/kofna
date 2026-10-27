import {useCallback, useEffect, useRef, useState} from 'react'
import * as api from '../wailsjs/go/main/App'
import {EventsOn} from '../wailsjs/runtime/runtime'
import {useConfirm} from './Confirm'
import {DistroView, Head, Tab} from './DistroView'
import {Explorer} from './Explorer'
import {LogPanel} from './LogPanel'
import {bytes, duration, errorText, isRunning, time} from './format'
import {dictionaries, languages, Lang, useI18n} from './i18n'
import {useTheme} from './theme'
import {Distro, LogLine, Platform, ProgressEvent, Report} from './types'


export default function App() {
  const [platform, setPlatform] = useState<Platform | null>(null)
  const [version, setVersion] = useState('')
  const [distros, setDistros] = useState<Distro[]>([])
  const [selected, setSelected] = useState('')
  const [reports, setReports] = useState<Record<string, Report>>({})
  const [logs, setLogs] = useState<LogLine[]>([])
  const [logOpen, setLogOpen] = useState(false)
  const [busy, setBusy] = useState<{task: string; since: number} | null>(null)
  const [progress, setProgress] = useState<ProgressEvent | null>(null)
  const [error, setError] = useState('')
  const [loadError, setLoadError] = useState('')
  const [, tick] = useState(0)
  const [tab, setTab] = useState<Tab>('space')
  const tabRef = useRef<Tab>('space')
  tabRef.current = tab
  const {ask, modal} = useConfirm()
  const {t, lang, setLang} = useI18n()
  const [theme, toggleTheme] = useTheme()

  const pushLog = (level: LogLine['level'], text: string) =>
    setLogs((ls) => [...ls.slice(-1999), {at: time(), level, text}])

  useEffect(() => {
    const offs = [
      EventsOn('log', (l: LogLine) => setLogs((ls) => [...ls.slice(-1999), l])),
      EventsOn('busy', (task: string) => {
        setBusy(task ? {task, since: Date.now()} : null)
        if (task && tabRef.current === 'space') setLogOpen(true)
        else setProgress(null)
      }),
      EventsOn('progress', (p: ProgressEvent) => setProgress(p)),
    ]
    return () => offs.forEach((off) => off())
  }, [])

  // Chronomètre de l'opération en cours.
  useEffect(() => {
    if (!busy) return
    const id = window.setInterval(() => tick((n) => n + 1), 1000)
    return () => window.clearInterval(id)
  }, [busy])

  const setReport = useCallback((r: Report) => {
    setReports((rs) => ({...rs, [r.distro.name]: r}))
    setDistros((ds) => ds.map((d) => (d.name === r.distro.name ? r.distro : d)))
  }, [])

  const loadDistros = useCallback(async () => {
    try {
      const ds = (await api.Distros()) as unknown as Distro[]
      setDistros(ds)
      setLoadError('')
      setSelected((cur) => (cur && ds.some((d) => d.name === cur) ? cur : ds[0]?.name ?? ''))
      return ds
    } catch (e) {
      setLoadError(errorText(e))
      return []
    }
  }, [])

  const loadReport = useCallback(async (name: string) => {
    if (!name) return
    try {
      setReport((await api.Report(name)) as unknown as Report)
    } catch (e) {
      pushLog('error', errorText(e))
    }
  }, [setReport])

  useEffect(() => {
    api.Platform().then((p) => setPlatform(p as unknown as Platform))
    api.Version().then(setVersion)
    loadDistros()
  }, [loadDistros])

  useEffect(() => {
    loadReport(selected)
  }, [selected, loadReport])

  /** Lance une action : affiche l'erreur éventuelle, puis rafraîchit la liste. */
  const run = useCallback(async <T, >(fn: () => Promise<T>): Promise<T | undefined> => {
    setError('')
    try {
      return await fn()
    } catch (e) {
      const msg = errorText(e)
      setError(msg)
      pushLog('error', msg)
      return undefined
    } finally {
      loadDistros()
    }
  }, [loadDistros])

  const refresh = async () => {
    await loadDistros()
    await loadReport(selected)
    api.Platform().then((p) => setPlatform(p as unknown as Platform))
  }

  const shutdown = async () => {
    const ok = await ask({
      title: t.shutdownTitle,
      confirm: t.shutdownWsl,
      body: <p className="warn-box">{t.shutdownWarn}</p>,
    })
    if (!ok) return
    await run(() => api.Shutdown())
    loadReport(selected)
  }

  const report = reports[selected]
  const totalOnDisk = distros.reduce((s, d) => s + d.sizeOnDisk, 0)

  return (
    <div className="app">
      <header className="topbar">
        <div className="brand">
          <span className="logo" aria-hidden/>
          <span className="brand-name">Kofna</span>
          <span className="brand-sub">WSL Space Liberator</span>
          {platform?.wslVersion && <span className="muted">WSL {platform.wslVersion}</span>}
          {platform?.isAdmin && <span className="pill">{t.admin}</span>}
        </div>
        <div className="top-actions">
          <div className="prefs">
            <label className="lang-select">
              <span className="sr-only">{t.language}</span>
              <svg viewBox="0 0 16 16" aria-hidden><circle cx="8" cy="8" r="6.25"/><path d="M1.75 8h12.5M8 1.75c1.9 1.7 2.9 3.8 2.9 6.25S9.9 12.55 8 14.25M8 1.75C6.1 3.45 5.1 5.55 5.1 8s1 4.55 2.9 6.25"/></svg>
              <select value={lang} onChange={(e) => setLang(e.target.value as Lang)}>
                {languages.map((l) => <option key={l} value={l}>{dictionaries[l].langName}</option>)}
              </select>
            </label>
            <button className="theme-toggle" onClick={toggleTheme}
                    aria-label={theme === 'dark' ? t.themeToLight : t.themeToDark}
                    title={theme === 'dark' ? t.themeToLight : t.themeToDark}>
              {theme === 'dark' ? (
                <svg viewBox="0 0 16 16" aria-hidden><circle cx="8" cy="8" r="3"/><path d="M8 1v1.5M8 13.5V15M1 8h1.5M13.5 8H15M3.05 3.05l1.06 1.06M11.89 11.89l1.06 1.06M3.05 12.95l1.06-1.06M11.89 4.11l1.06-1.06"/></svg>
              ) : (
                <svg viewBox="0 0 16 16" aria-hidden><path d="M13.5 9.6A5.75 5.75 0 0 1 6.4 2.5a5.75 5.75 0 1 0 7.1 7.1z"/></svg>
              )}
            </button>
          </div>
          <button className="btn ghost" disabled={!!busy} onClick={refresh}>{t.refresh}</button>
          <button className="btn ghost" disabled={!!busy} onClick={shutdown}>{t.shutdownWsl}</button>
        </div>
      </header>

      {busy && (
        <div className="busy" role="status">
          <span className="spinner" aria-hidden/>
          <span className="busy-task">{busy.task}…</span>
          <span className="muted">{duration(Date.now() - busy.since)}</span>
          {progress && (
            <>
              <div className="progress">
                <div className={progress.total > 0 ? 'bar' : 'bar indeterminate'}
                     style={progress.total > 0 ? {width: `${Math.min(100, (progress.done / progress.total) * 100)}%`} : undefined}/>
              </div>
              <span className="muted num">
                {bytes(progress.done)}{progress.total > 0 && ` / ${bytes(progress.total)}`}
              </span>
              <button className="btn ghost small" onClick={() => api.Cancel()}>{t.cancel}</button>
            </>
          )}
        </div>
      )}

      <div className="body">
        <aside className="sidebar">
          <h2 className="side-title">{t.distributions}</h2>
          <ul className="distros">
            {distros.map((d) => (
              <li key={d.id}>
                <button className={d.name === selected ? 'distro active' : 'distro'} onClick={() => setSelected(d.name)}>
                  <span className={isRunning(d.state) ? 'dot on' : 'dot'} title={d.state}/>
                  <span className="distro-name">{d.name}</span>
                  <span className="distro-meta">
                    <span className="ver">WSL {d.version}</span>
                    {d.version === 2 && <span className="num">{bytes(d.sizeOnDisk)}</span>}
                  </span>
                </button>
              </li>
            ))}
          </ul>
          {distros.length > 1 && (
            <p className="side-total">{t.totalVhdx} <strong>{bytes(totalOnDisk)}</strong></p>
          )}
          {/* 0.67.0 s'affiche « 0.67 » : le .0 final n'est là que pour Windows. */}
          {version && <p className="side-version">Kofna v{version.replace(/^(\d+\.\d+)\.0$/, '$1')}</p>}
        </aside>

        <main className="main">
          {loadError && <div className="error-banner">{loadError}</div>}
          {error && (
            <div className="error-banner" role="alert">
              <span>{error}</span>
              <button className="btn ghost small" onClick={() => setError('')}>{t.close}</button>
            </div>
          )}
          {report && platform && (
            <div className="pane">
              <Head report={report} tab={report.distro.managed ? 'space' : tab} onTab={setTab}/>
              <div hidden={tab === 'explorer' && !report.distro.managed}>
                <DistroView key={report.distro.name} report={report} platform={platform} busy={!!busy}
                            run={run} ask={ask} onReport={setReport} onPlatform={setPlatform}/>
              </div>
              {!report.distro.managed && (
                <div hidden={tab !== 'explorer'}>
                  <Explorer key={report.distro.name} distro={report.distro} active={tab === 'explorer'} busy={!!busy} run={run} ask={ask}
                            onGoSpace={() => {
                              setTab('space')
                              loadReport(report.distro.name)
                            }}/>
                </div>
              )}
            </div>
          )}
        </main>
      </div>

      <LogPanel lines={logs} open={logOpen} onToggle={() => setLogOpen((o) => !o)} onClear={() => setLogs([])}/>
      {modal}
    </div>
  )
}
