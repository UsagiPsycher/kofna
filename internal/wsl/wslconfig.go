package wsl

import (
	"strings"
)

// Lecture/écriture minimale du fichier %USERPROFILE%\.wslconfig (format INI).
// On ne touche qu'à la clé demandée et on garde le reste du fichier intact.

// IniValue renvoie la valeur d'une clé dans une section (insensible à la casse).
func IniValue(content, section, key string) (string, bool) {
	cur := ""
	for _, line := range strings.Split(normalize(content), "\n") {
		t := strings.TrimSpace(line)
		if t == "" || t[0] == '#' || t[0] == ';' {
			continue
		}
		if t[0] == '[' && strings.HasSuffix(t, "]") {
			cur = strings.TrimSpace(t[1 : len(t)-1])
			continue
		}
		if !strings.EqualFold(cur, section) {
			continue
		}
		k, v, ok := strings.Cut(t, "=")
		if ok && strings.EqualFold(strings.TrimSpace(k), key) {
			return stripComment(strings.TrimSpace(v)), true
		}
	}
	return "", false
}

func stripComment(v string) string {
	if i := strings.IndexAny(v, "#;"); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	return v
}

// SetIniValue remplace ou ajoute `key=value` dans `section`, en créant la section
// si besoin. Les fins de ligne CRLF sont conservées si le fichier en utilisait.
func SetIniValue(content, section, key, value string) string {
	eol := "\n"
	if strings.Contains(content, "\r\n") {
		eol = "\r\n"
	}
	lines := strings.Split(normalize(content), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	cur := ""
	sectionEnd := -1 // index après la dernière ligne non vide de la section
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if t != "" && t[0] == '[' && strings.HasSuffix(t, "]") {
			cur = strings.TrimSpace(t[1 : len(t)-1])
			if strings.EqualFold(cur, section) {
				sectionEnd = i + 1
			}
			continue
		}
		if !strings.EqualFold(cur, section) {
			continue
		}
		if t != "" {
			sectionEnd = i + 1
		}
		if t == "" || t[0] == '#' || t[0] == ';' {
			continue
		}
		if k, _, ok := strings.Cut(t, "="); ok && strings.EqualFold(strings.TrimSpace(k), key) {
			lines[i] = key + "=" + value
			return strings.Join(lines, eol) + eol
		}
	}

	entry := key + "=" + value
	if sectionEnd >= 0 {
		lines = append(lines[:sectionEnd], append([]string{entry}, lines[sectionEnd:]...)...)
	} else {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, "["+section+"]", entry)
	}
	return strings.Join(lines, eol) + eol
}
