package i18n

import (
	"strings"
	"testing"
)

// Chaque message existe dans les trois langues avec les mêmes verbes de formatage.
func TestCatalogComplete(t *testing.T) {
	for key, m := range messages {
		for _, lang := range []string{"en", "fr", "es"} {
			s, ok := m[lang]
			if !ok || s == "" {
				t.Errorf("%s : traduction %s manquante", key, lang)
				continue
			}
			if verbs(s) != verbs(m["en"]) {
				t.Errorf("%s/%s : verbes %q ≠ %q", key, lang, verbs(s), verbs(m["en"]))
			}
		}
	}
}

func verbs(s string) string {
	var b strings.Builder
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '%' {
			b.WriteByte(s[i+1])
			i++
		}
	}
	return b.String()
}

func TestSetAndFallback(t *testing.T) {
	defer Set(Default)
	if Lang() != "en" || T("busy") != "another operation is already running" {
		t.Fatal("anglais par défaut attendu")
	}
	Set("FR")
	if T("task.analyze", "Ubuntu") != "Analyse de Ubuntu" {
		t.Errorf("%q", T("task.analyze", "Ubuntu"))
	}
	Set("de")
	if Lang() != "en" {
		t.Error("une langue inconnue doit retomber sur l'anglais")
	}
	if T("inexistant") != "inexistant" {
		t.Error("une clé inconnue est renvoyée telle quelle")
	}
}
