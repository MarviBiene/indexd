package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShouldCompactConsensusDB(t *testing.T) {
	tests := []struct {
		name        string
		size        int64
		reclaimable int64
		want        bool
	}{
		{
			name:        "too small",
			size:        consensusCompactMinSize - 1,
			reclaimable: consensusCompactMinReclaim * 2,
		},
		{
			name:        "too little reclaimable space",
			size:        consensusCompactMinSize * 2,
			reclaimable: consensusCompactMinReclaim - 1,
		},
		{
			name:        "ratio too small",
			size:        20 << 30,
			reclaimable: 1 << 30,
		},
		{
			name:        "compact",
			size:        20 << 30,
			reclaimable: 10 << 30,
			want:        true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldCompactConsensusDB(test.size, test.reclaimable); got != test.want {
				t.Fatalf("expected %v, got %v", test.want, got)
			}
		})
	}
}

func TestRecoverConsensusCompaction(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "consensus.db")
	backup := consensusCompactBackupPath(path)
	temp := consensusCompactTempPath(path)

	if err := os.WriteFile(backup, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(temp, []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := recoverConsensusCompaction(path); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	} else if string(data) != "original" {
		t.Fatalf("expected restored original database, got %q", data)
	}
	if _, err := os.Stat(temp); !os.IsNotExist(err) {
		t.Fatalf("expected stale compact file to be removed, got %v", err)
	}

	if err := os.WriteFile(backup, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := recoverConsensusCompaction(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		t.Fatalf("expected stale backup to be removed, got %v", err)
	}
}
