package mpq

import (
	"strings"
	"testing"
)

func TestSortPatchArchives(t *testing.T) {
	// Simulates a real Faebright/Turtle WoW Data directory.
	// Input is deliberately shuffled (as filepath.WalkDir might return).
	input := []string{
		"/game/Data/enUS/patch-enUS.MPQ",
		"/game/Data/Patch-X.MPQ",
		"/game/Data/patch-3.MPQ",
		"/game/Data/enUS/locale-enUS.MPQ",
		"/game/Data/common.MPQ",
		"/game/Data/enUS/expansion-locale-enUS.MPQ",
		"/game/Data/Patch-F.MPQ",
		"/game/Data/patch.MPQ",
		"/game/Data/expansion.MPQ",
		"/game/Data/enUS/patch-enUS-2.MPQ",
		"/game/Data/patch-Q.mpq",
		"/game/Data/enUS/base-enUS.MPQ",
		"/game/Data/patch-2.MPQ",
		"/game/Data/lichking.MPQ",
		"/game/Data/Patch-S.MPQ",
		"/game/Data/common-2.MPQ",
		"/game/Data/enUS/patch-enUS-3.MPQ",
		"/game/Data/Patch-G.MPQ",
		"/game/Data/enUS/lichking-locale-enUS.MPQ",
		"/game/Data/enUS/speech-enUS.MPQ",
		"/game/Data/enUS/backup-enUS.MPQ",
		"/game/Data/Patch-H.MPQ",
		"/game/Data/enUS/expansion-speech-enUS.MPQ",
		"/game/Data/enUS/lichking-speech-enUS.MPQ",
	}

	SortPatchArchives(input)

	// Build a simplified view for assertion.
	var got []string
	for _, p := range input {
		// Strip common prefix for readability.
		got = append(got, strings.TrimPrefix(p, "/game/"))
	}

	// Expected order (ascending priority — last entry wins in Pool):
	//
	// 1. Data/ base archives (cat=0, depth=2)
	// 2. Data/enUS/ base archives (cat=0, depth=3)
	// 3. Data/ patch.MPQ (cat=1, kind=0)
	// 4. Data/ numbered patches (cat=1, kind=1)
	// 5. Data/enUS/ locale patches (cat=1, kind=2)
	// 6. Data/ letter patches (cat=1, kind=3) — HIGHEST priority
	want := []string{
		// --- cat=0 base archives, depth=2 (Data/) ---
		"Data/common-2.MPQ",
		"Data/common.MPQ",
		"Data/expansion.MPQ",
		"Data/lichking.MPQ",
		// --- cat=0 base archives, depth=3 (Data/enUS/) ---
		"Data/enUS/backup-enUS.MPQ",
		"Data/enUS/base-enUS.MPQ",
		"Data/enUS/expansion-locale-enUS.MPQ",
		"Data/enUS/expansion-speech-enUS.MPQ",
		"Data/enUS/lichking-locale-enUS.MPQ",
		"Data/enUS/lichking-speech-enUS.MPQ",
		"Data/enUS/locale-enUS.MPQ",
		"Data/enUS/speech-enUS.MPQ",
		// --- cat=1, kind=0: patch.MPQ ---
		"Data/patch.MPQ",
		// --- cat=1, kind=1: numbered patches ---
		"Data/patch-2.MPQ",
		"Data/patch-3.MPQ",
		// --- cat=1, kind=2: locale patches (num=0 first, then num=2, num=3) ---
		"Data/enUS/patch-enUS.MPQ",
		"Data/enUS/patch-enUS-2.MPQ",
		"Data/enUS/patch-enUS-3.MPQ",
		// --- cat=1, kind=3: letter patches (highest priority) ---
		"Data/Patch-F.MPQ",
		"Data/Patch-G.MPQ",
		"Data/Patch-H.MPQ",
		"Data/patch-Q.mpq",
		"Data/Patch-S.MPQ",
		"Data/Patch-X.MPQ",
	}

	if len(got) != len(want) {
		t.Fatalf("length mismatch: got %d, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d:\n  got  %s\n  want %s", i, got[i], want[i])
		}
	}

	if t.Failed() {
		t.Log("Full sorted order:")
		for i, v := range got {
			t.Logf("  [%2d] %s", i, v)
		}
	}
}

func TestArchiveOrderKey(t *testing.T) {
	tests := []struct {
		path     string
		wantCat  int
		wantKind int
	}{
		{"Data/common.MPQ", 0, 0},
		{"Data/expansion.MPQ", 0, 0},
		{"Data/enUS/locale-enUS.MPQ", 0, 0},
		{"Data/patch.MPQ", 1, 0},
		{"Data/patch-2.MPQ", 1, 1},
		{"Data/patch-3.MPQ", 1, 1},
		{"Data/enUS/patch-enUS.MPQ", 1, 2},
		{"Data/enUS/patch-enUS-2.MPQ", 1, 2},
		{"Data/Patch-F.MPQ", 1, 3},
		{"Data/patch-Q.mpq", 1, 3},
		{"Data/Patch-X.MPQ", 1, 3},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			k := archiveOrderKey(tt.path)
			if k.cat != tt.wantCat {
				t.Errorf("cat: got %d, want %d", k.cat, tt.wantCat)
			}
			if k.kind != tt.wantKind {
				t.Errorf("kind: got %d, want %d", k.kind, tt.wantKind)
			}
		})
	}
}
