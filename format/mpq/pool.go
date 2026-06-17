package mpq

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Pool pre-loads multiple MPQ archive headers, allowing fast, concurrent access of archive files
type Pool struct {
	// Archive data
	fmap map[string]*archiveEntry
	list []string
}

type archiveEntry struct {
	name       string
	header     *Header
	hashTable  []*HashEntry
	blockTable []*BlockEntry
}

func getArchiveName(name string) string {
	s := strings.Split(name, "/")
	return s[len(s)-1]
}

func (p *Pool) addArchive(name string) error {
	m, err := Open(name)
	if err != nil {
		return err
	}

	fmt.Println("[MPQ Pool] Opened", name)

	ae := new(archiveEntry)
	ae.name = name
	ae.header = m.Header
	ae.hashTable = m.HashTable
	ae.blockTable = m.BlockTable

	lf := m.ListFiles()

	for _, fv := range lf {
		p.fmap[fv] = ae
		//mappedFile := p.fmap[fv]
		//if mappedFile == nil {
		//	p.fmap[fv] = ae // map filepath string to MPQ data pointer
		//}
	}

	return nil
}

// OpenPool opens a Pool using a slice of MPQ file paths
func OpenPool(names []string) (*Pool, error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("mpq: cannot open Pool without at least one archive")
	}

	p := &Pool{}
	p.fmap = make(map[string]*archiveEntry)

	// Add patch archives first.
	//var patch, todo []string
	//for _, v := range names {
	//	if strings.Contains(getArchiveName(v), "patch") {
	//		patch = append(patch, v)
	//	} else {
	//		todo = append(todo, v)
	//	}
	//}

	SortPatchArchives(names)
	for _, v := range names {
		err := p.addArchive(v)
		if err != nil {
			return nil, err
		}
	}

	//// Add other archives later.
	//for _, v := range todo {
	//	err := p.addArchive(v)
	//	if err != nil {
	//		return nil, err
	//	}
	//}

	return p, nil
}

func (p *Pool) OpenFile(name string) (*File, error) {
	ae := p.fmap[name]
	if ae == nil {
		return nil, fmt.Errorf("File not found")
	}

	fmt.Printf("[MPQ Pool] OpenFile %q -> %s\n", name, ae.name) // <- add here
	m := new(MPQ)
	m.Path = ae.name
	var err error
	m.File, err = os.Open(ae.name)
	if err != nil {
		return nil, err
	}

	m.Header = ae.header
	m.BlockTable = ae.blockTable
	m.HashTable = ae.hashTable

	file, err := m.OpenFile(name)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (p *Pool) ListFiles() []string {
	str := make([]string, len(p.fmap))
	i := 0
	for k := range p.fmap {
		str[i] = k
		i++
	}

	sort.Strings(str)

	return str
}

// SortPatchArchives sorts MPQ archive paths so that archives are loaded in
// ascending priority order (lowest priority first). Because the Pool uses
// last-write-wins for its file map, the last archive loaded takes precedence.
//
// WoW 3.3.5a load order (ascending priority):
//
//  1. Data/ base archives  (common, expansion, lichking, …)
//  2. Data/{locale}/ base archives  (locale-enUS, base-enUS, …)
//  3. Data/ patch.MPQ
//  4. Data/ numbered patches  (patch-2, patch-3)
//  5. Data/{locale}/ locale patches  (patch-enUS, patch-enUS-2, patch-enUS-3)
//  6. Data/ letter patches  (Patch-F, Patch-G, …, Patch-X)  ← highest
//
// Within the same kind, archives in a locale subdirectory (deeper path) load
// after archives in Data/ so they take precedence.
func SortPatchArchives(names []string) {
	sort.SliceStable(names, func(i, j int) bool {
		ai := archiveOrderKey(names[i])
		aj := archiveOrderKey(names[j])
		// category first: base archives, then patch stream
		if ai.cat != aj.cat {
			return ai.cat < aj.cat
		}
		// within same category, sort by kind
		if ai.kind != aj.kind {
			return ai.kind < aj.kind
		}
		// within same kind, sub-sort
		if (ai.kind == 1 || ai.kind == 2) && ai.num != aj.num {
			return ai.num < aj.num
		}
		if ai.kind == 3 && ai.letter != aj.letter {
			return ai.letter < aj.letter
		}
		// locale subdirectory loads after Data/ (higher priority)
		if ai.depth != aj.depth {
			return ai.depth < aj.depth
		}
		// deterministic fallback for same class
		return ai.norm < aj.norm
	})
}

type orderKey struct {
	cat    int    // 0=base/non-patch, 1=patch-family
	kind   int    // 0=patch, 1=patch-N, 2=patch-locale (multi-char), 3=patch-X (single letter)
	num    int    // for kind==1: numeric suffix
	letter rune   // for kind==3: letter suffix
	depth  int    // path component count (deeper = locale subdir = higher priority)
	norm   string // lowercase filename for deterministic fallback
}

func archiveOrderKey(fullPath string) orderKey {
	name := getArchiveName(fullPath)
	norm := strings.ToLower(name)
	stem := strings.TrimSuffix(norm, filepath.Ext(norm))
	depth := strings.Count(fullPath, "/") + strings.Count(fullPath, "\\")
	k := orderKey{cat: 0, kind: 0, depth: depth, norm: norm}

	if stem == "patch" {
		k.cat, k.kind = 1, 0
		return k
	}
	if strings.HasPrefix(stem, "patch-") {
		k.cat = 1
		s := strings.TrimPrefix(stem, "patch-")
		if v, err := strconv.Atoi(s); err == nil {
			// patch-2, patch-3
			k.kind, k.num = 1, v
			return k
		}
		if len(s) == 1 && s[0] >= 'a' && s[0] <= 'z' {
			// Single-letter custom server patches (Patch-F, Patch-Q, …).
			// These have the HIGHEST priority.
			k.kind, k.letter = 3, rune(s[0])
			return k
		}
		// Multi-char patch variants: locale patches (patch-enUS, patch-enUS-2)
		// and other custom names. These sit between numbered and letter patches.
		// Extract a trailing number for sub-sorting (e.g. patch-enUS-2 → num=2).
		k.kind = 2
		if idx := strings.LastIndex(s, "-"); idx >= 0 {
			if v, err := strconv.Atoi(s[idx+1:]); err == nil {
				k.num = v
			}
		}
	}
	return k
}
