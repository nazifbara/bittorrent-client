package main

import "testing"

func TestBuildBlockSizeRegularPiece(t *testing.T) {
	c := &Client{torrent: &Torrent{numOfPieces: 5, pieceSize: 32768, finalPieceSize: 20000}}

	got := c.buildBlockSize(0, 0)
	if got != blockSize {
		t.Fatalf("expected full block size %d, got %d", blockSize, got)
	}

	// last block of a regular piece: only the remainder should be returned
	got = c.buildBlockSize(0, 32768-100)
	if got != 100 {
		t.Fatalf("expected remainder 100, got %d", got)
	}
}

func TestBuildBlockSizeFinalPiece(t *testing.T) {
	c := &Client{torrent: &Torrent{numOfPieces: 5, pieceSize: 32768, finalPieceSize: 20000}}

	lastIdx := uint32(4) // NumOfPieces - 1
	got := c.buildBlockSize(lastIdx, 0)
	if got != blockSize {
		t.Fatalf("expected full block size %d for the start of the final piece, got %d", blockSize, got)
	}

	got = c.buildBlockSize(lastIdx, 20000-500)
	if got != 500 {
		t.Fatalf("expected remainder 500 on the final piece, got %d", got)
	}
}

func TestBuildBlockSizeFinalPieceSmallerThanOneBlock(t *testing.T) {
	// When the final piece itself is smaller than blockSize, even the first
	// request for it should return the whole (short) remainder, not a full block.
	c := &Client{torrent: &Torrent{numOfPieces: 5, pieceSize: 32768, finalPieceSize: 5000}}

	lastIdx := uint32(4)
	got := c.buildBlockSize(lastIdx, 0)
	if got != 5000 {
		t.Fatalf("expected the whole short final piece (5000), got %d", got)
	}
}
