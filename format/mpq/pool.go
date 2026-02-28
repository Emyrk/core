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

	sortPatchArchives(names)
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

func sortPatchArchives(names []string) {
	sort.SliceStable(names, func(i, j int) bool {
		ai := archiveOrderKey(getArchiveName(names[i]))
		aj := archiveOrderKey(getArchiveName(names[j]))
		// category first: base archives, then patch stream
		if ai.cat != aj.cat {
			return ai.cat < aj.cat
		}
		// patch.MPQ first in patch stream
		if ai.cat == 1 && ai.kind != aj.kind {
			return ai.kind < aj.kind
		}
		// numeric patches ascending: patch-2, patch-3, ...
		if ai.cat == 1 && ai.kind == 1 && ai.num != aj.num {
			return ai.num < aj.num
		}
		// letter patches ascending: patch-A ... patch-Z
		if ai.cat == 1 && ai.kind == 2 && ai.letter != aj.letter {
			return ai.letter < aj.letter
		}
		// deterministic fallback for same class
		return ai.norm < aj.norm
	})
}

type orderKey struct {
	cat    int // 0=base/non-patch, 1=patch-family
	kind   int // for patch-family: 0=patch, 1=patch-N, 2=patch-X, 3=other patch*
	num    int
	letter rune
	norm   string
}

func archiveOrderKey(name string) orderKey {
	norm := strings.ToLower(name)
	stem := strings.TrimSuffix(norm, filepath.Ext(norm))
	k := orderKey{cat: 0, kind: 3, norm: norm}
	if stem == "patch" {
		k.cat, k.kind = 1, 0
		return k
	}
	if strings.HasPrefix(stem, "patch-") {
		k.cat = 1
		s := strings.TrimPrefix(stem, "patch-")
		if v, err := strconv.Atoi(s); err == nil {
			k.kind, k.num = 1, v
			return k
		}
		if len(s) == 1 && s[0] >= 'a' && s[0] <= 'z' {
			k.kind, k.letter = 2, rune(s[0])
			return k
		}
		// unknown patch-* still in patch family, sorted by norm fallback
	}
	return k
}
