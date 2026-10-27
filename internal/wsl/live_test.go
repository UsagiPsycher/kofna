package wsl

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"
)

// Test en lecture seule contre le WSL réel de la machine :
//
//	set KOFNA_LIVE=1 && go test -run Live -v ./internal/wsl
//
// Il démarre la distribution par défaut (pour df) mais ne modifie rien.
func TestLiveReadOnly(t *testing.T) {
	if os.Getenv("KOFNA_LIVE") == "" {
		t.Skip("KOFNA_LIVE non défini")
	}
	ctx := context.Background()
	m := New(func(level, text string) { t.Logf("[%s] %s", level, text) })

	p := m.Platform(ctx)
	t.Logf("plateforme : %+v", p)

	ds, err := m.Distros(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) == 0 {
		t.Fatal("aucune distribution")
	}
	for _, d := range ds {
		t.Logf("%s v%d %s défaut=%v sparse=%v vhd=%s fichier=%s disque=%s locked=%v",
			d.Name, d.Version, d.State, d.Default, d.Sparse, d.VhdPath,
			HumanBytes(d.FileSize), HumanBytes(d.SizeOnDisk), fileLocked(d.VhdPath))
	}
	u, err := m.Usage(ctx, ds[0].Name)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("usage %s : utilisé %s / %s ; récupérable estimé %s",
		ds[0].Name, HumanBytes(u.Used), HumanBytes(u.Total), HumanBytes(ds[0].SizeOnDisk-u.Used))
}

// Scan ncdu réel (lecture seule) : KOFNA_LIVE_SCAN=/ go test -run LiveScan -v ./internal/wsl
func TestLiveScan(t *testing.T) {
	root := os.Getenv("KOFNA_LIVE_SCAN")
	if root == "" {
		t.Skip("KOFNA_LIVE_SCAN non défini")
	}
	ctx := context.Background()
	m := New(func(level, text string) { t.Logf("[%s] %s", level, text) })
	ds, err := m.Distros(ctx)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	n, err := m.ScanPath(ctx, ds[0].Name, root, "", func(items, b int64) {})
	if err != nil {
		t.Fatal(err)
	}
	var ms runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&ms)
	tr := &Tree{Root: n}
	total, items := tr.Stats()
	t.Logf("%s : %s, %d éléments en %s, tas %s", root, HumanBytes(total), items,
		time.Since(start).Round(time.Millisecond), HumanBytes(int64(ms.HeapAlloc)))
	v, _ := tr.View(root, 300, 40)
	for _, c := range v.Node.Children[:min(8, len(v.Node.Children))] {
		t.Logf("  %-30s %10s %s", c.Name, HumanBytes(c.Size), c.Category)
	}
	t.Logf("catégories : %v", v.Categories)
	for _, f := range tr.TopFiles(5) {
		t.Logf("  gros fichier %s %s", HumanBytes(f.Size), f.Path)
	}
	for _, c := range Cleanups(tr) {
		t.Logf("  nettoyage %s %s %s", c.Key, c.Path, HumanBytes(c.Size))
	}
}
