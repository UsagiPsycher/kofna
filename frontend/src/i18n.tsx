import {createContext, ReactNode, useCallback, useContext, useEffect, useMemo, useState} from 'react'
import * as api from '../wailsjs/go/main/App'
import {setFormatLocale} from './format'

// Textes de l'interface. L'anglais fait référence : les autres langues ont le
// même type, donc une clé oubliée ou une signature différente ne compile pas.

const en = {
  langName: 'English',
  language: 'Language',
  themeToLight: 'Switch to light theme',
  themeToDark: 'Switch to dark theme',

  refresh: 'Refresh',
  shutdownWsl: 'Shut down WSL',
  shutdownTitle: 'Shut down WSL?',
  shutdownWarn: 'Distributions, Docker Desktop and WSL terminals will be closed.',
  admin: 'administrator',
  cancel: 'Cancel',
  close: 'Close',
  distributions: 'Distributions',
  totalVhdx: 'Total VHDX:',

  log: 'Log',
  clear: 'Clear',
  logEmpty: 'Commands and their output will appear here.',
  typeToConfirm: ['Type', 'to confirm'] as [string, string],

  running: 'running',
  stopped: 'stopped',
  isDefault: 'default distro',
  sparse: 'sparse',
  notSparse: 'not sparse',
  badgeTips: {
    wsl2: 'Runs in a lightweight virtual machine; its files live in a virtual disk (ext4.vhdx)',
    wsl1: 'Files are stored directly on the Windows disk, without a virtual disk',
    running: 'The distribution is currently started',
    stopped: 'The distribution is not running',
    isDefault: 'Opened by the plain "wsl" command',
    sparse: 'The virtual disk gives freed space back to Windows on its own',
    notSparse: 'The virtual disk never shrinks on its own: compact it to reclaim space',
    managed: 'Created and managed by Docker Desktop',
  },
  revealVhd: 'Show in File Explorer',
  tabSpace: 'Space',
  tabExplorer: 'Explorer',

  wsl1: 'WSL 1: no virtual disk, space freed in Linux goes straight back to Windows.',
  onWindows: 'On Windows',
  inLinux: 'In Linux',
  reclaimable: 'Reclaimable',
  reclaimed: 'Reclaimed',
  data: 'Data',
  gaugeLabel: 'Virtual disk usage',

  analyze: 'Analyze',
  analyzeTip: 'Compares the space used in Linux with the virtual disk size',
  reclaimableResult: 'reclaimable',
  trim: 'Trim',
  trimTip: 'Tells the virtual disk which space was freed in Linux',
  reported: 'reported',
  onWindowsLower: 'on Windows',
  backup: 'Back up',
  optional: 'optional',
  backupTip: (size: string) => `Copy of ext4.vhdx (${size})`,
  free: (size: string) => `${size} free`,
  freeNeeded: (free: string, need: string) => `${free} free, ${need} required`,
  folder: 'Folder…',
  backupFolder: 'Backup folder',
  backupTitle: (size: string) => `Back up ${size}?`,
  compact: 'Compact',
  compactTip: 'Shuts down WSL, then shrinks the virtual disk',
  compactTitle: (name: string) => `Compact ${name}?`,
  method: 'Method',
  wslWillStop: 'WSL will be shut down: distributions, Docker Desktop and WSL terminals will be closed.',
  quitDocker: 'Quit Docker Desktop first.',
  adminPrompt: 'Windows will ask for administrator permission.',
  needsWslUpdate: 'Requires a recent WSL version (wsl --update)',
  needsHyperV: 'Requires Hyper-V',
  asAdmin: 'administrator',

  summary: 'Summary',
  step: 'Step',
  time: 'Time',
  change: 'Change',
  total: 'Total',
  snapshots: {
    analyse: 'Analysis',
    beforeTrim: 'Before trim',
    afterTrim: 'After trim',
    beforeCompact: 'Before compaction',
    afterCompact: 'After compaction',
    sparseOn: 'Sparse enabled',
    sparseOff: 'Sparse disabled',
    beforeReimport: 'Before reimport',
    afterReimport: 'After reimport',
  } as Record<string, string>,

  advanced: 'Advanced',
  sparseMode: 'Sparse mode',
  on: 'on',
  off: 'off',
  enable: 'Enable',
  disable: 'Disable',
  sparseOnTitle: 'Enable sparse mode?',
  sparseOffTitle: 'Disable sparse mode?',
  sparseWarn: 'Corruption has been reported with this mode. Keep a backup.',
  willStop: (name: string) => `${name} will be stopped.`,
  sparseDefault: 'Sparse by default',
  sparseDefaultTip: (path: string) => `New distributions (${path})`,
  exportReimport: 'Export / reimport',
  exportReimportTip: 'wsl --export, then wsl --import into a new disk',
  exportBtn: 'Export…',
  reimportBtn: 'Reimport…',
  reimport: 'Reimport',
  reimportTitle: (name: string) => `Reimport ${name}?`,
  reimportWarn: 'The current disk will be deleted and replaced with this archive:',

  managedDocker: 'Managed by Docker Desktop.',
  items: (n: number) => `${n.toLocaleString('en-US')} item${n === 1 ? '' : 's'}`,
  scanDistro: (name: string) => `Scan ${name}`,
  installNcdu: 'Install ncdu',
  ncduMissing: (name: string) => `ncdu is not installed in ${name}`,
  path: 'Path',
  parentFolder: 'Parent folder',
  folders: 'Folders',
  bigFiles: 'Large files',
  rescan: 'Rescan',
  rescanTip: (total: string, items: string, at: string) => `${total} in ${items}, scanned at ${at}`,
  empty: 'Empty',
  deleteTitle: (size: string) => `Delete ${size}?`,
  delete: 'Delete',
  andMore: (n: number) => `and ${n} more`,
  inItems: (n: number) => `in ${n} item${n === 1 ? '' : 's'}`,
  deselect: 'Deselect',
  select: 'Select',
  unselect: 'Remove from selection',
  freedInLinux: 'freed in Linux',
  trimAndCompact: 'Trim and compact',
  protected: 'protected',
  smallFiles: (n: number) => `${n.toLocaleString('en-US')} small file${n === 1 ? '' : 's'}`,
  otherItems: (n: number) => `${n.toLocaleString('en-US')} other item${n === 1 ? '' : 's'}`,
  categories: {data: 'Data', cache: 'Cache', deps: 'Dependencies', logs: 'Logs', docker: 'Docker', system: 'System'},
  cleanups: {
    apt: 'apt packages', journal: 'systemd journal', cache: 'Cache', trash: 'Trash', npm: 'npm cache',
    gradle: 'Gradle cache', cargo: 'Cargo registry', gomod: 'Go modules', maven: 'Maven repository',
  } as Record<string, string>,
}

export type Dict = typeof en

const fr: Dict = {
  langName: 'Français',
  language: 'Langue',
  themeToLight: 'Passer au thème clair',
  themeToDark: 'Passer au thème sombre',

  refresh: 'Rafraîchir',
  shutdownWsl: 'Arrêter WSL',
  shutdownTitle: 'Arrêter WSL ?',
  shutdownWarn: 'Les distributions, Docker Desktop et les terminaux WSL seront fermés.',
  admin: 'administrateur',
  cancel: 'Annuler',
  close: 'Fermer',
  distributions: 'Distributions',
  totalVhdx: 'Total des VHDX :',

  log: 'Journal',
  clear: 'Effacer',
  logEmpty: "Les commandes lancées et leur sortie s'afficheront ici.",
  typeToConfirm: ['Tapez', 'pour confirmer'],

  running: 'en marche',
  stopped: 'arrêtée',
  isDefault: 'distrib. par défaut',
  sparse: 'sparse',
  notSparse: 'non sparse',
  badgeTips: {
    wsl2: 'Tourne dans une machine virtuelle légère ; ses fichiers sont dans un disque virtuel (ext4.vhdx)',
    wsl1: 'Les fichiers sont stockés directement sur le disque Windows, sans disque virtuel',
    running: 'La distribution est démarrée',
    stopped: "La distribution n'est pas démarrée",
    isDefault: 'Ouverte par la simple commande « wsl »',
    sparse: "Le disque virtuel rend de lui-même l'espace libéré à Windows",
    notSparse: "Le disque virtuel ne rétrécit jamais seul : compactez-le pour récupérer l'espace",
    managed: 'Créée et gérée par Docker Desktop',
  },
  revealVhd: "Afficher dans l'Explorateur",
  tabSpace: 'Espace',
  tabExplorer: 'Explorer',

  wsl1: "WSL 1 : pas de disque virtuel, l'espace libéré dans Linux revient directement à Windows.",
  onWindows: 'Sur Windows',
  inLinux: 'Dans Linux',
  reclaimable: 'Récupérable',
  reclaimed: 'Récupéré',
  data: 'Données',
  gaugeLabel: 'Occupation du disque virtuel',

  analyze: 'Analyser',
  analyzeTip: "Compare l'espace utilisé dans Linux à la taille du disque virtuel",
  reclaimableResult: 'récupérables',
  trim: 'Trim',
  trimTip: "Signale au disque virtuel l'espace libéré dans Linux",
  reported: 'signalés',
  onWindowsLower: 'sur Windows',
  backup: 'Sauvegarder',
  optional: 'facultatif',
  backupTip: (size) => `Copie de ext4.vhdx (${size})`,
  free: (size) => `${size} libres`,
  freeNeeded: (free, need) => `${free} libres, ${need} requis`,
  folder: 'Dossier…',
  backupFolder: 'Dossier de sauvegarde',
  backupTitle: (size) => `Sauvegarder ${size} ?`,
  compact: 'Compacter',
  compactTip: 'Arrête WSL puis réduit le disque virtuel',
  compactTitle: (name) => `Compacter ${name} ?`,
  method: 'Méthode',
  wslWillStop: 'WSL va être arrêté : les distributions, Docker Desktop et les terminaux WSL seront fermés.',
  quitDocker: "Quittez d'abord Docker Desktop.",
  adminPrompt: 'Windows va demander une autorisation administrateur.',
  needsWslUpdate: 'Nécessite une version récente de WSL (wsl --update)',
  needsHyperV: 'Nécessite Hyper-V',
  asAdmin: 'administrateur',

  summary: 'Bilan',
  step: 'Étape',
  time: 'Heure',
  change: 'Variation',
  total: 'Total',
  snapshots: {
    analyse: 'Analyse',
    beforeTrim: 'Avant trim',
    afterTrim: 'Après trim',
    beforeCompact: 'Avant compactage',
    afterCompact: 'Après compactage',
    sparseOn: 'Sparse activé',
    sparseOff: 'Sparse désactivé',
    beforeReimport: 'Avant réimport',
    afterReimport: 'Après réimport',
  },

  advanced: 'Avancé',
  sparseMode: 'Mode sparse',
  on: 'activé',
  off: 'désactivé',
  enable: 'Activer',
  disable: 'Désactiver',
  sparseOnTitle: 'Activer le mode sparse ?',
  sparseOffTitle: 'Désactiver le mode sparse ?',
  sparseWarn: 'Des corruptions ont été signalées avec ce mode. Gardez une sauvegarde.',
  willStop: (name) => `${name} sera arrêtée.`,
  sparseDefault: 'Sparse par défaut',
  sparseDefaultTip: (path) => `Nouvelles distributions (${path})`,
  exportReimport: 'Export / réimport',
  exportReimportTip: 'wsl --export puis wsl --import dans un disque neuf',
  exportBtn: 'Exporter…',
  reimportBtn: 'Réimporter…',
  reimport: 'Réimporter',
  reimportTitle: (name) => `Réimporter ${name} ?`,
  reimportWarn: 'Le disque actuel sera supprimé et remplacé par cette archive :',

  managedDocker: 'Distribution gérée par Docker Desktop.',
  items: (n) => `${n.toLocaleString('fr-FR')} élément${n > 1 ? 's' : ''}`,
  scanDistro: (name) => `Scanner ${name}`,
  installNcdu: 'Installer ncdu',
  ncduMissing: (name) => `ncdu n'est pas installé dans ${name}`,
  path: 'Chemin',
  parentFolder: 'Dossier parent',
  folders: 'Dossiers',
  bigFiles: 'Gros fichiers',
  rescan: 'Rescanner',
  rescanTip: (total, items, at) => `${total} en ${items}, scanné à ${at}`,
  empty: 'Vider',
  deleteTitle: (size) => `Supprimer ${size} ?`,
  delete: 'Supprimer',
  andMore: (n) => `et ${n} autres`,
  inItems: (n) => `dans ${n} élément${n > 1 ? 's' : ''}`,
  deselect: 'Désélectionner',
  select: 'Sélectionner',
  unselect: 'Retirer de la sélection',
  freedInLinux: 'libérés dans Linux',
  trimAndCompact: 'Trim et compacter',
  protected: 'protégé',
  smallFiles: (n) => `${n.toLocaleString('fr-FR')} petit${n > 1 ? 's' : ''} fichier${n > 1 ? 's' : ''}`,
  otherItems: (n) => `${n.toLocaleString('fr-FR')} autre${n > 1 ? 's' : ''} élément${n > 1 ? 's' : ''}`,
  categories: {data: 'Données', cache: 'Cache', deps: 'Dépendances', logs: 'Journaux', docker: 'Docker', system: 'Système'},
  cleanups: {
    apt: 'Paquets apt', journal: 'Journal systemd', cache: 'Cache', trash: 'Corbeille', npm: 'Cache npm',
    gradle: 'Cache Gradle', cargo: 'Registre Cargo', gomod: 'Modules Go', maven: 'Dépôt Maven',
  },
}

const es: Dict = {
  langName: 'Español',
  language: 'Idioma',
  themeToLight: 'Cambiar al tema claro',
  themeToDark: 'Cambiar al tema oscuro',

  refresh: 'Actualizar',
  shutdownWsl: 'Apagar WSL',
  shutdownTitle: '¿Apagar WSL?',
  shutdownWarn: 'Se cerrarán las distribuciones, Docker Desktop y las terminales de WSL.',
  admin: 'administrador',
  cancel: 'Cancelar',
  close: 'Cerrar',
  distributions: 'Distribuciones',
  totalVhdx: 'Total de VHDX:',

  log: 'Registro',
  clear: 'Borrar',
  logEmpty: 'Aquí aparecerán los comandos ejecutados y su salida.',
  typeToConfirm: ['Escriba', 'para confirmar'],

  running: 'en ejecución',
  stopped: 'detenida',
  isDefault: 'distro predeterminada',
  sparse: 'sparse',
  notSparse: 'no sparse',
  badgeTips: {
    wsl2: 'Se ejecuta en una máquina virtual ligera; sus archivos están en un disco virtual (ext4.vhdx)',
    wsl1: 'Los archivos se guardan directamente en el disco de Windows, sin disco virtual',
    running: 'La distribución está iniciada',
    stopped: 'La distribución no está en ejecución',
    isDefault: 'Se abre con el comando "wsl" sin argumentos',
    sparse: 'El disco virtual devuelve por sí solo a Windows el espacio liberado',
    notSparse: 'El disco virtual nunca se reduce solo: compáctelo para recuperar espacio',
    managed: 'Creada y gestionada por Docker Desktop',
  },
  revealVhd: 'Mostrar en el Explorador de archivos',
  tabSpace: 'Espacio',
  tabExplorer: 'Explorador',

  wsl1: 'WSL 1: sin disco virtual, el espacio liberado en Linux vuelve directamente a Windows.',
  onWindows: 'En Windows',
  inLinux: 'En Linux',
  reclaimable: 'Recuperable',
  reclaimed: 'Recuperado',
  data: 'Datos',
  gaugeLabel: 'Ocupación del disco virtual',

  analyze: 'Analizar',
  analyzeTip: 'Compara el espacio usado en Linux con el tamaño del disco virtual',
  reclaimableResult: 'recuperables',
  trim: 'Trim',
  trimTip: 'Indica al disco virtual el espacio liberado en Linux',
  reported: 'informados',
  onWindowsLower: 'en Windows',
  backup: 'Copia de seguridad',
  optional: 'opcional',
  backupTip: (size) => `Copia de ext4.vhdx (${size})`,
  free: (size) => `${size} libres`,
  freeNeeded: (free, need) => `${free} libres, se necesitan ${need}`,
  folder: 'Carpeta…',
  backupFolder: 'Carpeta de copia de seguridad',
  backupTitle: (size) => `¿Copiar ${size}?`,
  compact: 'Compactar',
  compactTip: 'Apaga WSL y reduce el disco virtual',
  compactTitle: (name) => `¿Compactar ${name}?`,
  method: 'Método',
  wslWillStop: 'WSL se apagará: se cerrarán las distribuciones, Docker Desktop y las terminales de WSL.',
  quitDocker: 'Cierre primero Docker Desktop.',
  adminPrompt: 'Windows pedirá permiso de administrador.',
  needsWslUpdate: 'Requiere una versión reciente de WSL (wsl --update)',
  needsHyperV: 'Requiere Hyper-V',
  asAdmin: 'administrador',

  summary: 'Resumen',
  step: 'Paso',
  time: 'Hora',
  change: 'Variación',
  total: 'Total',
  snapshots: {
    analyse: 'Análisis',
    beforeTrim: 'Antes del trim',
    afterTrim: 'Después del trim',
    beforeCompact: 'Antes de compactar',
    afterCompact: 'Después de compactar',
    sparseOn: 'Sparse activado',
    sparseOff: 'Sparse desactivado',
    beforeReimport: 'Antes de reimportar',
    afterReimport: 'Después de reimportar',
  },

  advanced: 'Avanzado',
  sparseMode: 'Modo sparse',
  on: 'activado',
  off: 'desactivado',
  enable: 'Activar',
  disable: 'Desactivar',
  sparseOnTitle: '¿Activar el modo sparse?',
  sparseOffTitle: '¿Desactivar el modo sparse?',
  sparseWarn: 'Se han notificado corrupciones con este modo. Conserve una copia de seguridad.',
  willStop: (name) => `${name} se detendrá.`,
  sparseDefault: 'Sparse por defecto',
  sparseDefaultTip: (path) => `Distribuciones nuevas (${path})`,
  exportReimport: 'Exportar / reimportar',
  exportReimportTip: 'wsl --export y luego wsl --import en un disco nuevo',
  exportBtn: 'Exportar…',
  reimportBtn: 'Reimportar…',
  reimport: 'Reimportar',
  reimportTitle: (name) => `¿Reimportar ${name}?`,
  reimportWarn: 'El disco actual se eliminará y se reemplazará por este archivo:',

  managedDocker: 'Distribución gestionada por Docker Desktop.',
  items: (n) => `${n.toLocaleString('es-ES')} elemento${n === 1 ? '' : 's'}`,
  scanDistro: (name) => `Escanear ${name}`,
  installNcdu: 'Instalar ncdu',
  ncduMissing: (name) => `ncdu no está instalado en ${name}`,
  path: 'Ruta',
  parentFolder: 'Carpeta superior',
  folders: 'Carpetas',
  bigFiles: 'Archivos grandes',
  rescan: 'Volver a escanear',
  rescanTip: (total, items, at) => `${total} en ${items}, escaneado a las ${at}`,
  empty: 'Vaciar',
  deleteTitle: (size) => `¿Eliminar ${size}?`,
  delete: 'Eliminar',
  andMore: (n) => `y ${n} más`,
  inItems: (n) => `en ${n} elemento${n === 1 ? '' : 's'}`,
  deselect: 'Deseleccionar',
  select: 'Seleccionar',
  unselect: 'Quitar de la selección',
  freedInLinux: 'liberados en Linux',
  trimAndCompact: 'Trim y compactar',
  protected: 'protegido',
  smallFiles: (n) => `${n.toLocaleString('es-ES')} archivo${n === 1 ? '' : 's'} pequeño${n === 1 ? '' : 's'}`,
  otherItems: (n) => `${n.toLocaleString('es-ES')} elemento${n === 1 ? '' : 's'} más`,
  categories: {data: 'Datos', cache: 'Caché', deps: 'Dependencias', logs: 'Registros', docker: 'Docker', system: 'Sistema'},
  cleanups: {
    apt: 'Paquetes apt', journal: 'Diario de systemd', cache: 'Caché', trash: 'Papelera', npm: 'Caché de npm',
    gradle: 'Caché de Gradle', cargo: 'Registro de Cargo', gomod: 'Módulos de Go', maven: 'Repositorio de Maven',
  },
}

export const dictionaries = {en, fr, es}
export type Lang = keyof typeof dictionaries
export const languages = Object.keys(dictionaries) as Lang[]

const LANG_KEY = 'kofna.lang'

function storedLang(): Lang {
  try {
    const v = localStorage.getItem(LANG_KEY)
    if (v && v in dictionaries) return v as Lang
  } catch {
    // Stockage indisponible : on garde la langue par défaut.
  }
  return 'en'
}

interface I18n {
  t: Dict
  lang: Lang
  setLang: (l: Lang) => void
}

const Ctx = createContext<I18n>({t: en, lang: 'en', setLang: () => {}})

export function I18nProvider({children}: {children: ReactNode}) {
  const [lang, setLangState] = useState<Lang>(storedLang)

  // Format des nombres, langue des messages Go et attribut lang du document.
  setFormatLocale(lang)
  useEffect(() => {
    document.documentElement.lang = lang
    api.SetLanguage(lang)
  }, [lang])

  const setLang = useCallback((l: Lang) => {
    setLangState(l)
    try {
      localStorage.setItem(LANG_KEY, l)
    } catch {
      // Préférence non mémorisée, sans conséquence.
    }
  }, [])

  const value = useMemo(() => ({t: dictionaries[lang], lang, setLang}), [lang, setLang])
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

export const useI18n = () => useContext(Ctx)
