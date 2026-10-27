package main

import (
	"errors"
	"kofna/internal/i18n"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"kofna/internal/wsl"
)

// Explorateur : scan ncdu de la distribution, vues pour la treemap, suppression
// et nettoyages rapides. L'arbre reste en mémoire pour la session.

// ExplorerStatus résume l'état de l'explorateur pour une distribution.
type ExplorerStatus struct {
	Ncdu      string    `json:"ncdu"` // version, vide si absent
	Scanned   bool      `json:"scanned"`
	ScannedAt time.Time `json:"scannedAt"`
	Seconds   float64   `json:"seconds"`
	Total     int64     `json:"total"`
	Items     int64     `json:"items"`
	Freed     int64     `json:"freed"` // supprimé depuis l'explorateur pendant la session
}

// DeleteResult est le bilan d'une suppression.
type DeleteResult struct {
	Deleted []string `json:"deleted"`
	Failed  []string `json:"failed"`
	Freed   int64    `json:"freed"`
}

type scanEvent struct {
	Items int64 `json:"items"`
	Bytes int64 `json:"bytes"`
	Total int64 `json:"total"`
}

func (a *App) tree(name string) (*wsl.Tree, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	t := a.trees[strings.ToLower(name)]
	if t == nil {
		return nil, errors.New(i18n.T("err.noScan"))
	}
	return t, nil
}

func (a *App) explorerStatus(name, ncdu string) ExplorerStatus {
	s := ExplorerStatus{Ncdu: ncdu}
	a.mu.Lock()
	t := a.trees[strings.ToLower(name)]
	s.Freed = a.freed[strings.ToLower(name)]
	a.mu.Unlock()
	if t != nil {
		s.Scanned = true
		s.ScannedAt = t.ScannedAt
		s.Seconds = t.Duration.Seconds()
		s.Total, s.Items = t.Stats()
	}
	return s
}

// ExplorerStatus indique si ncdu est présent et si un scan existe.
func (a *App) ExplorerStatus(name string) ExplorerStatus {
	v, _ := a.m.NcduVersion(a.ctx, name)
	return a.explorerStatus(name, v)
}

// InstallNcdu installe ncdu dans la distribution.
func (a *App) InstallNcdu(name string) (ExplorerStatus, error) {
	ctx, done, err := a.begin(i18n.T("task.installNcdu"))
	if err != nil {
		return ExplorerStatus{}, err
	}
	defer done()
	if err := a.m.InstallNcdu(ctx, name); err != nil {
		return ExplorerStatus{}, err
	}
	v, err := a.m.NcduVersion(ctx, name)
	if err != nil {
		return ExplorerStatus{}, err
	}
	return a.explorerStatus(name, v), nil
}

// Scan parcourt toute la distribution avec ncdu.
func (a *App) Scan(name string) (ExplorerStatus, error) {
	ctx, done, err := a.begin(i18n.T("task.scan", name))
	if err != nil {
		return ExplorerStatus{}, err
	}
	defer done()
	v, err := a.m.NcduVersion(ctx, name)
	if err != nil {
		return ExplorerStatus{}, err
	}
	// L'espace utilisé sert de total pour la progression.
	var total int64
	if u, err := a.m.Usage(ctx, name); err == nil {
		total = u.Used
		a.setUsage(name, &u)
	}
	start := time.Now()
	root, err := a.m.ScanPath(ctx, name, "/", "", func(items, bytes int64) {
		runtime.EventsEmit(a.ctx, "scan", scanEvent{Items: items, Bytes: bytes, Total: total})
	})
	if err != nil {
		return ExplorerStatus{}, err
	}
	t := &wsl.Tree{Root: root, ScannedAt: time.Now(), Duration: time.Since(start)}
	a.mu.Lock()
	a.trees[strings.ToLower(name)] = t
	a.mu.Unlock()
	size, items := t.Stats()
	a.info(i18n.T("log.scanDone", wsl.HumanBytes(size), items, t.Duration.Round(time.Second)))
	return a.explorerStatus(name, v), nil
}

// View renvoie un dossier du scan, sur deux niveaux.
func (a *App) View(name, path string) (wsl.FsView, error) {
	t, err := a.tree(name)
	if err != nil {
		return wsl.FsView{}, err
	}
	return t.View(path, 300, 120)
}

// TopFiles renvoie les plus gros fichiers du scan.
func (a *App) TopFiles(name string) ([]wsl.FsNode, error) {
	t, err := a.tree(name)
	if err != nil {
		return nil, err
	}
	return t.TopFiles(200), nil
}

// Cleanups renvoie les nettoyages rapides disponibles.
func (a *App) Cleanups(name string) ([]wsl.Cleanup, error) {
	t, err := a.tree(name)
	if err != nil {
		return nil, err
	}
	return wsl.Cleanups(t), nil
}

// RunCleanup exécute un nettoyage rapide.
func (a *App) RunCleanup(name, id string) (ExplorerStatus, error) {
	t, err := a.tree(name)
	if err != nil {
		return ExplorerStatus{}, err
	}
	ctx, done, err := a.begin(i18n.T("task.cleanup"))
	if err != nil {
		return ExplorerStatus{}, err
	}
	defer done()
	freed, err := a.m.RunCleanup(ctx, name, t, id)
	if err != nil {
		return ExplorerStatus{}, err
	}
	a.afterDelete(name, freed)
	a.info(i18n.T("log.cleanupDone", id, wsl.HumanBytes(freed)))
	v, _ := a.m.NcduVersion(a.ctx, name)
	return a.explorerStatus(name, v), nil
}

// Delete supprime des fichiers ou dossiers de la distribution.
func (a *App) Delete(name string, paths []string) (DeleteResult, error) {
	t, err := a.tree(name)
	if err != nil {
		return DeleteResult{}, err
	}
	keep, _, err := t.CheckDeletable(paths)
	if err != nil {
		return DeleteResult{}, err
	}
	ctx, done, err := a.begin(i18n.T("task.delete", len(keep)))
	if err != nil {
		return DeleteResult{}, err
	}
	defer done()
	remaining, delErr := a.m.Delete(ctx, name, keep)
	left := map[string]bool{}
	for _, p := range remaining {
		left[p] = true
	}
	res := DeleteResult{Deleted: []string{}, Failed: remaining}
	for _, p := range keep {
		if !left[p] {
			res.Deleted = append(res.Deleted, p)
		}
	}
	res.Freed = t.Remove(res.Deleted)
	a.afterDelete(name, res.Freed)
	a.info(i18n.T("log.deleted", len(res.Deleted), wsl.HumanBytes(res.Freed)))
	if len(res.Failed) > 0 {
		msg := i18n.T("err.deleteFailed", len(res.Failed))
		if delErr != nil {
			msg += " : " + delErr.Error()
		}
		return res, errors.New(msg)
	}
	return res, nil
}

// afterDelete cumule l'espace libéré et rafraîchit la mesure df utilisée par l'onglet Espace.
func (a *App) afterDelete(name string, freed int64) {
	a.mu.Lock()
	a.freed[strings.ToLower(name)] += freed
	a.mu.Unlock()
	if u, err := a.m.Usage(a.ctx, name); err == nil {
		a.setUsage(name, &u)
	}
}
