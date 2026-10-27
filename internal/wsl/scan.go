package wsl

import (
	"bufio"
	"container/heap"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"kofna/internal/i18n"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
)

// Arborescence issue de `ncdu -o-` (export JSON). Pour tenir en mémoire sur un
// disque de plusieurs millions de fichiers, les fichiers de moins de
// SmallFileLimit sont regroupés par dossier ; dossiers et gros fichiers restent
// des nœuds à part entière.

const SmallFileLimit = 1 << 20

// Catégories : ce qu'on peut supprimer, et avec quel risque.
const (
	CatData   = "data"
	CatCache  = "cache"
	CatDeps   = "deps"
	CatLogs   = "logs"
	CatDocker = "docker"
	CatSystem = "system"
)

// Node est un dossier ou un gros fichier de l'arborescence scannée.
type Node struct {
	Name       string
	Parent     *Node
	Children   []*Node
	Dir        bool
	Size       int64 // espace disque (dsize) du sous-arbre
	Items      int64 // nombre d'entrées du sous-arbre, nœud compris
	Own        int64 // espace propre : l'entrée elle-même + petits fichiers regroupés
	SmallCount int64 // petits fichiers regroupés dans Own
	SmallSize  int64
	Cat        string
	ReadError  bool
}

// Path reconstruit le chemin absolu du nœud.
func (n *Node) Path() string {
	if n.Parent == nil {
		return n.Name
	}
	return path.Join(n.Parent.Path(), n.Name)
}

// Tree est le résultat d'un scan, partagé entre l'interface et les suppressions.
type Tree struct {
	mu        sync.RWMutex
	Root      *Node
	ScannedAt time.Time
	Duration  time.Duration
}

type parser struct {
	dec      *json.Decoder
	items    int64
	bytes    int64
	progress func(items, bytes int64)
}

// ParseNcdu lit un export ncdu. rootPath est le chemin scanné (ncdu le met dans
// le nom de la racine) et parentCat la catégorie héritée, pour un sous-scan.
func ParseNcdu(r io.Reader, parentCat string, progress func(items, bytes int64)) (*Node, error) {
	if progress == nil {
		progress = func(int64, int64) {}
	}
	p := &parser{dec: json.NewDecoder(bufio.NewReaderSize(r, 1<<20)), progress: progress}
	p.dec.UseNumber()
	if err := p.delim('['); err != nil {
		return nil, fmt.Errorf("invalid ncdu export: %w", err)
	}
	// Version majeure, mineure, puis métadonnées.
	for i := 0; i < 2; i++ {
		if _, err := p.dec.Token(); err != nil {
			return nil, err
		}
	}
	var meta map[string]any
	if err := p.dec.Decode(&meta); err != nil {
		return nil, err
	}
	if err := p.delim('['); err != nil {
		return nil, fmt.Errorf("ncdu export without root: %w", err)
	}
	root, err := p.dir(nil, parentCat)
	if err != nil {
		return nil, err
	}
	progress(p.items, p.bytes)
	return root, nil
}

func (p *parser) delim(want json.Delim) error {
	t, err := p.dec.Token()
	if err != nil {
		return err
	}
	if d, ok := t.(json.Delim); !ok || d != want {
		return fmt.Errorf("expected %q, got %v", want, t)
	}
	return nil
}

type entry struct {
	name      string
	dsize     int64
	excluded  bool
	readError bool
}

// info lit un objet d'entrée ; l'accolade ouvrante est déjà consommée si opened.
func (p *parser) info(opened bool) (entry, error) {
	if !opened {
		if err := p.delim('{'); err != nil {
			return entry{}, err
		}
	}
	var e entry
	for p.dec.More() {
		kt, err := p.dec.Token()
		if err != nil {
			return e, err
		}
		key, _ := kt.(string)
		vt, err := p.dec.Token()
		if err != nil {
			return e, err
		}
		switch key {
		case "name":
			e.name, _ = vt.(string)
		case "dsize":
			if n, ok := vt.(json.Number); ok {
				e.dsize, _ = n.Int64()
			}
		case "excluded":
			e.excluded = true
		case "read_error":
			e.readError = vt == true
		}
	}
	_, err := p.dec.Token() // '}'
	return e, err
}

// dir lit un dossier : '[' déjà consommé, puis l'objet du dossier et ses enfants.
func (p *parser) dir(parent *Node, parentCat string) (*Node, error) {
	e, err := p.info(false)
	if err != nil {
		return nil, err
	}
	n := &Node{Name: e.name, Parent: parent, Dir: true, Own: e.dsize, ReadError: e.readError}
	n.Cat = Categorize(parentCat, n.Path(), e.name, true)
	n.Size, n.Items = e.dsize, 1
	p.tick(e.dsize)
	for p.dec.More() {
		t, err := p.dec.Token()
		if err != nil {
			return nil, err
		}
		switch t {
		case json.Delim('['):
			c, err := p.dir(n, n.Cat)
			if err != nil {
				return nil, err
			}
			n.Children = append(n.Children, c)
			n.Size += c.Size
			n.Items += c.Items
		case json.Delim('{'):
			f, err := p.info(true)
			if err != nil {
				return nil, err
			}
			if f.excluded { // autre système de fichiers, /proc…
				continue
			}
			p.tick(f.dsize)
			n.Size += f.dsize
			n.Items++
			if f.dsize >= SmallFileLimit {
				fc := &Node{Name: f.name, Parent: n, Size: f.dsize, Items: 1, ReadError: f.readError}
				fc.Cat = Categorize(n.Cat, "", f.name, false)
				n.Children = append(n.Children, fc)
			} else {
				n.Own += f.dsize
				n.SmallCount++
				n.SmallSize += f.dsize
			}
		default:
			return nil, fmt.Errorf("unexpected element in ncdu export: %v", t)
		}
	}
	if err := p.delim(']'); err != nil {
		return nil, err
	}
	return n, nil
}

func (p *parser) tick(size int64) {
	p.items++
	p.bytes += size
	if p.items%25000 == 0 {
		p.progress(p.items, p.bytes)
	}
}

// --- Catégories et protections -------------------------------------------

var systemRoots = []string{"/usr", "/bin", "/sbin", "/lib", "/lib32", "/lib64", "/libx32", "/etc", "/boot",
	"/var/lib/dpkg", "/var/lib/apt", "/var/lib/rpm", "/var/lib/pacman", "/snap", "/init"}

var cacheNames = map[string]bool{".cache": true, "__pycache__": true, ".npm": true, ".pnpm-store": true,
	".yarn": true, ".Trash": true, "Trash": true, ".sass-cache": true, ".pytest_cache": true, ".mypy_cache": true,
	".ruff_cache": true, ".parcel-cache": true, ".next": true, ".nuxt": true, ".turbo": true}

var depsNames = map[string]bool{"node_modules": true, ".venv": true, "venv": true, "site-packages": true,
	".tox": true, "bower_components": true, ".gradle": true, ".m2": true, ".cargo": true, ".rustup": true,
	".nvm": true, ".pyenv": true, ".rbenv": true, ".rvm": true, ".bun": true, ".deno": true, ".conda": true,
	"miniconda3": true, "anaconda3": true,
	// Serveurs d'IDE distants : retéléchargés automatiquement à la connexion suivante.
	".vscode-server": true, ".vscode-server-insiders": true, ".cursor-server": true, ".windsurf-server": true}

// Categorize classe un chemin. Les règles portent sur le nom (dossiers de cache,
// de dépendances…) et quelques chemins absolus ; sinon la catégorie du parent s'applique.
func Categorize(parentCat, p, name string, dir bool) string {
	if parentCat == CatSystem {
		return CatSystem
	}
	if dir && p != "" {
		for _, s := range systemRoots {
			if p == s {
				return CatSystem
			}
		}
		switch p {
		case "/tmp", "/var/tmp", "/var/cache":
			return CatCache
		case "/var/log":
			return CatLogs
		case "/var/lib/docker", "/var/lib/containerd":
			return CatDocker
		}
	}
	if dir {
		switch {
		case cacheNames[name]:
			return CatCache
		case depsNames[name]:
			return CatDeps
		case name == ".docker":
			return CatDocker
		case name == "logs" || name == "log":
			return CatLogs
		}
	} else if strings.HasSuffix(name, ".log") || strings.Contains(name, ".log.") {
		return CatLogs
	}
	if parentCat == "" {
		return CatData
	}
	return parentCat
}

var protectedExact = map[string]bool{"/": true, "/home": true, "/root": true, "/var": true, "/tmp": true,
	"/opt": true, "/srv": true, "/mnt": true, "/media": true, "/proc": true, "/sys": true, "/dev": true,
	"/run": true, "/var/lib": true, "/var/cache": true, "/var/log": true, "/var/tmp": true, "/lost+found": true}

// Protected dit si un chemin ne doit pas être supprimé depuis l'explorateur.
func Protected(p, cat string) bool {
	if cat == CatSystem || protectedExact[p] || strings.ContainsRune(p, '�') {
		return true
	}
	// /home/<utilisateur> lui-même
	if strings.HasPrefix(p, "/home/") && !strings.Contains(p[len("/home/"):], "/") {
		return true
	}
	for _, s := range systemRoots {
		if p == s || strings.HasPrefix(p, s+"/") {
			return true
		}
	}
	return false
}

// --- Vues pour l'interface ------------------------------------------------

// FsNode est la forme envoyée au front-end.
type FsNode struct {
	Name      string   `json:"name"`
	Path      string   `json:"path"`
	Size      int64    `json:"size"`
	Items     int64    `json:"items"`
	Kind      string   `json:"kind"`            // dir, file, small (petits fichiers regroupés), rest (éléments non détaillés)
	Count     int64    `json:"count,omitempty"` // nombre d'éléments regroupés (small, rest)
	Category  string   `json:"category"`
	Protected bool     `json:"protected"`
	ReadError bool     `json:"readError"`
	Children  []FsNode `json:"children,omitempty"`
}

// FsView décrit un dossier : ses enfants (sur deux niveaux pour la treemap)
// et la répartition de son contenu par catégorie.
type FsView struct {
	Node       FsNode           `json:"node"`
	Categories map[string]int64 `json:"categories"`
}

// Find renvoie le nœud d'un chemin absolu.
func (t *Tree) find(p string) *Node {
	if t.Root == nil {
		return nil
	}
	rootPath := t.Root.Name
	if p == rootPath {
		return t.Root
	}
	rel := strings.TrimPrefix(p, strings.TrimSuffix(rootPath, "/")+"/")
	if rel == p {
		return nil
	}
	n := t.Root
	for _, part := range strings.Split(rel, "/") {
		var next *Node
		for _, c := range n.Children {
			if c.Name == part {
				next = c
				break
			}
		}
		if next == nil {
			return nil
		}
		n = next
	}
	return n
}

// View construit la vue d'un dossier.
func (t *Tree) View(p string, limit, subLimit int) (FsView, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n := t.find(p)
	if n == nil {
		return FsView{}, errors.New(i18n.T("err.notInScan", p))
	}
	if !n.Dir {
		n = n.Parent
	}
	cats := map[string]int64{}
	sumCategories(n, cats)
	// Sous le premier niveau, on ne détaille que ce qui pèse au moins 1/3000 du
	// dossier affiché (une centaine de pixels dans la treemap) : le reste est regroupé.
	b := dtoBudget{limit: limit, subLimit: subLimit, minSize: n.Size / 3000}
	return FsView{Node: b.build(n, n.Path(), 0), Categories: cats}, nil
}

func sumCategories(n *Node, cats map[string]int64) {
	if !n.Dir {
		cats[n.Cat] += n.Size
		return
	}
	cats[n.Cat] += n.Own
	for _, c := range n.Children {
		sumCategories(c, cats)
	}
}

// toDTO convertit un nœud sans ses enfants.
func toDTO(n *Node, p string) FsNode {
	d := FsNode{Name: n.Name, Path: p, Size: n.Size, Items: n.Items, Kind: "file", Category: n.Cat,
		Protected: Protected(p, n.Cat), ReadError: n.ReadError}
	if n.Dir {
		d.Kind = "dir"
	}
	return d
}

type dtoBudget struct {
	limit, subLimit int
	minSize         int64
}

const maxViewDepth = 16

// build convertit un dossier et ses enfants : tous au niveau 0 (dans la limite),
// puis seulement les éléments assez gros pour apparaître dans la treemap.
func (b dtoBudget) build(n *Node, p string, level int) FsNode {
	d := toDTO(n, p)
	if !n.Dir || level >= maxViewDepth || (level > 0 && n.Size < b.minSize) {
		return d
	}
	limit := b.limit
	if level > 0 {
		limit = b.subLimit
	}
	kids := make([]*Node, len(n.Children))
	copy(kids, n.Children)
	sort.Slice(kids, func(i, j int) bool { return kids[i].Size > kids[j].Size })
	var rest, restItems, restCount int64
	for i, c := range kids {
		if i >= limit || (level > 0 && c.Size < b.minSize) {
			rest += c.Size
			restItems += c.Items
			restCount++
			continue
		}
		d.Children = append(d.Children, b.build(c, path.Join(p, c.Name), level+1))
	}
	// L'entrée du dossier lui-même et ses petits fichiers.
	if own := n.Own; n.SmallCount > 0 && own > 0 {
		d.Children = append(d.Children, FsNode{Count: n.SmallCount,
			Path: p + "/", Size: own, Items: n.SmallCount, Kind: "small", Category: n.Cat, Protected: true})
	}
	if rest > 0 {
		d.Children = append(d.Children, FsNode{Count: restCount,
			Path: p + "/…", Size: rest, Items: restItems, Kind: "rest", Category: n.Cat, Protected: true})
	}
	sort.SliceStable(d.Children, func(i, j int) bool { return d.Children[i].Size > d.Children[j].Size })
	return d
}

// TopFiles renvoie les plus gros fichiers (≥ SmallFileLimit) du scan.
func (t *Tree) TopFiles(limit int) []FsNode {
	t.mu.RLock()
	defer t.mu.RUnlock()
	h := &fileHeap{}
	var walk func(n *Node)
	walk = func(n *Node) {
		for _, c := range n.Children {
			if c.Dir {
				walk(c)
				continue
			}
			if h.Len() < limit {
				heap.Push(h, c)
			} else if c.Size > (*h)[0].Size {
				(*h)[0] = c
				heap.Fix(h, 0)
			}
		}
	}
	if t.Root != nil {
		walk(t.Root)
	}
	res := make([]FsNode, 0, h.Len())
	for h.Len() > 0 {
		n := heap.Pop(h).(*Node)
		res = append(res, toDTO(n, n.Path()))
	}
	for i, j := 0, len(res)-1; i < j; i, j = i+1, j-1 {
		res[i], res[j] = res[j], res[i]
	}
	return res
}

type fileHeap []*Node

func (h fileHeap) Len() int           { return len(h) }
func (h fileHeap) Less(i, j int) bool { return h[i].Size < h[j].Size }
func (h fileHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *fileHeap) Push(x any)        { *h = append(*h, x.(*Node)) }
func (h *fileHeap) Pop() any {
	old := *h
	n := old[len(old)-1]
	*h = old[:len(old)-1]
	return n
}

// Size renvoie la taille d'un chemin dans le scan (0 s'il n'y figure pas).
func (t *Tree) Size(p string) int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if n := t.find(p); n != nil {
		return n.Size
	}
	return 0
}

// Exists dit si un chemin figure dans le scan.
func (t *Tree) Exists(p string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.find(p) != nil
}

// CheckDeletable valide une sélection : chemins présents, non protégés, et sans
// doublon (un chemin contenu dans un autre chemin sélectionné est retiré).
func (t *Tree) CheckDeletable(paths []string) ([]string, int64, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	sorted := append([]string{}, paths...)
	sort.Strings(sorted)
	var keep []string
	var total int64
	for _, p := range sorted {
		if len(keep) > 0 {
			last := keep[len(keep)-1]
			if p == last || strings.HasPrefix(p, strings.TrimSuffix(last, "/")+"/") {
				continue
			}
		}
		n := t.find(p)
		if n == nil {
			return nil, 0, errors.New(i18n.T("err.notInScan", p))
		}
		if Protected(p, n.Cat) {
			return nil, 0, errors.New(i18n.T("err.protected", p))
		}
		keep = append(keep, p)
		total += n.Size
	}
	if len(keep) == 0 {
		return nil, 0, errors.New(i18n.T("err.nothingToDelete"))
	}
	return keep, total, nil
}

// Remove retire des chemins de l'arbre et met à jour les tailles des parents.
func (t *Tree) Remove(paths []string) int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	var freed int64
	for _, p := range paths {
		n := t.find(p)
		if n == nil || n.Parent == nil {
			continue
		}
		detach(n)
		freed += n.Size
	}
	return freed
}

func detach(n *Node) {
	par := n.Parent
	for i, c := range par.Children {
		if c == n {
			par.Children = append(par.Children[:i], par.Children[i+1:]...)
			break
		}
	}
	for a := par; a != nil; a = a.Parent {
		a.Size -= n.Size
		a.Items -= n.Items
	}
}

// Replace remplace le sous-arbre d'un chemin par un nouveau scan de ce chemin.
// Renvoie la variation de taille (négative si de la place a été libérée).
func (t *Tree) Replace(p string, sub *Node) int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	old := t.find(p)
	if old == nil || old.Parent == nil {
		return 0
	}
	par := old.Parent
	sub.Name = old.Name
	sub.Parent = par
	for i, c := range par.Children {
		if c == old {
			par.Children[i] = sub
			break
		}
	}
	dSize, dItems := sub.Size-old.Size, sub.Items-old.Items
	for a := par; a != nil; a = a.Parent {
		a.Size += dSize
		a.Items += dItems
	}
	return dSize
}

// CategoryOf renvoie la catégorie d'un chemin du scan.
func (t *Tree) CategoryOf(p string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if n := t.find(p); n != nil {
		return n.Cat
	}
	return CatData
}

// Homes liste les dossiers personnels présents dans le scan (/root, /home/*).
func (t *Tree) Homes() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var res []string
	if t.find("/root") != nil {
		res = append(res, "/root")
	}
	if h := t.find("/home"); h != nil {
		for _, c := range h.Children {
			if c.Dir {
				res = append(res, "/home/"+c.Name)
			}
		}
	}
	return res
}

// Stats renvoie la taille totale et le nombre d'entrées.
func (t *Tree) Stats() (int64, int64) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.Root == nil {
		return 0, 0
	}
	return t.Root.Size, t.Root.Items
}
