package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewClientBuildsPiecesGrid(t *testing.T) {
	torrent := &Torrent{
		NumOfPieces:    3,
		PieceSize:      32768,
		FinalPieceSize: 10000,
		Files:          []FileDict{{Length: 75536, Path: []string{"f.bin"}}},
		NumOfBlocks:    5,
	}

	c := NewClient(torrent)
	t.Cleanup(c.cancel)

	if len(c.piecesGrid) != 3 {
		t.Fatalf("expected 3 pieces, got %d", len(c.piecesGrid))
	}

	for i, ps := range c.piecesGrid {
		wantSize := torrent.PieceSize
		if i == len(c.piecesGrid)-1 {
			wantSize = torrent.FinalPieceSize
		}
		if uint64(len(ps.Bytes)) != wantSize {
			t.Fatalf("piece %d: expected buffer size %d, got %d", i, wantSize, len(ps.Bytes))
		}
		if ps.PieceSize != wantSize {
			t.Fatalf("piece %d: expected PieceSize %d, got %d", i, wantSize, ps.PieceSize)
		}
		wantBlocks := int(wantSize / uint64(blockSize))
		if wantSize%uint64(blockSize) != 0 {
			wantBlocks++
		}
		if ps.TotalBlocks != wantBlocks {
			t.Fatalf("piece %d: expected %d total blocks, got %d", i, wantBlocks, ps.TotalBlocks)
		}
		if ps.Done {
			t.Fatalf("piece %d: expected new piece to not be marked done", i)
		}
	}

	if len(c.filesGrid) != len(torrent.Files) {
		t.Fatalf("expected filesGrid sized to %d files, got %d", len(torrent.Files), len(c.filesGrid))
	}
	if cap(c.queue) != int(torrent.NumOfBlocks) {
		t.Fatalf("expected queue capacity %d, got %d", torrent.NumOfBlocks, cap(c.queue))
	}
}

func TestCreateFileFromPathNested(t *testing.T) {
	root := t.TempDir()

	f, err := createFileFromPath(root, []string{"sub", "dir", "file.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer f.Close()

	wantPath := filepath.Join(root, "sub", "dir", "file.txt")
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected file to exist at %s: %v", wantPath, err)
	}
}

func TestCreateFileFromPathSingleFileNoSubdirs(t *testing.T) {
	root := t.TempDir()

	f, err := createFileFromPath(root, []string{"file.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer f.Close()

	wantPath := filepath.Join(root, "file.txt")
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected file to exist at %s: %v", wantPath, err)
	}
}

func TestCreateFileFromPathIdempotentDirs(t *testing.T) {
	root := t.TempDir()

	f1, err := createFileFromPath(root, []string{"sub", "a.txt"})
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}
	f1.Close()

	// Same subdirectory reused for a second file -- must not error on
	// "directory already exists".
	f2, err := createFileFromPath(root, []string{"sub", "b.txt"})
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	f2.Close()

	if _, err := os.Stat(filepath.Join(root, "sub", "a.txt")); err != nil {
		t.Fatalf("expected first file to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "sub", "b.txt")); err != nil {
		t.Fatalf("expected second file to exist: %v", err)
	}
}
