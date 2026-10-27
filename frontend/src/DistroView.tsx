import {ReactNode, useEffect, useState} from 'react'
import * as api from '../wailsjs/go/main/App'
import {Ask} from './Confirm'
import {bytes, delta, isRunning, time} from './format'
import {useI18n} from './i18n'
import {CompactMethod, Platform, Report} from './types'

type Run = <T>(fn: () => Promise<T>) => Promise<T | undefined>

interface Props {
  report: Report
  platform: Platform
  busy: boolean
  run: Run
  ask: Ask
  onReport: (r: Report) => void
  onPlatform: (p: Platform) => void
}

// Les classes générées par Wails ont des types plus larges que nos interfaces.
const asReport = (p: Promise<unknown>) => p as Promise<Report>
const asPlatform = (p: Promise<unknown>) => p as Promise<Platform>

const ShutdownWarning = () => {
  const {t} = useI18n()
  return <p className="warn-box">{t.wslWillStop}</p>
}

export function DistroView({report, platform, busy, run, ask, onReport, onPlatform}: Props) {
  const {t} = useI18n()
  const d = report.distro
  const snaps = report.snapshots ?? []
  const [method, setMethod] = useState<CompactMethod>(platform.nativeCompact ? 'wsl' : 'diskpart')
  const [backupDir, setBackupDir] = useState('')
  const [backupFree, setBackupFree] = useState<number | null>(null)
  const [lastBackup, setLastBackup] = useState('')
  const [lastExport, setLastExport] = useState('')

  useEffect(() => {
    setLastBackup('')
    setLastExport('')
  }, [d.name])

  useEffect(() => {
    if (!backupDir) return setBackupFree(null)
    api.FreeSpace(backupDir).then(setBackupFree, () => setBackupFree(null))
  }, [backupDir])

  if (d.version !== 2) {
    return (
      <div className="view">
        <div className="card notice">
          <p>{t.wsl1}</p>
        </div>
      </div>
    )
  }

  const baseline = snaps[0]?.sizeOnDisk ?? d.sizeOnDisk
  const gained = Math.max(0, baseline - d.sizeOnDisk)
  const used = report.usage?.used ?? null
  const trimIdx = lastIndex(snaps, 'trim')
  const trimHost = trimIdx > 0 ? snaps[trimIdx].sizeOnDisk - snaps[trimIdx - 1].sizeOnDisk : null
  const compactIdx = lastIndex(snaps, 'compact')
  const compactGain = compactIdx > 0 ? snaps[compactIdx - 1].sizeOnDisk - snaps[compactIdx].sizeOnDisk : null
  const needBackup = d.fileSize
  const lowSpace = backupFree !== null && backupFree < needBackup

  const analyze = () => run(() => asReport(api.Analyze(d.name))).then((r) => r && onReport(r))
  const trim = () => run(() => asReport(api.Trim(d.name))).then((r) => r && onReport(r))

  const compact = async () => {
    const ok = await ask({
      title: t.compactTitle(d.name),
      confirm: t.compact,
      body: <>
        <ShutdownWarning/>
        {d.managed && <p>{t.quitDocker}</p>}
        {method !== 'wsl' && <p className="muted">{t.adminPrompt}</p>}
      </>,
    })
    if (!ok) return
    const r = await run(() => asReport(api.Compact(d.name, method)))
    if (r) onReport(r)
  }

  const pickBackupDir = async () => {
    const dir = await run(() => api.PickFolder(t.backupFolder))
    if (dir) setBackupDir(dir)
  }

  const backup = async () => {
    const ok = await ask({
      title: t.backupTitle(bytes(needBackup)),
      confirm: t.backup,
      body: <>
        <ShutdownWarning/>
        <p><code>{backupDir}</code></p>
      </>,
    })
    if (!ok) return
    const dest = await run(() => api.Backup(d.name, backupDir))
    if (dest) setLastBackup(dest)
    api.Report(d.name).then((r) => onReport(r as unknown as Report))
  }

  const toggleSparse = async () => {
    const on = !d.sparse
    const ok = await ask({
      title: on ? t.sparseOnTitle : t.sparseOffTitle,
      confirm: on ? t.enable : t.disable,
      danger: on,
      body: <>
        {on && <p className="warn-box">{t.sparseWarn}</p>}
        <p className="muted">{t.willStop(d.name)}</p>
      </>,
    })
    if (!ok) return
    const r = await run(() => asReport(api.SetSparse(d.name, on)))
    if (r) onReport(r)
  }

  const toggleWslConfig = async () => {
    const p = await run(() => asPlatform(api.SetWslConfigSparse(!platform.wslConfigSparse)))
    if (p) onPlatform(p)
  }

  const exportDistro = async () => {
    const file = await run(() => api.PickExportFile(d.name))
    if (!file) return
    const ok = await run(() => api.Export(d.name, file).then(() => true))
    if (ok) setLastExport(file)
  }

  const reimport = async () => {
    const tar = lastExport || await run(() => api.PickTarFile())
    if (!tar) return
    const ok = await ask({
      title: t.reimportTitle(d.name),
      confirm: t.reimport,
      danger: true,
      typeToConfirm: d.name,
      body: <>
        <p className="warn-box">{t.reimportWarn}</p>
        <p><code>{tar}</code></p>
      </>,
    })
    if (!ok) return
    const r = await run(() => asReport(api.Reimport(d.name, tar)))
    if (r) onReport(r)
  }

  const methods: {id: CompactMethod; label: string; ok: boolean; tip: string}[] = [
    {id: 'wsl', label: 'WSL', ok: platform.nativeCompact,
      tip: platform.nativeCompact ? 'wsl --manage --compact' : t.needsWslUpdate},
    {id: 'diskpart', label: 'DiskPart', ok: true, tip: `compact vdisk (${t.asAdmin})`},
    {id: 'optimize', label: 'Optimize-VHD', ok: platform.optimizeVhd,
      tip: platform.optimizeVhd ? `Optimize-VHD -Mode Full (${t.asAdmin})` : t.needsHyperV},
  ]

  const vhd = `"${d.vhdPath}"`
  const compactCmds: Record<CompactMethod, string[]> = {
    wsl: ['wsl --shutdown', `wsl --manage ${d.name} --compact`],
    diskpart: ['wsl --shutdown',
      `diskpart : select vdisk file=${vhd} ; attach vdisk readonly ; compact vdisk ; detach vdisk`],
    optimize: ['wsl --shutdown', `Mount-VHD -Path ${vhd} -ReadOnly ; Optimize-VHD -Path ${vhd} -Mode Full ; Dismount-VHD -Path ${vhd}`],
  }

  return (
    <div className="view">
      <Gauge report={report} baseline={baseline} gained={gained}/>

      <ol className="steps">
        <Step n={1} title={t.analyze} tip={t.analyzeTip}
              done={used !== null}
              result={used !== null && <><strong className="reclaim-text">{bytes(report.reclaimable)}</strong> {t.reclaimableResult}</>}>
          <button className={used === null ? 'btn primary' : 'btn'} disabled={busy} onClick={analyze}>{t.analyze}</button>
        </Step>

        <Step n={2} title={t.trim} tip={t.trimTip}
              cmds={[`wsl -d ${d.name} -u root -- fstrim -v /`]}
              done={report.trimmed !== null}
              result={report.trimmed !== null && <>
                <strong>{bytes(report.trimmed)}</strong> {t.reported}
                {trimHost !== null && trimHost < 0 && <span className="gain-text"> · {delta(trimHost)} {t.onWindowsLower}</span>}
              </>}>
          <button className="btn" disabled={busy} onClick={trim}>{t.trim}</button>
        </Step>

        <Step n={3} title={t.backup} optional done={!!lastBackup}
              tip={t.backupTip(bytes(needBackup))}
              result={lastBackup
                ? <button className="link" onClick={() => api.Reveal(lastBackup)}>{lastBackup}</button>
                : backupDir && <span className={lowSpace ? 'bad' : ''}>
                  <code className="path-chip" title={backupDir}>{backupDir}</code>
                  {backupFree !== null && <> {lowSpace ? t.freeNeeded(bytes(backupFree), bytes(needBackup)) : t.free(bytes(backupFree))}</>}
                </span>}>
          <button className="btn ghost" disabled={busy} onClick={pickBackupDir}>{t.folder}</button>
          <button className="btn" disabled={busy || !backupDir || lowSpace} onClick={backup}>{t.backup}</button>
        </Step>

        <Step n={4} title={t.compact} done={compactIdx >= 0} tip={t.compactTip}
              cmds={compactCmds[method]}
              result={compactGain !== null && <strong className={compactGain > 0 ? 'gain-text' : ''}>{delta(-compactGain)}</strong>}>
          <div className="segctl" role="radiogroup" aria-label={t.method}>
            {methods.map((m) => (
              <button key={m.id} role="radio" aria-checked={method === m.id} className={method === m.id ? 'on' : ''}
                      disabled={!m.ok || busy} title={m.tip} onClick={() => setMethod(m.id)}>{m.label}</button>
            ))}
          </div>
          <button className="btn primary" disabled={busy} onClick={compact}>{t.compact}</button>
        </Step>
      </ol>

      <Summary report={report}/>

      <details className="card advanced">
        <summary>{t.advanced}</summary>

        <div className="adv-item">
          <span className="adv-title" title={`wsl --manage ${d.name} --set-sparse`}>{t.sparseMode}</span>
          <span className={d.sparse ? 'pill on' : 'pill'}>{d.sparse ? t.on : t.off}</span>
          <button className="btn small" disabled={busy || !platform.nativeSparse} onClick={toggleSparse}>
            {d.sparse ? t.disable : t.enable}
          </button>
        </div>

        <div className="adv-item">
          <span className="adv-title" title={t.sparseDefaultTip(platform.wslConfigPath)}>{t.sparseDefault}</span>
          <span className={platform.wslConfigSparse ? 'pill on' : 'pill'}>{platform.wslConfigSparse ? t.on : t.off}</span>
          <button className="btn small" disabled={busy} onClick={toggleWslConfig}>
            {platform.wslConfigSparse ? t.disable : t.enable}
          </button>
        </div>

        <div className="adv-item">
          <span className="adv-title" title={t.exportReimportTip}>{t.exportReimport}</span>
          {lastExport
            ? <button className="link" onClick={() => api.Reveal(lastExport)}>{lastExport}</button>
            : <span/>}
          <span className="adv-actions">
            <button className="btn small" disabled={busy || d.managed} onClick={exportDistro}>{t.exportBtn}</button>
            <button className="btn small danger-outline" disabled={busy || d.managed} onClick={reimport}>{t.reimportBtn}</button>
          </span>
        </div>
      </details>
    </div>
  )
}

function lastIndex(snaps: Report['snapshots'], step: string) {
  for (let i = snaps.length - 1; i >= 0; i--) if (snaps[i].step === step) return i
  return -1
}

export type Tab = 'space' | 'explorer'

export function Head({report, tab, onTab}: {report: Report; tab: Tab; onTab: (t: Tab) => void}) {
  const {t} = useI18n()
  const d = report.distro
  return (
    <>
    <header className="view-head">
      <div>
        <h1>{d.name}</h1>
        <div className="badges">
          <span className={d.version === 2 ? 'pill accent' : 'pill'}
                title={d.version === 2 ? t.badgeTips.wsl2 : t.badgeTips.wsl1}>WSL {d.version}</span>
          <span className={isRunning(d.state) ? 'pill on' : 'pill'}
                title={isRunning(d.state) ? t.badgeTips.running : t.badgeTips.stopped}>
            {isRunning(d.state) ? t.running : t.stopped}
          </span>
          {d.default && <span className="pill" title={t.badgeTips.isDefault}>{t.isDefault}</span>}
          {d.version === 2 && (
            <span className={d.sparse ? 'pill on' : 'pill'} title={d.sparse ? t.badgeTips.sparse : t.badgeTips.notSparse}>
              {d.sparse ? t.sparse : t.notSparse}
            </span>
          )}
          {d.managed && <span className="pill" title={t.badgeTips.managed}>Docker Desktop</span>}
        </div>
      </div>
      {d.vhdPath && (
        <button className="path" title={t.revealVhd} onClick={() => api.Reveal(d.vhdPath)}>
          {d.vhdPath}
        </button>
      )}
    </header>
    <div className="tabs" role="tablist">
      <button role="tab" aria-selected={tab === 'space'} className={tab === 'space' ? 'on' : ''}
              onClick={() => onTab('space')}>{t.tabSpace}</button>
      <button role="tab" aria-selected={tab === 'explorer'} className={tab === 'explorer' ? 'on' : ''}
              disabled={d.managed} onClick={() => onTab('explorer')}>{t.tabExplorer}</button>
    </div>
    </>
  )
}

function Gauge({report, baseline, gained}: {report: Report; baseline: number; gained: number}) {
  const {t} = useI18n()
  const d = report.distro
  const used = report.usage?.used ?? null
  const scale = Math.max(baseline, d.sizeOnDisk, 1)
  const pct = (n: number) => `${(Math.max(0, n) / scale) * 100}%`
  const dataPart = used === null ? d.sizeOnDisk : Math.min(used, d.sizeOnDisk)
  return (
    <section className="card gauge-card" aria-label={t.gaugeLabel}>
      <div className="figures">
        <Figure label={t.onWindows} value={bytes(d.sizeOnDisk)}/>
        <Figure label={t.inLinux} value={used === null ? '—' : bytes(used)} tone="data"/>
        <Figure label={t.reclaimable} value={used === null ? '—' : bytes(report.reclaimable)} tone="reclaim"/>
        <Figure label={t.reclaimed} value={gained > 0 ? bytes(gained) : '—'} tone="gain"/>
      </div>
      <div className="gauge" role="img"
           aria-label={`${t.onWindows}: ${bytes(d.sizeOnDisk)}${used === null ? '' : `, ${t.inLinux}: ${bytes(used)}`}`}>
        <div className={used === null ? 'seg unknown' : 'seg data'} style={{width: pct(dataPart)}}/>
        {used !== null && <div className="seg reclaim" style={{width: pct(d.sizeOnDisk - dataPart)}}/>}
        {gained > 0 && <div className="seg gained" style={{width: pct(gained)}}/>}
      </div>
      <div className="legend">
        <span><i className="sw data"/>{t.data}</span>
        <span><i className="sw reclaim"/>{t.reclaimable}</span>
        {gained > 0 && <span><i className="sw gained"/>{t.reclaimed}</span>}
      </div>
    </section>
  )
}

function Figure({label, value, hint, tone}: {label: string; value: string; hint?: string; tone?: string}) {
  return (
    <div className={`figure ${tone ?? ''}`}>
      <span className="fig-label">{label}</span>
      <span className="fig-value">{value}</span>
      {hint && <span className="fig-hint">{hint}</span>}
    </div>
  )
}

function Step({n, title, tip, cmds, result, done, optional, children}: {
  n: number
  title: string
  tip?: string
  /** Commandes exécutées par l'étape, affichées telles quelles. */
  cmds?: string[]
  result?: ReactNode
  done?: boolean
  optional?: boolean
  children: ReactNode
}) {
  const {t} = useI18n()
  return (
    <li className={done ? 'pstep done' : 'pstep'}>
      <span className="pstep-n" aria-hidden>{done ? '✓' : n}</span>
      <div className="pstep-main">
        <h3 title={tip}>{title}{optional && <span className="optional">{t.optional}</span>}</h3>
        <div className="pstep-result">{result}</div>
        <div className="pstep-actions">{children}</div>
        {cmds && (
          <div className="pstep-cmds">
            {cmds.map((c) => <code key={c}>{c}</code>)}
          </div>
        )}
      </div>
    </li>
  )
}

function Summary({report}: {report: Report}) {
  const {t} = useI18n()
  const snaps = report.snapshots ?? []
  if (snaps.length === 0) return null
  const first = snaps[0].sizeOnDisk
  const last = snaps[snaps.length - 1].sizeOnDisk
  return (
    <section className="card summary">
      <div className="summary-head">
        <h3>{t.summary}</h3>
        <p>
          {first - last > 0
            ? <strong className="gain-text">{delta(last - first)}</strong>
            : null}
        </p>
      </div>
      <table>
        <thead>
          <tr><th>{t.step}</th><th>{t.time}</th><th className="num">{t.onWindows}</th><th className="num">{t.change}</th><th className="num">{t.total}</th></tr>
        </thead>
        <tbody>
          {snaps.map((s, i) => {
            const step = i > 0 ? s.sizeOnDisk - snaps[i - 1].sizeOnDisk : 0
            const total = s.sizeOnDisk - first
            return (
              <tr key={i}>
                <td>{t.snapshots[s.label] ?? s.label}</td>
                <td className="muted">{time(s.at)}</td>
                <td className="num">{bytes(s.sizeOnDisk)}</td>
                <td className={`num ${step < 0 ? 'gain-text' : 'muted'}`}>{i === 0 ? '' : delta(step)}</td>
                <td className={`num ${total < 0 ? 'gain-text' : 'muted'}`}>{i === 0 ? '' : delta(total)}</td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </section>
  )
}
