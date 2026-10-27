package wsl

import (
	"strings"
	"testing"
)

// Export réduit au format de ncdu 1.14 (`ncdu -x -0 -o- /`).
const ncduSample = `[1,1,{"progname":"ncdu","progver":"1.14.1","timestamp":1793101676},
[{"name":"/","asize":4096,"dsize":4096,"dev":2128,"ino":2},
 [{"name":"home","asize":4096,"dsize":4096,"ino":3},
  [{"name":"usagi","asize":4096,"dsize":4096,"ino":4},
   [{"name":".cache","asize":4096,"dsize":4096,"ino":5},
    {"name":"pip.whl","asize":3000000,"dsize":3002368,"ino":6},
    {"name":"petit","asize":10,"dsize":4096,"ino":7}],
   [{"name":"projet","asize":4096,"dsize":4096,"ino":8},
    [{"name":"node_modules","asize":4096,"dsize":4096,"ino":9},
     {"name":"gros é.js","asize":2000000,"dsize":2002944,"ino":10}],
    {"name":"build.log","asize":5000000,"dsize":5001216,"ino":11}],
   {"name":"film.mkv","asize":9000000,"dsize":9003008,"ino":12}]],
 [{"name":"usr","asize":4096,"dsize":4096,"ino":13},
  {"name":"libx.so","asize":4000000,"dsize":4001792,"ino":14}],
 [{"name":"var","asize":4096,"dsize":4096,"ino":15},
  [{"name":"log","asize":4096,"dsize":4096,"ino":16},
   {"name":"syslog","asize":20,"dsize":4096,"ino":17}]],
 {"name":"mnt","asize":4096,"excluded":"otherfs","ino":18},
 {"name":"lien","asize":4,"ino":19,"notreg":true}]]`

func sampleTree(t *testing.T) *Tree {
	t.Helper()
	var calls int
	root, err := ParseNcdu(strings.NewReader(ncduSample), "", func(int64, int64) { calls++ })
	if err != nil {
		t.Fatal(err)
	}
	if calls == 0 {
		t.Error("progression jamais appelée")
	}
	return &Tree{Root: root}
}

func TestParseNcdu(t *testing.T) {
	tr := sampleTree(t)
	total, items := tr.Stats()
	// Somme de toutes les dsize, entrée exclue (/mnt) comprise à zéro.
	want := int64(4096*9 + 3002368 + 4096 + 2002944 + 5001216 + 9003008 + 4001792 + 4096 + 0)
	if total != want {
		t.Errorf("total %d, attendu %d", total, want)
	}
	if items != 17 {
		t.Errorf("items %d", items)
	}
	cases := map[string]string{
		"/home/usagi/.cache":                        CatCache,
		"/home/usagi/.cache/pip.whl":                CatCache,
		"/home/usagi/projet":                        CatData,
		"/home/usagi/projet/node_modules":           CatDeps,
		"/home/usagi/projet/node_modules/gros é.js": CatDeps,
		"/home/usagi/projet/build.log":              CatLogs,
		"/usr":                                      CatSystem,
		"/usr/libx.so":                              CatSystem,
		"/var/log":                                  CatLogs,
	}
	for p, cat := range cases {
		if !tr.Exists(p) {
			t.Errorf("%s absent", p)
			continue
		}
		if got := tr.CategoryOf(p); got != cat {
			t.Errorf("%s : %s, attendu %s", p, got, cat)
		}
	}
	// Les petits fichiers sont regroupés, pas des nœuds.
	if tr.Exists("/home/usagi/.cache/petit") || tr.Exists("/mnt") || tr.Exists("/lien") {
		t.Error("petit fichier, exclusion ou lien présents comme nœuds")
	}
}

func TestViewAndCategories(t *testing.T) {
	tr := sampleTree(t)
	v, err := tr.View("/", 300, 40)
	if err != nil {
		t.Fatal(err)
	}
	if v.Node.Children[0].Name != "home" {
		t.Errorf("tri par taille : %s en premier", v.Node.Children[0].Name)
	}
	var sum int64
	for _, s := range v.Categories {
		sum += s
	}
	if total, _ := tr.Stats(); sum != total {
		t.Errorf("catégories %d ≠ total %d", sum, total)
	}
	if v.Categories[CatCache] != 4096+3002368+4096 {
		t.Errorf("cache : %d", v.Categories[CatCache])
	}
	home := v.Node.Children[0]
	if len(home.Children) == 0 || home.Children[0].Name != "usagi" {
		t.Errorf("deuxième niveau absent : %+v", home)
	}
	// Limite : les éléments au-delà sont regroupés.
	v2, _ := tr.View("/home/usagi", 1, 1)
	kinds := map[string]bool{}
	for _, c := range v2.Node.Children {
		kinds[c.Kind] = true
	}
	if !kinds["rest"] {
		t.Errorf("pas de regroupement « autres » : %+v", v2.Node.Children)
	}
}

func TestCategorizeTools(t *testing.T) {
	for name, want := range map[string]string{".vscode-server": CatDeps, ".rbenv": CatDeps, "miniconda3": CatDeps, "Documents": CatData} {
		if got := Categorize(CatData, "/home/u/"+name, name, true); got != want {
			t.Errorf("%s : %s", name, got)
		}
	}
}

func TestProtected(t *testing.T) {
	for p, want := range map[string]bool{
		"/":                  true,
		"/home":              true,
		"/home/usagi":        true,
		"/home/usagi/projet": false,
		"/usr/lib":           true,
		"/var/log":           true,
		"/var/log/syslog.1":  false,
		"/tmp/x":             false,
		"/home/usagi/�x":     true,
	} {
		if got := Protected(p, Categorize("", p, p[strings.LastIndex(p, "/")+1:], true)); got != want {
			t.Errorf("%s : %v", p, got)
		}
	}
}

func TestDeleteBookkeeping(t *testing.T) {
	tr := sampleTree(t)
	before, _ := tr.Stats()
	keep, size, err := tr.CheckDeletable([]string{"/home/usagi/projet", "/home/usagi/projet/build.log", "/home/usagi/film.mkv"})
	if err != nil {
		t.Fatal(err)
	}
	if len(keep) != 2 {
		t.Errorf("le chemin inclus aurait dû être retiré : %v", keep)
	}
	if _, _, err := tr.CheckDeletable([]string{"/usr"}); err == nil {
		t.Error("/usr devrait être refusé")
	}
	freed := tr.Remove(keep)
	if freed != size {
		t.Errorf("libéré %d, annoncé %d", freed, size)
	}
	after, _ := tr.Stats()
	if before-after != freed || tr.Exists("/home/usagi/projet") {
		t.Errorf("arbre non mis à jour : %d → %d", before, after)
	}
	if tr.Size("/home") != tr.Size("/home/usagi")+4096 {
		t.Error("tailles des parents incohérentes")
	}
}

func TestTopFilesAndCleanups(t *testing.T) {
	tr := sampleTree(t)
	top := tr.TopFiles(3)
	if len(top) != 3 || top[0].Name != "film.mkv" || top[0].Path != "/home/usagi/film.mkv" {
		t.Errorf("%+v", top)
	}
	// .cache fait ~3 Mo : sous le seuil des nettoyages proposés.
	if c := Cleanups(tr); len(c) != 0 {
		t.Errorf("%+v", c)
	}
	sub, err := ParseNcdu(strings.NewReader(`[1,1,{},[{"name":"/home/usagi/.cache","dsize":4096}]]`), CatCache, nil)
	if err != nil {
		t.Fatal(err)
	}
	if d := tr.Replace("/home/usagi/.cache", sub); d != -(3002368 + 4096) {
		t.Errorf("variation %d", d)
	}
	if tr.Size("/home/usagi/.cache") != 4096 || !tr.Exists("/home/usagi/.cache") {
		t.Error("remplacement raté")
	}
}
