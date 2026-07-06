package document

import "testing"

func TestDeletePathAdjustsActivePathIdx(t *testing.T) {
	makePaths := func(n int) []NamedPath {
		paths := make([]NamedPath, 0, n)
		for range n {
			paths, _ = CreatePath(paths, "")
		}
		return paths
	}

	cases := []struct {
		name       string
		count      int
		active     int
		deleteIdx  int
		wantActive int
	}{
		{"delete below active shifts active down", 4, 2, 0, 1},
		{"delete above active keeps active", 3, 0, 2, 0},
		{"delete active keeps index (next path)", 3, 1, 1, 1},
		{"delete active at end clamps", 3, 2, 2, 1},
		{"delete last remaining path", 1, 0, 0, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			paths, active, err := DeletePath(makePaths(tc.count), tc.active, tc.deleteIdx)
			if err != nil {
				t.Fatalf("DeletePath: %v", err)
			}
			if len(paths) != tc.count-1 {
				t.Fatalf("len(paths) = %d, want %d", len(paths), tc.count-1)
			}
			if active != tc.wantActive {
				t.Errorf("activePathIdx = %d, want %d", active, tc.wantActive)
			}
		})
	}
}
