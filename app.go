package main

import (
	"context"
	"errors"
	"fmt"
	"kofna/internal/i18n"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"kofna/internal/wsl"
)

// App est l'objet exposé au front-end : chaque méthode publique devient une
// fonction TypeScript dans frontend/wailsjs/go/main/App.
type App struct {
	ctx context.Context
	m   *wsl.Manager

	busy   sync.Mutex // une seule opération longue à la fois
	mu     sync.Mutex // protège states et cancel
	cancel context.CancelFunc
	states map[string]*distroState
	trees  map[string]*wsl.Tree // scans de l'explorateur
	freed  map[string]int64
}

// distroState garde, pour la session, ce qu'on a mesuré sur une distribution.
type distroState struct {
	usage     *wsl.Usage
	trimmed   *int64
	snapshots []wsl.Snapshot
}

// Report est ce que le front affiche pour une distribution.
type Report struct {
	Distro      wsl.Distro     `json:"distro"`
	Usage       *wsl.Usage     `json:"usage"`
	Reclaimable int64          `json:"reclaimable"` // estimation : taille sur disque − espace utilisé dans Linux
	Trimmed     *int64         `json:"trimmed"`     // octets signalés par le dernier fstrim
	Snapshots   []wsl.Snapshot `json:"snapshots"`
}

type logLine struct {
	At    string `json:"at"`
	Level string `json:"level"`
	Text  string `json:"text"`
}

type progressEvent struct {
	Task  string `json:"task"`
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
}

func NewApp() *App {
	return &App{states: map[string]*distroState{}, trees: map[string]*wsl.Tree{}, freed: map[string]int64{}}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.m = wsl.New(func(level, text string) {
		runtime.EventsEmit(a.ctx, "log", logLine{At: time.Now().Format("15:04:05"), Level: level, Text: text})
	})
}

func (a *App) info(text string) {
	runtime.EventsEmit(a.ctx, "log", logLine{At: time.Now().Format("15:04:05"), Level: "info", Text: text})
}

// begin réserve l'application pour une opération longue et annulable.
func (a *App) begin(task string) (context.Context, func(), error) {
	if !a.busy.TryLock() {
		return nil, nil, errors.New(i18n.T("busy"))
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.mu.Lock()
	a.cancel = cancel
	a.mu.Unlock()
	runtime.EventsEmit(a.ctx, "busy", task)
	return ctx, func() {
		a.mu.Lock()
		a.cancel = nil
		a.mu.Unlock()
		cancel()
		runtime.EventsEmit(a.ctx, "busy", "")
		a.busy.Unlock()
	}, nil
}

func (a *App) progress(task string) wsl.Progress {
	return func(done, total int64) {
		runtime.EventsEmit(a.ctx, "progress", progressEvent{Task: task, Done: done, Total: total})
	}
}

func (a *App) state(name string) *distroState {
	key := strings.ToLower(name)
	a.mu.Lock()
	defer a.mu.Unlock()
	s, ok := a.states[key]
	if !ok {
		s = &distroState{}
		a.states[key] = s
	}
	return s
}

func (a *App) setUsage(name string, u *wsl.Usage) {
	s := a.state(name)
	a.mu.Lock()
	s.usage = u
	a.mu.Unlock()
}

func (a *App) snapshot(d wsl.Distro, step, label string) {
	s := a.state(d.Name)
	snap := wsl.Snapshot{Step: step, Label: label, At: time.Now(), FileSize: d.FileSize, SizeOnDisk: d.SizeOnDisk}
	a.mu.Lock()
	if s.usage != nil {
		snap.Used = s.usage.Used
	}
	s.snapshots = append(s.snapshots, snap)
	a.mu.Unlock()
}

// snapshotIfChanged évite d'empiler deux étapes identiques dans le bilan.
func (a *App) snapshotIfChanged(d wsl.Distro, step, label string) {
	s := a.state(d.Name)
	a.mu.Lock()
	n := len(s.snapshots)
	same := n > 0 && s.snapshots[n-1].SizeOnDisk == d.SizeOnDisk
	a.mu.Unlock()
	if !same {
		a.snapshot(d, step, label)
	}
}

func (a *App) report(d wsl.Distro) Report {
	s := a.state(d.Name)
	a.mu.Lock()
	defer a.mu.Unlock()
	r := Report{Distro: d, Usage: s.usage, Trimmed: s.trimmed, Snapshots: append([]wsl.Snapshot{}, s.snapshots...)}
	if s.usage != nil && d.SizeOnDisk > s.usage.Used {
		r.Reclaimable = d.SizeOnDisk - s.usage.Used
	}
	return r
}

// --- Méthodes exposées ------------------------------------------------------

// SetLanguage choisit la langue des messages produits côté Go (en, fr, es).
func (a *App) SetLanguage(lang string) { i18n.Set(lang) }

// Platform décrit la machine (version de WSL, méthodes de compactage disponibles…).
func (a *App) Platform() wsl.Platform {
	return a.m.Platform(a.ctx)
}

// Distros liste les distributions installées.
func (a *App) Distros() ([]wsl.Distro, error) {
	return a.m.Distros(a.ctx)
}

// Report renvoie l'état connu d'une distribution, tailles rafraîchies.
func (a *App) Report(name string) (Report, error) {
	d, err := a.m.Distro(a.ctx, name)
	if err != nil {
		return Report{}, err
	}
	return a.report(d), nil
}

// Analyze mesure l'espace utilisé dans Linux et estime ce qu'on peut récupérer.
func (a *App) Analyze(name string) (Report, error) {
	ctx, done, err := a.begin(i18n.T("task.analyze", name))
	if err != nil {
		return Report{}, err
	}
	defer done()
	u, err := a.m.Usage(ctx, name)
	if err != nil {
		return Report{}, err
	}
	a.setUsage(name, &u)
	d, err := a.m.Distro(ctx, name)
	if err != nil {
		return Report{}, err
	}
	a.snapshot(d, "analyse", "analyse")
	r := a.report(d)
	a.info(i18n.T("log.analyzed",
		name, wsl.HumanBytes(d.SizeOnDisk), wsl.HumanBytes(u.Used), wsl.HumanBytes(r.Reclaimable)))
	return r, nil
}

// Trim lance fstrim pour signaler les blocs libres au disque virtuel.
func (a *App) Trim(name string) (Report, error) {
	ctx, done, err := a.begin(i18n.T("task.trim", name))
	if err != nil {
		return Report{}, err
	}
	defer done()
	before, err := a.m.Distro(ctx, name)
	if err != nil {
		return Report{}, err
	}
	n, err := a.m.Trim(ctx, name)
	if err != nil {
		return Report{}, err
	}
	s := a.state(name)
	a.mu.Lock()
	s.trimmed = &n
	a.mu.Unlock()
	if u, err := a.m.Usage(ctx, name); err == nil {
		a.setUsage(name, &u)
	}
	// Un VHDX sparse rend l'espace en quelques secondes ; on laisse à Windows le temps de le faire.
	if before.Sparse {
		time.Sleep(3 * time.Second)
	}
	d, err := a.m.Distro(ctx, name)
	if err != nil {
		return Report{}, err
	}
	a.mu.Lock()
	first := len(s.snapshots) == 0
	a.mu.Unlock()
	if first {
		a.snapshot(before, "analyse", "beforeTrim")
	}
	a.snapshot(d, "trim", "afterTrim")
	a.info(i18n.T("log.trimmed",
		wsl.HumanBytes(n), wsl.HumanBytes(before.SizeOnDisk), wsl.HumanBytes(d.SizeOnDisk)))
	return a.report(d), nil
}

// Shutdown exécute `wsl --shutdown`.
func (a *App) Shutdown() error {
	ctx, done, err := a.begin(i18n.T("task.shutdown"))
	if err != nil {
		return err
	}
	defer done()
	return a.m.Shutdown(ctx, "")
}

// Compact arrête WSL puis compacte le VHDX de la distribution.
func (a *App) Compact(name, method string) (Report, error) {
	ctx, done, err := a.begin(i18n.T("task.compact", name))
	if err != nil {
		return Report{}, err
	}
	defer done()
	d, err := a.m.Distro(ctx, name)
	if err != nil {
		return Report{}, err
	}
	if d.VhdPath == "" {
		return Report{}, errors.New(i18n.T("err.wsl1"))
	}
	a.info(i18n.T("log.shutdownBeforeCompact"))
	if err := a.m.Shutdown(ctx, d.VhdPath); err != nil {
		return Report{}, err
	}
	a.snapshotIfChanged(d, "analyse", "beforeCompact")
	started := time.Now()
	if err := a.m.Compact(ctx, d, wsl.CompactMethod(method)); err != nil {
		return Report{}, err
	}
	after, err := a.m.Distro(ctx, name)
	if err != nil {
		return Report{}, err
	}
	a.snapshot(after, "compact", "afterCompact")
	a.info(i18n.T("log.compacted",
		time.Since(started).Round(time.Second), wsl.HumanBytes(d.SizeOnDisk),
		wsl.HumanBytes(after.SizeOnDisk), wsl.HumanBytes(d.SizeOnDisk-after.SizeOnDisk)))
	return a.report(after), nil
}

// Backup arrête WSL et copie le VHDX dans un dossier.
func (a *App) Backup(name, dir string) (string, error) {
	ctx, done, err := a.begin(i18n.T("task.backup", name))
	if err != nil {
		return "", err
	}
	defer done()
	d, err := a.m.Distro(ctx, name)
	if err != nil {
		return "", err
	}
	if err := a.m.Shutdown(ctx, d.VhdPath); err != nil {
		return "", err
	}
	dest, err := a.m.Backup(ctx, d, dir, a.progress("backup"))
	if err != nil {
		return "", err
	}
	a.info(i18n.T("log.backupDone", dest))
	return dest, nil
}

// SetSparse active ou désactive le mode sparse de la distribution.
func (a *App) SetSparse(name string, on bool) (Report, error) {
	ctx, done, err := a.begin(i18n.T("task.sparse", name))
	if err != nil {
		return Report{}, err
	}
	defer done()
	d, err := a.m.Distro(ctx, name)
	if err != nil {
		return Report{}, err
	}
	if err := a.m.SetSparse(ctx, d, on); err != nil {
		return Report{}, err
	}
	after, err := a.m.Distro(ctx, name)
	if err != nil {
		return Report{}, err
	}
	a.snapshotIfChanged(after, "sparse", map[bool]string{true: "sparseOn", false: "sparseOff"}[on])
	return a.report(after), nil
}

// SetWslConfigSparse règle sparseVhd dans .wslconfig (nouvelles distributions).
func (a *App) SetWslConfigSparse(on bool) (wsl.Platform, error) {
	if err := a.m.WriteWslConfigSparse(on); err != nil {
		return wsl.Platform{}, err
	}
	return a.m.Platform(a.ctx), nil
}

// Export écrit la distribution dans une archive tar (wsl --export).
func (a *App) Export(name, file string) error {
	ctx, done, err := a.begin(i18n.T("task.export", name))
	if err != nil {
		return err
	}
	defer done()
	var expected int64
	s := a.state(name)
	a.mu.Lock()
	if s.usage != nil {
		expected = s.usage.Used
	}
	a.mu.Unlock()
	if err := a.m.Export(ctx, name, file, expected, a.progress("export")); err != nil {
		return err
	}
	a.info(i18n.T("log.exportDone", file))
	return nil
}

// Reimport remplace la distribution par une archive exportée.
func (a *App) Reimport(name, tarPath string) (Report, error) {
	ctx, done, err := a.begin(i18n.T("task.reimport", name))
	if err != nil {
		return Report{}, err
	}
	defer done()
	d, err := a.m.Distro(ctx, name)
	if err != nil {
		return Report{}, err
	}
	a.snapshotIfChanged(d, "analyse", "beforeReimport")
	if err := a.m.Reimport(ctx, d, tarPath); err != nil {
		return Report{}, err
	}
	after, err := a.m.Distro(a.ctx, name)
	if err != nil {
		return Report{}, err
	}
	a.snapshot(after, "reimport", "afterReimport")
	a.info(i18n.T("log.reimportDone", wsl.HumanBytes(d.SizeOnDisk), wsl.HumanBytes(after.SizeOnDisk)))
	return a.report(after), nil
}

// Cancel interrompt l'opération en cours quand c'est possible (copie, export).
func (a *App) Cancel() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		a.info(i18n.T("log.cancel"))
		a.cancel()
	}
}

// Reveal ouvre l'explorateur sur un fichier.
func (a *App) Reveal(path string) error { return a.m.Reveal(path) }

// FreeSpace renvoie l'espace libre du volume d'un dossier.
func (a *App) FreeSpace(dir string) (int64, error) { return wsl.FreeSpace(dir) }

// PickFolder ouvre un sélecteur de dossier.
func (a *App) PickFolder(title string) (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: title})
}

// PickExportFile demande où écrire l'archive d'export.
func (a *App) PickExportFile(name string) (string, error) {
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           i18n.T("dlg.export", name),
		DefaultFilename: fmt.Sprintf("%s-%s.tar", name, time.Now().Format("20060102")),
		Filters:         []runtime.FileFilter{{DisplayName: i18n.T("dlg.tarFilter"), Pattern: "*.tar"}},
	})
}

// PickTarFile demande l'archive à réimporter.
func (a *App) PickTarFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   i18n.T("dlg.reimport"),
		Filters: []runtime.FileFilter{{DisplayName: i18n.T("dlg.tarFilter"), Pattern: "*.tar"}},
	})
}
