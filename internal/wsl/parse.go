package wsl

import (
	"bytes"
	"errors"
	"kofna/internal/i18n"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"
)

// DecodeOutput convertit la sortie d'un outil Windows en texte.
// wsl.exe écrit en UTF-16LE (sauf si WSL_UTF8=1 est pris en compte), PowerShell 5
// ajoute parfois un BOM ; on gère les deux cas et on normalise les fins de ligne.
func DecodeOutput(b []byte) string {
	switch {
	case bytes.HasPrefix(b, []byte{0xFF, 0xFE}):
		b = b[2:]
		return normalize(decodeUTF16LE(b))
	case bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}):
		b = b[3:]
	case looksUTF16LE(b):
		return normalize(decodeUTF16LE(b))
	}
	return normalize(string(b))
}

func looksUTF16LE(b []byte) bool {
	if len(b) < 4 {
		return false
	}
	n := len(b)
	if n > 200 {
		n = 200
	}
	zeros := 0
	for i := 1; i < n; i += 2 {
		if b[i] == 0 {
			zeros++
		}
	}
	return zeros*2 >= n/2
}

func decodeUTF16LE(b []byte) string {
	u := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		u = append(u, uint16(b[i])|uint16(b[i+1])<<8)
	}
	return string(utf16.Decode(u))
}

func normalize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.ReplaceAll(s, "\x00", "")
}

// ListEntry est une ligne de `wsl --list --verbose`.
type ListEntry struct {
	Name    string
	State   string
	Version int
	Default bool
}

// ParseListVerbose lit la sortie de `wsl -l -v`. La première ligne est l'en-tête
// (traduit selon la langue de Windows), les suivantes : [*] NOM ÉTAT VERSION.
func ParseListVerbose(out string) []ListEntry {
	var res []ListEntry
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		if i == 0 {
			continue
		}
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		e := ListEntry{}
		if f[0] == "*" {
			e.Default = true
			f = f[1:]
		}
		if len(f) < 3 {
			continue
		}
		v, err := strconv.Atoi(f[len(f)-1])
		if err != nil {
			continue
		}
		e.Name = f[0]
		e.Version = v
		e.State = strings.Join(f[1:len(f)-1], " ")
		res = append(res, e)
	}
	return res
}

// ParseDF lit la sortie POSIX de `df -P -k /` (fonctionne aussi avec busybox).
func ParseDF(out string) (Usage, error) {
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 6 || f[len(f)-1] != "/" {
			continue
		}
		total, err1 := strconv.ParseInt(f[1], 10, 64)
		used, err2 := strconv.ParseInt(f[2], 10, 64)
		avail, err3 := strconv.ParseInt(f[3], 10, 64)
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		return Usage{Total: total * 1024, Used: used * 1024, Avail: avail * 1024}, nil
	}
	return Usage{}, errors.New(i18n.T("err.unexpectedDf", strings.TrimSpace(out)))
}

// util-linux : "/: 8.9 GiB (9566502912 bytes) trimmed"
// busybox    : "/: 9566502912 bytes trimmed"
var fstrimRe = regexp.MustCompile(`(\d+) bytes\)? trimmed`)

// ParseFstrim renvoie le nombre d'octets signalés comme libérés par `fstrim -v`.
func ParseFstrim(out string) (int64, error) {
	m := fstrimRe.FindStringSubmatch(out)
	if m == nil {
		return 0, errors.New(i18n.T("err.unexpectedFstrim", strings.TrimSpace(out)))
	}
	return strconv.ParseInt(m[1], 10, 64)
}

// HelpHasOption indique si `wsl --help` mentionne une option donnée.
func HelpHasOption(help, option string) bool {
	return strings.Contains(help, option)
}

var versionRe = regexp.MustCompile(`\d+(\.\d+)+`)

// ParseVersion extrait le numéro de `wsl --version` (« Version WSL : 3.0.3.0 » → « 3.0.3.0 »).
func ParseVersion(out string) string {
	if v := versionRe.FindString(FirstLine(out)); v != "" {
		return v
	}
	return FirstLine(out)
}

// FirstLine renvoie la première ligne non vide d'un texte.
func FirstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			return t
		}
	}
	return ""
}

// IsManaged repère les distributions gérées par un autre logiciel.
func IsManaged(name string) bool {
	return strings.HasPrefix(strings.ToLower(name), "docker-desktop")
}

// TrimLongPath retire le préfixe \\?\ que WSL met dans BasePath.
func TrimLongPath(p string) string {
	return strings.TrimPrefix(p, `\\?\`)
}
