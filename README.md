# Kofna - WSL Space Liberator

A small Windows desktop app that measures and reclaims the space lost by WSL 2 virtual disks
(`ext4.vhdx`).

With WSL 2, `ext4.vhdx` grows on its own but never shrinks when you delete files in Linux. Kofna shows
the gap, helps you clean up, then gives the space back to Windows.

## Features

| Need | In the app |
| --- | --- |
| Check WSL 1 vs WSL 2 (`wsl -l -v`) | Distribution list with version, state and VHDX location (read from the `HKCU\…\Lxss` registry key) |
| See the lost space | **Analyze**: `df` inside Linux, compared with the space the VHDX *actually* allocates on Windows → estimated reclaimable space |
| `fstrim` | **Trim**: `fstrim -v /` as root, with the reported amount and the VHDX size before/after |
| `wsl --shutdown` | Done automatically before backup and compaction (after confirmation), or via "Shut down WSL" |
| Back up the VHDX | **Back up**: copy with progress, free-space check, cancellable |
| Compact the VHDX | **Compact** with `wsl --manage <distro> --compact` (recent WSL), DiskPart (`select / attach readonly / compact / detach vdisk`) or `Optimize-VHD` (Hyper-V). The last two show a UAC prompt |
| Check the new size | **Summary**: VHDX size at each step, change per step and since the start |
| Sparse mode | Toggle `wsl --manage <distro> --set-sparse`, and `[experimental] sparseVhd` in `.wslconfig` (a `.bak` copy is kept) |
| Export / reimport | Export to `.tar` with progress. Reimport: `--unregister` then `--import` at the same location, restoring the default user, the default distribution and sparse mode. The distribution name must be typed to confirm |

Every command and its output are shown in the **Log** panel. The commands run by the Trim and Compact
steps are also displayed next to them.

The UI is available in English (default), French and Spanish, with a dark or light theme. On first
launch the app follows the Windows theme, then remembers both choices.

### Explorer tab

A WinDirStat-like visual explorer, powered by **ncdu** (one-click install if missing):

- `ncdu -x -0 -o- /` runs as root inside the distribution and its JSON export is streamed. 1.2 million
  files take under a minute and about 30 MB of memory. Files under 1 MB are grouped per folder.
- **Cushion treemap** (van Wijk), like WinDirStat: the whole hierarchy is drawn edge to edge on a canvas,
  and shading separates the folders. Colors show what is safe to delete: caches (amber), re-downloadable
  dependencies (`node_modules`, `.venv`, IDE servers…), logs, Docker, data, system. Clicking opens the
  hovered item.
- **Automatic scan** the first time the tab is opened, then a "Rescan" button.
- **ncdu-style list** next to it: bar, size, name.
- **Large files**: the 200 largest files on the disk.
- **Quick cleanups**: apt cache, systemd journal, `~/.cache`, npm cache, trash, Gradle, Cargo, Go
  modules, Maven. Only offered from 10 MB.
- **Deletion**: `rm -rf --one-file-system` as root, without a shell. System folders and top-level folders
  (`/home`, `/var`…) are protected. After a deletion the tree is updated without rescanning, and a button
  leads to trim and compaction.

## Development

WSL only exists on Windows, so Kofna is Windows-only.

Stack: [Wails v2](https://wails.io) (Go + WebView2), with a Vite + React + TypeScript front end.

Requirements: Windows 10/11 (WebView2 is preinstalled), Go ≥ 1.25, Node ≥ 20.19, and the Wails CLI:

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0
```

```sh
wails build    # → build/bin/kofna.exe
wails dev      # hot reload
wails doctor   # checks the machine's dependencies
```

### Versioning

The version lives in one place: `info.productVersion` in `wails.json` (`X.Y.Z`, as Windows requires).
Bump it before each build. A trailing `.0` is hidden in the UI: `0.67.0` shows as `v0.67`. It is embedded in the binary, shown at the bottom of the sidebar, and written by Wails into the
executable's properties. A CI can override it with `wails build -ldflags "-X main.version=1.2.3"`.

### Tests

```sh
go test ./internal/...                                    # parsers, .wslconfig, formats, ncdu tree, i18n
set KOFNA_LIVE=1 && go test -run Live -v ./internal/wsl   # read-only, against the real WSL
set KOFNA_LIVE_SCAN=/ && go test -run LiveScan -v ./internal/wsl   # real ncdu scan, read-only
```

## Layout

```
main.go, app.go           Wails app: methods exposed to the front end, events (log, busy, progress)
version.go                app version, read from wails.json
app_explorer.go           Explorer tab: scan, views, deletion, quick cleanups
internal/wsl/             WSL logic, testable without the UI
  manager.go              commands: analyze, trim, shutdown, backup, compaction, sparse, export/import
  scan.go                 ncdu export reader, tree, categories, protections, treemap views
  explorer.go             ncdu, deletion, quick cleanups
  parse.go                wsl.exe output decoding (UTF-8/UTF-16), wsl -l -v, df, fstrim parsing
  wslconfig.go            targeted .wslconfig read/write
  platform.go             registry, allocated size (GetCompressedFileSizeW), file locks, elevation
internal/i18n/            Go-side messages (errors, log) in English, French and Spanish
frontend/src/             React UI; texts in i18n.tsx, theme in theme.ts
```

## Good to know

- Compaction only reclaims space that is free *inside* Linux: delete unneeded files first (Docker
  images, caches…).
- Running `fstrim` before compacting often helps DiskPart and `Optimize-VHD` reclaim more.
- `wsl --shutdown` stops everything, including Docker Desktop and open WSL terminals. To compact the
  `docker-desktop` distribution, quit Docker Desktop first, otherwise it restarts it right away.
- Sparse mode was experimental for a long time. Only enable it with a backup.
