// Miroir des structures Go (internal/wsl et app.go). On garde nos propres
// interfaces plutôt que les classes générées par Wails pour rester lisibles.

export interface Distro {
  id: string
  name: string
  version: number
  state: string
  default: boolean
  basePath: string
  vhdPath: string
  fileSize: number
  sizeOnDisk: number
  sparse: boolean
  defaultUid: number
  managed: boolean
}

export interface Usage {
  total: number
  used: number
  avail: number
}

export interface Snapshot {
  step: 'analyse' | 'trim' | 'compact' | 'sparse' | 'reimport'
  label: string
  at: string
  fileSize: number
  sizeOnDisk: number
  used: number
}

export interface Report {
  distro: Distro
  usage: Usage | null
  reclaimable: number
  trimmed: number | null
  snapshots: Snapshot[]
}

export interface Platform {
  wslVersion: string
  nativeCompact: boolean
  nativeSparse: boolean
  optimizeVhd: boolean
  isAdmin: boolean
  wslConfigPath: string
  wslConfigSparse: boolean
}

export interface LogLine {
  at: string
  level: 'cmd' | 'out' | 'info' | 'warn' | 'error'
  text: string
}

export interface ProgressEvent {
  task: string
  done: number
  total: number
}

export type CompactMethod = 'wsl' | 'diskpart' | 'optimize'

export type Category = 'data' | 'cache' | 'deps' | 'logs' | 'docker' | 'system'

export interface FsNode {
  name: string
  path: string
  size: number
  items: number
  kind: 'dir' | 'file' | 'small' | 'rest'
  /** Nombre d'éléments regroupés (small, rest). */
  count?: number
  category: Category
  protected: boolean
  readError: boolean
  children?: FsNode[]
}

export interface FsView {
  node: FsNode
  categories: Partial<Record<Category, number>>
}

export interface Cleanup {
  id: string
  key: string
  path: string
  size: number
  category: Category
  command: string
}

export interface ExplorerStatus {
  ncdu: string
  scanned: boolean
  scannedAt: string
  seconds: number
  total: number
  items: number
  freed: number
}

export interface DeleteResult {
  deleted: string[]
  failed: string[] | null
  freed: number
}

export interface ScanEvent {
  items: number
  bytes: number
  total: number
}
