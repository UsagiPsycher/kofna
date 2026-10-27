// Package wsl regroupe tout ce qui touche à WSL : inventaire des distributions,
// mesure de l'espace, trim, compactage du disque virtuel et options sparse.
//
// Les appels système (registre, taille réellement allouée, élévation UAC) sont
// regroupés dans platform.go.
package wsl

import (
	"time"
)

// Distro décrit une distribution WSL telle que vue depuis Windows.
type Distro struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Version    int    `json:"version"` // 1 ou 2
	State      string `json:"state"`   // Running, Stopped, Installing… (tel que renvoyé par wsl -l -v)
	Default    bool   `json:"default"`
	BasePath   string `json:"basePath"`
	VhdPath    string `json:"vhdPath"`    // vide pour WSL 1
	FileSize   int64  `json:"fileSize"`   // taille logique du fichier ext4.vhdx
	SizeOnDisk int64  `json:"sizeOnDisk"` // espace réellement alloué sur le disque Windows
	Sparse     bool   `json:"sparse"`
	DefaultUID uint32 `json:"defaultUid"`
	// Managed signale une distribution pilotée par un autre logiciel (Docker Desktop) :
	// on peut la mesurer et la compacter, mais pas l'exporter/réimporter.
	Managed bool `json:"managed"`
}

// Usage est l'occupation du système de fichiers racine, vue depuis Linux.
type Usage struct {
	Total int64 `json:"total"`
	Used  int64 `json:"used"`
	Avail int64 `json:"avail"`
}

// Snapshot fige la taille du disque virtuel à une étape donnée de la procédure.
type Snapshot struct {
	Step       string    `json:"step"`  // analyse, trim, compact, sparse, reimport
	Label      string    `json:"label"` // clé traduite par l'interface (beforeTrim, afterCompact…)
	At         time.Time `json:"at"`
	FileSize   int64     `json:"fileSize"`
	SizeOnDisk int64     `json:"sizeOnDisk"`
	Used       int64     `json:"used"` // 0 si inconnu à cette étape
}

// Platform résume ce que la machine sait faire.
type Platform struct {
	WSLVersion      string `json:"wslVersion"`
	NativeCompact   bool   `json:"nativeCompact"` // wsl --manage <distro> --compact
	NativeSparse    bool   `json:"nativeSparse"`  // wsl --manage <distro> --set-sparse
	OptimizeVHD     bool   `json:"optimizeVhd"`   // cmdlet Hyper-V présente
	IsAdmin         bool   `json:"isAdmin"`
	WslConfigPath   string `json:"wslConfigPath"`
	WslConfigSparse bool   `json:"wslConfigSparse"` // [experimental] sparseVhd=true
}

// CompactMethod identifie l'outil utilisé pour compacter le VHDX.
type CompactMethod string

const (
	CompactNative   CompactMethod = "wsl"      // wsl --manage <distro> --compact
	CompactDiskPart CompactMethod = "diskpart" // select/attach/compact/detach vdisk
	CompactOptimize CompactMethod = "optimize" // Optimize-VHD -Mode Full (Hyper-V)
)
