package wsl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"kofna/internal/i18n"
	"path"
	"strings"
)

// NcduVersion renvoie la version de ncdu installée dans la distribution, ou une erreur.
func (m *Manager) NcduVersion(ctx context.Context, name string) (string, error) {
	out, err := m.runQuiet(ctx, "wsl.exe", "-d", name, "-u", "root", "-e", "ncdu", "-v")
	if err != nil {
		return "", errors.New(i18n.T("err.ncduMissing", name))
	}
	return FirstLine(out), nil
}

// InstallNcdu installe ncdu avec le gestionnaire de paquets disponible.
func (m *Manager) InstallNcdu(ctx context.Context, name string) error {
	script := `set -e
if command -v apt-get >/dev/null; then apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq ncdu
elif command -v dnf >/dev/null; then dnf install -y ncdu
elif command -v apk >/dev/null; then apk add ncdu
elif command -v pacman >/dev/null; then pacman -S --noconfirm ncdu
elif command -v zypper >/dev/null; then zypper -n install ncdu
else echo "aucun gestionnaire de paquets reconnu" >&2; exit 1; fi`
	_, err := m.run(ctx, "wsl.exe", "-d", name, "-u", "root", "-e", "sh", "-c", script)
	return err
}

// ScanPath lance `ncdu -x -o-` sur un chemin de la distribution et lit l'export au fil de l'eau.
func (m *Manager) ScanPath(ctx context.Context, name, root, parentCat string, progress func(items, bytes int64)) (*Node, error) {
	args := []string{"-d", name, "-u", "root", "-e", "ncdu", "-x", "-0", "-o-", root}
	m.log("cmd", "wsl.exe "+strings.Join(quoteAll(args), " "))
	cmd := m.command(ctx, "wsl.exe", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	node, perr := ParseNcdu(out, parentCat, progress)
	if perr != nil {
		cmd.Process.Kill()
	}
	werr := cmd.Wait()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if msg := strings.TrimSpace(DecodeOutput(stderr.Bytes())); msg != "" {
		m.log("out", msg)
	}
	if perr != nil {
		return nil, fmt.Errorf("%s: %w", i18n.T("err.ncduRead"), perr)
	}
	if werr != nil {
		return nil, fmt.Errorf("ncdu : %w", werr)
	}
	return node, nil
}

// Delete supprime des chemins en root (`rm -rf --one-file-system`, sans shell)
// et renvoie ceux qui existent encore ensuite.
func (m *Manager) Delete(ctx context.Context, name string, paths []string) ([]string, error) {
	const batch = 100
	var runErr error
	for i := 0; i < len(paths); i += batch {
		end := min(i+batch, len(paths))
		args := append([]string{"-d", name, "-u", "root", "-e", "rm", "-rf", "--one-file-system", "--"}, paths[i:end]...)
		if _, err := m.run(ctx, "wsl.exe", args...); err != nil {
			runErr = err
		}
	}
	remaining, err := m.existing(ctx, name, paths)
	if err != nil {
		return paths, err
	}
	if len(remaining) > 0 && runErr != nil {
		return remaining, runErr
	}
	return remaining, nil
}

// existing renvoie, parmi des chemins, ceux qui existent dans la distribution.
func (m *Manager) existing(ctx context.Context, name string, paths []string) ([]string, error) {
	args := append([]string{"-d", name, "-u", "root", "-e", "sh", "-c",
		`for p; do if [ -e "$p" ] || [ -L "$p" ]; then printf '%s\0' "$p"; fi; done`, "sh"}, paths...)
	out, err := m.command(ctx, "wsl.exe", args...).Output()
	if err != nil {
		return nil, err
	}
	var res []string
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			res = append(res, p)
		}
	}
	return res, nil
}

// --- Nettoyages rapides ---------------------------------------------------

// Cleanup est un nettoyage proposé, avec la taille mesurée dans le dernier scan.
type Cleanup struct {
	ID       string `json:"id"`  // chemin concerné
	Key      string `json:"key"` // type de nettoyage, traduit par l'interface
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Category string `json:"category"`
	Command  string `json:"command"` // ce qui sera exécuté, pour l'infobulle
}

type cleanupDef struct {
	key  string
	path string
	cmd  []string // nil : suppression du dossier
}

const cleanupMin = 10 << 20

func cleanupDefs(t *Tree) []cleanupDef {
	defs := []cleanupDef{
		{"apt", "/var/cache/apt/archives", []string{"apt-get", "clean"}},
		{"journal", "/var/log/journal", []string{"journalctl", "--vacuum-size=50M"}},
	}
	for _, h := range t.Homes() {
		defs = append(defs,
			cleanupDef{"cache", h + "/.cache", nil},
			cleanupDef{"trash", h + "/.local/share/Trash", nil},
			cleanupDef{"npm", h + "/.npm/_cacache", nil},
			cleanupDef{"gradle", h + "/.gradle/caches", nil},
			cleanupDef{"cargo", h + "/.cargo/registry", nil},
			cleanupDef{"gomod", h + "/go/pkg/mod", nil},
			cleanupDef{"maven", h + "/.m2/repository", nil},
		)
	}
	return defs
}

// Cleanups liste les nettoyages utiles (au moins 10 Mo) d'après le scan.
func Cleanups(t *Tree) []Cleanup {
	var res []Cleanup
	for _, d := range cleanupDefs(t) {
		size := t.Size(d.path)
		if size < cleanupMin {
			continue
		}
		cmd := "rm -rf " + d.path
		if d.cmd != nil {
			cmd = strings.Join(d.cmd, " ")
		}
		res = append(res, Cleanup{ID: d.path, Key: d.key, Path: d.path, Size: size,
			Category: t.CategoryOf(d.path), Command: cmd})
	}
	return res
}

// RunCleanup exécute un nettoyage et met l'arbre à jour. Renvoie l'espace libéré.
func (m *Manager) RunCleanup(ctx context.Context, name string, t *Tree, id string) (int64, error) {
	var def *cleanupDef
	for _, d := range cleanupDefs(t) {
		if d.path == id {
			d := d
			def = &d
			break
		}
	}
	if def == nil {
		return 0, errors.New(i18n.T("err.unknownCleanup", id))
	}
	if def.cmd == nil {
		remaining, err := m.Delete(ctx, name, []string{def.path})
		if len(remaining) > 0 {
			return 0, errors.New(i18n.T("err.notDeleted", def.path, err))
		}
		return t.Remove([]string{def.path}), nil
	}
	args := append([]string{"-d", name, "-u", "root", "-e"}, def.cmd...)
	if _, err := m.run(ctx, "wsl.exe", args...); err != nil {
		return 0, err
	}
	sub, err := m.ScanPath(ctx, name, def.path, t.CategoryOf(path.Dir(def.path)), nil)
	if err != nil {
		// Le dossier a pu disparaître avec le nettoyage.
		if rem, _ := m.existing(ctx, name, []string{def.path}); len(rem) == 0 {
			return t.Remove([]string{def.path}), nil
		}
		return 0, err
	}
	return -t.Replace(def.path, sub), nil
}
