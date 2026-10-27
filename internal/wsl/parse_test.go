package wsl

import (
	"kofna/internal/i18n"
	"testing"
	"unicode/utf16"
)

func utf16le(s string, bom bool) []byte {
	var b []byte
	if bom {
		b = append(b, 0xFF, 0xFE)
	}
	for _, c := range utf16.Encode([]rune(s)) {
		b = append(b, byte(c), byte(c>>8))
	}
	return b
}

// Sortie réelle de `wsl -l -v` sur la machine de développement.
const listOut = "  NAME              STATE           VERSION\r\n* Ubuntu-18.04      Running         2\r\n  docker-desktop    Stopped         2\r\n  Legacy            Stopped         1\r\n"

func TestDecodeOutput(t *testing.T) {
	want := "  NAME              STATE           VERSION\n* Ubuntu-18.04      Running         2\n  docker-desktop    Stopped         2\n  Legacy            Stopped         1\n"
	for name, in := range map[string][]byte{
		"utf8":      []byte(listOut),
		"utf16":     utf16le(listOut, false),
		"utf16 bom": utf16le(listOut, true),
		"utf8 bom":  append([]byte{0xEF, 0xBB, 0xBF}, listOut...),
	} {
		if got := DecodeOutput(in); got != want {
			t.Errorf("%s : %q", name, got)
		}
	}
	if got := DecodeOutput(utf16le("Récupérer l’espace", false)); got != "Récupérer l’espace" {
		t.Errorf("accents : %q", got)
	}
}

func TestParseListVerbose(t *testing.T) {
	got := ParseListVerbose(DecodeOutput([]byte(listOut)))
	want := []ListEntry{
		{Name: "Ubuntu-18.04", State: "Running", Version: 2, Default: true},
		{Name: "docker-desktop", State: "Stopped", Version: 2},
		{Name: "Legacy", State: "Stopped", Version: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ligne %d : %+v, attendu %+v", i, got[i], want[i])
		}
	}
	// État sur plusieurs mots (Windows dans une autre langue).
	fr := "  NOM  ÉTAT  VERSION\n* Debian  En cours d'exécution  2\n"
	if e := ParseListVerbose(fr); len(e) != 1 || e[0].State != "En cours d'exécution" {
		t.Errorf("état multi-mots : %+v", e)
	}
}

func TestParseDF(t *testing.T) {
	out := "Filesystem     1024-blocks      Used Available Capacity Mounted on\n/dev/sdf        263112772 208558952  41116640      84% /\n"
	u, err := ParseDF(out)
	if err != nil {
		t.Fatal(err)
	}
	if u.Total != 263112772*1024 || u.Used != 208558952*1024 || u.Avail != 41116640*1024 {
		t.Errorf("%+v", u)
	}
	if _, err := ParseDF("df: /: No such file"); err == nil {
		t.Error("erreur attendue")
	}
}

func TestParseFstrim(t *testing.T) {
	cases := map[string]int64{
		"/: 8.9 GiB (9566502912 bytes) trimmed\n": 9566502912,
		"/: 9566502912 bytes trimmed":             9566502912,
		"/: 0 B (0 bytes) trimmed":                0,
	}
	for in, want := range cases {
		got, err := ParseFstrim(in)
		if err != nil || got != want {
			t.Errorf("%q : %d, %v", in, got, err)
		}
	}
	if _, err := ParseFstrim("fstrim: /: the discard operation is not supported"); err == nil {
		t.Error("erreur attendue")
	}
}

func TestHumanBytes(t *testing.T) {
	cases := map[int64]string{
		512:          "512 B",
		1536:         "1.5 KB",
		222740611072: "207.4 GB",
		-1 << 30:     "-1.0 GB",
	}
	for in, want := range cases {
		if got := HumanBytes(in); got != want {
			t.Errorf("%d : %q, attendu %q", in, got, want)
		}
	}
	i18n.Set("fr")
	defer i18n.Set(i18n.Default)
	if got := HumanBytes(222740611072); got != "207,4 Go" {
		t.Errorf("fr : %q", got)
	}
}

func TestIni(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"fichier vide", "", "[experimental]\nsparseVhd=true\n"},
		{"section absente", "[wsl2]\nmemory=8GB\n", "[wsl2]\nmemory=8GB\n\n[experimental]\nsparseVhd=true\n"},
		{"section présente", "[experimental]\nautoMemoryReclaim=gradual\n\n[wsl2]\nswap=0\n",
			"[experimental]\nautoMemoryReclaim=gradual\nsparseVhd=true\n\n[wsl2]\nswap=0\n"},
		{"clé présente", "[Experimental]\nSparseVhd = false # commentaire\n", "[Experimental]\nsparseVhd=true\n"},
		{"CRLF conservé", "[wsl2]\r\nmemory=8GB\r\n", "[wsl2]\r\nmemory=8GB\r\n\r\n[experimental]\r\nsparseVhd=true\r\n"},
	}
	for _, c := range cases {
		got := SetIniValue(c.in, "experimental", "sparseVhd", "true")
		if got != c.want {
			t.Errorf("%s :\n%q\nattendu\n%q", c.name, got, c.want)
		}
		if v, ok := IniValue(got, "experimental", "sparsevhd"); !ok || v != "true" {
			t.Errorf("%s : relecture %q %v", c.name, v, ok)
		}
	}
	if v, _ := IniValue("[experimental]\nsparseVhd=false ; non\n", "experimental", "sparseVhd"); v != "false" {
		t.Errorf("commentaire : %q", v)
	}
	if _, ok := IniValue("[wsl2]\nsparseVhd=true\n", "experimental", "sparseVhd"); ok {
		t.Error("mauvaise section")
	}
}

func TestParseVersion(t *testing.T) {
	for in, want := range map[string]string{
		"Version WSL : 3.0.3.0\nVersion du noyau : 6.18.54.1-1\n": "3.0.3.0",
		"WSL version: 2.4.13.0\n":                                 "2.4.13.0",
	} {
		if got := ParseVersion(in); got != want {
			t.Errorf("%q : %q", in, got)
		}
	}
}

func TestEncodePS(t *testing.T) {
	// "ab" en UTF-16LE = 61 00 62 00
	if got := encodePS("ab"); got != "YQBiAA==" {
		t.Errorf("%q", got)
	}
}
