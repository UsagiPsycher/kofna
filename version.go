package main

import (
	_ "embed"
	"encoding/json"
)

// La version vient de info.productVersion dans wails.json : c'est aussi celle que
// Wails inscrit dans les propriétés de l'exécutable. Un seul endroit à modifier
// avant chaque build.

//go:embed wails.json
var wailsConfig []byte

// version peut être forcée au build : wails build -ldflags "-X main.version=1.2.3".
var version = ""

func appVersion() string {
	if version != "" {
		return version
	}
	var c struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(wailsConfig, &c); err != nil || c.Info.ProductVersion == "" {
		return "dev"
	}
	return c.Info.ProductVersion
}

// Version renvoie la version de l'application.
func (a *App) Version() string { return appVersion() }
