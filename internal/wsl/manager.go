package wsl

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"kofna/internal/i18n"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf16"
)

// Logger reçoit le journal des opérations : niveau (cmd, out, info, warn, error) et texte.
type Logger func(level, text string)

// Progress reçoit l'avancement d'une copie ou d'un export (total = 0 si inconnu).
type Progress func(done, total int64)

// Manager exécute les commandes WSL et les outils Windows associés.
type Manager struct {
	log Logger
}

func New(log Logger) *Manager {
	if log == nil {
		log = func(string, string) {}
	}
	return &Manager{log: log}
}

func (m *Manager) command(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	// Demande à wsl.exe une sortie UTF-8 plutôt qu'UTF-16 (DecodeOutput gère les deux).
	cmd.Env = append(os.Environ(), "WSL_UTF8=1")
	hideWindow(cmd)
	return cmd
}

// run lance une commande en journalisant la ligne de commande et sa sortie.
func (m *Manager) run(ctx context.Context, name string, args ...string) (string, error) {
	m.log("cmd", name+" "+strings.Join(quoteAll(args), " "))
	out, err := m.command(ctx, name, args...).CombinedOutput()
	text := strings.TrimSpace(DecodeOutput(out))
	if text != "" {
		m.log("out", text)
	}
	if err != nil {
		if ctx.Err() != nil {
			return text, ctx.Err()
		}
		if line := FirstLine(text); line != "" {
			return text, fmt.Errorf("%s : %s", name, line)
		}
		return text, fmt.Errorf("%s : %w", name, err)
	}
	return text, nil
}

// runQuiet lance une commande sans rien journaliser (sondes, rafraîchissements).
func (m *Manager) runQuiet(ctx context.Context, name string, args ...string) (string, error) {
	out, err := m.command(ctx, name, args...).CombinedOutput()
	return strings.TrimSpace(DecodeOutput(out)), err
}

func quoteAll(args []string) []string {
	q := make([]string, len(args))
	for i, a := range args {
		if a == "" || strings.ContainsAny(a, " \t") {
			a = `"` + a + `"`
		}
		q[i] = a
	}
	return q
}

// Platform sonde les capacités de la machine.
func (m *Manager) Platform(ctx context.Context) Platform {
	var p Platform
	if v, err := m.runQuiet(ctx, "wsl.exe", "--version"); err == nil {
		p.WSLVersion = ParseVersion(v)
	}
	help, _ := m.runQuiet(ctx, "wsl.exe", "--help")
	p.NativeCompact = HelpHasOption(help, "--compact")
	p.NativeSparse = HelpHasOption(help, "--set-sparse")
	if out, err := m.runQuiet(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
		"if (Get-Command Optimize-VHD -ErrorAction SilentlyContinue) { 'yes' } else { 'no' }"); err == nil {
		p.OptimizeVHD = strings.TrimSpace(out) == "yes"
	}
	p.IsAdmin = isAdmin()
	p.WslConfigPath, p.WslConfigSparse, _ = ReadWslConfigSparse()
	return p
}

// Distros liste les distributions : le registre donne l'emplacement du VHDX,
// `wsl -l -v` donne l'état courant.
func (m *Manager) Distros(ctx context.Context) ([]Distro, error) {
	regs, defID, err := registryDistros()
	if err != nil {
		return nil, err
	}
	states := map[string]ListEntry{}
	if out, err := m.runQuiet(ctx, "wsl.exe", "--list", "--verbose"); err == nil {
		for _, e := range ParseListVerbose(out) {
			states[strings.ToLower(e.Name)] = e
		}
	}
	res := make([]Distro, 0, len(regs))
	for _, r := range regs {
		d := Distro{
			ID:         r.ID,
			Name:       r.Name,
			Version:    r.Version,
			State:      "Stopped",
			Default:    r.ID == defID,
			BasePath:   TrimLongPath(r.BasePath),
			DefaultUID: r.DefaultUID,
			Managed:    IsManaged(r.Name),
		}
		if e, ok := states[strings.ToLower(r.Name)]; ok {
			d.State = e.State
			d.Version = e.Version
		}
		if d.Version == 2 {
			vhd := r.VhdFile
			if vhd == "" {
				vhd = "ext4.vhdx"
			}
			d.VhdPath = filepath.Join(d.BasePath, vhd)
			fillSizes(&d)
		}
		res = append(res, d)
	}
	sort.Slice(res, func(i, j int) bool {
		if res[i].Default != res[j].Default {
			return res[i].Default
		}
		return strings.ToLower(res[i].Name) < strings.ToLower(res[j].Name)
	})
	return res, nil
}

func fillSizes(d *Distro) {
	if st, err := os.Stat(d.VhdPath); err == nil {
		d.FileSize = st.Size()
		d.SizeOnDisk = st.Size()
	}
	if n, err := allocatedSize(d.VhdPath); err == nil {
		d.SizeOnDisk = n
	}
	d.Sparse = isSparse(d.VhdPath)
}

// Distro renvoie une distribution par son nom.
func (m *Manager) Distro(ctx context.Context, name string) (Distro, error) {
	all, err := m.Distros(ctx)
	if err != nil {
		return Distro{}, err
	}
	for _, d := range all {
		if strings.EqualFold(d.Name, name) {
			return d, nil
		}
	}
	return Distro{}, errors.New(i18n.T("err.distroNotFound", name))
}

// Usage mesure l'espace utilisé dans Linux (démarre la distribution si besoin).
func (m *Manager) Usage(ctx context.Context, name string) (Usage, error) {
	out, err := m.run(ctx, "wsl.exe", "-d", name, "-u", "root", "--", "df", "-P", "-k", "/")
	if err != nil {
		return Usage{}, err
	}
	return ParseDF(out)
}

// Trim lance fstrim sur la racine et renvoie le volume signalé comme libéré.
func (m *Manager) Trim(ctx context.Context, name string) (int64, error) {
	out, err := m.run(ctx, "wsl.exe", "-d", name, "-u", "root", "--", "fstrim", "-v", "/")
	if err != nil {
		return 0, err
	}
	return ParseFstrim(out)
}

// Shutdown arrête toutes les distributions et la VM WSL 2, puis attend que le
// VHDX donné (s'il y en a un) soit libéré.
func (m *Manager) Shutdown(ctx context.Context, vhd string) error {
	if _, err := m.run(ctx, "wsl.exe", "--shutdown"); err != nil {
		return err
	}
	return m.waitReleased(ctx, vhd)
}

// Terminate arrête une seule distribution.
func (m *Manager) Terminate(ctx context.Context, name, vhd string) error {
	if _, err := m.run(ctx, "wsl.exe", "--terminate", name); err != nil {
		return err
	}
	return m.waitReleased(ctx, vhd)
}

func (m *Manager) waitReleased(ctx context.Context, vhd string) error {
	if vhd == "" {
		return nil
	}
	deadline := time.Now().Add(45 * time.Second)
	for fileLocked(vhd) {
		if time.Now().After(deadline) {
			return errors.New(i18n.T("err.stillLocked", vhd))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return nil
}

// Compact compacte le VHDX avec la méthode choisie. WSL doit être arrêté.
func (m *Manager) Compact(ctx context.Context, d Distro, method CompactMethod) error {
	if d.VhdPath == "" {
		return errors.New(i18n.T("err.wsl1"))
	}
	switch method {
	case CompactNative:
		_, err := m.run(ctx, "wsl.exe", "--manage", d.Name, "--compact")
		return err
	case CompactDiskPart:
		return m.runElevated(ctx, "DiskPart", diskpartScript(d.VhdPath))
	case CompactOptimize:
		p := psQuote(d.VhdPath)
		return m.runElevated(ctx, "Optimize-VHD", strings.Join([]string{
			"Mount-VHD -Path " + p + " -ReadOnly",
			"try { Optimize-VHD -Path " + p + " -Mode Full *>&1 | Out-File -FilePath $log -Append -Encoding utf8 }",
			"finally { Dismount-VHD -Path " + p + " }",
			"'Optimize-VHD: OK' | Out-File -FilePath $log -Append -Encoding utf8",
		}, "\r\n"))
	}
	return errors.New(i18n.T("err.unknownMethod", method))
}

func diskpartScript(vhd string) string {
	sel := psQuote(`select vdisk file="` + vhd + `"`)
	return strings.Join([]string{
		"$dp = Join-Path $dir 'compact.txt'",
		"@(" + sel + ", 'attach vdisk readonly', 'compact vdisk', 'detach vdisk', 'exit') | Out-File -FilePath $dp -Encoding oem",
		"& diskpart.exe /s $dp 2>&1 | Out-File -FilePath $log -Append -Encoding utf8",
		"$code = $LASTEXITCODE",
		"if ($code -ne 0) {",
		"  # En cas d'échec après attach, on détache pour ne pas laisser le disque monté.",
		"  $dd = Join-Path $dir 'detach.txt'",
		"  @(" + sel + ", 'detach vdisk', 'exit') | Out-File -FilePath $dd -Encoding oem",
		"  & diskpart.exe /s $dd 2>&1 | Out-File -FilePath $log -Append -Encoding utf8",
		"}",
		"exit $code",
	}, "\r\n")
}

func psQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// runElevated exécute un script PowerShell en administrateur (une invite UAC
// s'affiche si l'application ne l'est pas déjà). Le script dispose de $dir
// (dossier temporaire) et $log (fichier dont le contenu est remonté au journal).
func (m *Manager) runElevated(ctx context.Context, title, body string) error {
	dir, err := os.MkdirTemp("", "kofna-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	logPath := filepath.Join(dir, "out.log")
	script := filepath.Join(dir, "run.ps1")
	full := strings.Join([]string{
		"$ErrorActionPreference = 'Stop'",
		"$dir = " + psQuote(dir),
		"$log = " + psQuote(logPath),
		"try {",
		body,
		"} catch {",
		"  $_ | Out-String | Out-File -FilePath $log -Append -Encoding utf8",
		"  exit 1",
		"}",
	}, "\r\n")
	// BOM UTF-8 : sans lui, PowerShell 5 lit le script en ANSI et casse les accents.
	if err := os.WriteFile(script, append([]byte{0xEF, 0xBB, 0xBF}, full...), 0o600); err != nil {
		return err
	}

	var cmd *exec.Cmd
	if isAdmin() {
		m.log("cmd", "powershell.exe -File "+script+" ("+title+")")
		cmd = m.command(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script)
	} else {
		m.log("info", i18n.T("log.uac", title))
		launcher := "$p = Start-Process -FilePath powershell.exe -ArgumentList " +
			psQuote(`-NoProfile -NonInteractive -ExecutionPolicy Bypass -File "`+script+`"`) +
			" -Verb RunAs -WindowStyle Hidden -PassThru -Wait; exit $p.ExitCode"
		cmd = m.command(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-EncodedCommand", encodePS(launcher))
	}
	out, runErr := cmd.CombinedOutput()
	if b, err := os.ReadFile(logPath); err == nil {
		if text := strings.TrimSpace(DecodeOutput(b)); text != "" {
			m.log("out", text)
		}
	}
	if runErr != nil {
		text := strings.TrimSpace(DecodeOutput(out))
		if text != "" {
			m.log("out", text)
		}
		low := strings.ToLower(text)
		if strings.Contains(low, "annul") || strings.Contains(low, "cancel") {
			return errors.New(i18n.T("err.uacDenied", title))
		}
		return errors.New(i18n.T("err.elevatedFailed", title, runErr))
	}
	return nil
}

// encodePS prépare un script pour -EncodedCommand (UTF-16LE en base64),
// ce qui évite tout problème de guillemets entre Go, cmd et PowerShell.
func encodePS(s string) string {
	u := utf16.Encode([]rune(s))
	b := make([]byte, len(u)*2)
	for i, c := range u {
		b[2*i] = byte(c)
		b[2*i+1] = byte(c >> 8)
	}
	return base64.StdEncoding.EncodeToString(b)
}

// SetSparse active ou désactive le mode sparse d'une distribution (elle est arrêtée avant).
func (m *Manager) SetSparse(ctx context.Context, d Distro, on bool) error {
	if err := m.Terminate(ctx, d.Name, d.VhdPath); err != nil {
		return err
	}
	_, err := m.run(ctx, "wsl.exe", "--manage", d.Name, "--set-sparse", fmt.Sprint(on))
	return err
}

// Backup copie le VHDX dans destDir. WSL doit être arrêté.
func (m *Manager) Backup(ctx context.Context, d Distro, destDir string, progress Progress) (string, error) {
	if d.VhdPath == "" {
		return "", errors.New(i18n.T("err.wsl1"))
	}
	if free, err := freeSpace(destDir); err == nil && free < d.FileSize {
		return "", errors.New(i18n.T("err.noSpace", destDir, HumanBytes(free), HumanBytes(d.FileSize)))
	}
	dest := filepath.Join(destDir, fmt.Sprintf("%s-ext4-%s.vhdx", d.Name, time.Now().Format("20060102-150405")))
	m.log("info", i18n.T("log.copy", d.VhdPath, dest))
	if err := copyFile(ctx, d.VhdPath, dest, d.FileSize, progress); err != nil {
		os.Remove(dest)
		return "", err
	}
	return dest, nil
}

func copyFile(ctx context.Context, src, dst string, total int64, progress Progress) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	buf := make([]byte, 8<<20)
	var done int64
	last := time.Now()
	for {
		if err := ctx.Err(); err != nil {
			out.Close()
			return err
		}
		n, rerr := in.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				out.Close()
				return werr
			}
			done += int64(n)
			if time.Since(last) > 250*time.Millisecond {
				progress(done, total)
				last = time.Now()
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			out.Close()
			return rerr
		}
	}
	progress(done, total)
	return out.Close()
}

// Export lance `wsl --export` et suit la taille de l'archive pendant l'opération.
// expected sert d'estimation pour la barre de progression (espace utilisé dans Linux).
func (m *Manager) Export(ctx context.Context, name, file string, expected int64, progress Progress) error {
	stop := make(chan struct{})
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				if st, err := os.Stat(file); err == nil {
					progress(st.Size(), expected)
				}
			}
		}
	}()
	_, err := m.run(ctx, "wsl.exe", "--export", name, file)
	close(stop)
	if err != nil {
		os.Remove(file)
		return err
	}
	if st, err := os.Stat(file); err == nil {
		progress(st.Size(), st.Size())
	}
	return nil
}

// Reimport remplace une distribution par le contenu d'une archive exportée :
// désinscription, import au même emplacement, puis restauration de l'utilisateur
// par défaut, de la distribution par défaut et du mode sparse.
func (m *Manager) Reimport(ctx context.Context, d Distro, tarPath string) error {
	if d.Managed {
		return errors.New(i18n.T("err.managedReimport", d.Name))
	}
	st, err := os.Stat(tarPath)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("err.archiveMissing"), err)
	}
	if st.Size() < 1<<20 {
		return errors.New(i18n.T("err.archiveSuspect", HumanBytes(st.Size())))
	}
	if _, err := m.run(ctx, "wsl.exe", "--shutdown"); err != nil {
		return err
	}
	if _, err := m.run(ctx, "wsl.exe", "--unregister", d.Name); err != nil {
		return err
	}
	// À partir d'ici, la distribution n'existe plus : on ne doit plus abandonner
	// sur une annulation, seulement sur un échec réel.
	ctx = context.WithoutCancel(ctx)
	if _, err := m.run(ctx, "wsl.exe", "--import", d.Name, d.BasePath, tarPath, "--version", "2"); err != nil {
		return errors.New(i18n.T("err.importFailed", err, tarPath, d.Name, d.BasePath, tarPath))
	}
	if d.DefaultUID != 0 {
		if err := setRegistryDefaultUID(d.Name, d.DefaultUID); err != nil {
			m.log("warn", i18n.T("log.uidNotRestored", err.Error()))
		} else {
			m.log("info", i18n.T("log.uidRestored", d.DefaultUID))
		}
	}
	if d.Default {
		m.run(ctx, "wsl.exe", "--set-default", d.Name)
	}
	if d.Sparse {
		m.run(ctx, "wsl.exe", "--manage", d.Name, "--set-sparse", "true")
	}
	return nil
}

// Reveal ouvre l'explorateur sur un fichier.
func (m *Manager) Reveal(path string) error { return revealInExplorer(path) }

// FreeSpace renvoie l'espace libre du volume contenant dir.
func FreeSpace(dir string) (int64, error) { return freeSpace(dir) }

// HumanBytes formate une taille dans la langue courante (1 Go = 1024³ octets, comme l'Explorateur).
func HumanBytes(n int64) string {
	const unit = 1024
	all, sep := i18n.Units()
	if n < unit && n > -unit {
		return fmt.Sprintf("%d %s", n, all[0])
	}
	f := float64(n)
	units := all[1:]
	i := -1
	for (f >= unit || f <= -unit) && i < len(units)-1 {
		f /= unit
		i++
	}
	return strings.Replace(fmt.Sprintf("%.1f %s", f, units[i]), ".", sep, 1)
}

// .wslconfig ---------------------------------------------------------------

// WslConfigPath renvoie le chemin de %USERPROFILE%\.wslconfig.
func WslConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".wslconfig"), nil
}

// ReadWslConfigSparse lit [experimental] sparseVhd.
func ReadWslConfigSparse() (string, bool, error) {
	p, err := WslConfigPath()
	if err != nil {
		return "", false, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return p, false, nil
		}
		return p, false, err
	}
	v, _ := IniValue(DecodeOutput(b), "experimental", "sparseVhd")
	return p, strings.EqualFold(v, "true"), nil
}

// WriteWslConfigSparse écrit [experimental] sparseVhd, après une copie .bak.
func (m *Manager) WriteWslConfigSparse(on bool) error {
	p, err := WslConfigPath()
	if err != nil {
		return err
	}
	b, err := os.ReadFile(p)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil {
		if err := os.WriteFile(p+".bak", b, 0o644); err != nil {
			return err
		}
	}
	// On garde le BOM et les fins de ligne d'origine : SetIniValue respecte les CRLF.
	bom := []byte{0xEF, 0xBB, 0xBF}
	content := string(bytes.TrimPrefix(b, bom))
	next := SetIniValue(content, "experimental", "sparseVhd", fmt.Sprint(on))
	if bytes.HasPrefix(b, bom) {
		next = string(bom) + next
	}
	if err := os.WriteFile(p, []byte(next), 0o644); err != nil {
		return err
	}
	m.log("info", i18n.T("log.wslconfig", p, on))
	return nil
}
