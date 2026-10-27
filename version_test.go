package main

import (
	"regexp"
	"testing"
)

func TestAppVersion(t *testing.T) {
	if v := appVersion(); !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(v) {
		t.Errorf("info.productVersion de wails.json doit être de la forme X.Y.Z, lu %q", v)
	}
	version = "9.9.9"
	defer func() { version = "" }()
	if appVersion() != "9.9.9" {
		t.Error("la version passée par -ldflags doit primer")
	}
}
