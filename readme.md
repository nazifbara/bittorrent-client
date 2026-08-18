# Ziftorrent

A BitTorrent client written in Go from scratch on top of raw TCP peer connections and
a UDP tracker client — no third-party torrent library is used for the protocol itself
(only `bencode-go` for parsing `.torrent` files).

It's a learning project, built to understand the BitTorrent protocol and Go's
concurrency primitives by implementing them directly rather than wrapping an existing
library. The long-term goal is to grow it into something production-ready; see
[Limitations](#limitations) for what that still requires.

## What it does

Given a `.torrent` file, the client:

1. Parses the bencoded metadata (`torrent.go`) and computes the info hash, piece
   layout, and per-file byte ranges.
2. Announces to a UDP tracker to get a peer list (`tracker.go`).
3. Connects to peers over TCP and performs the BitTorrent handshake (`handshake.go`).
4. Requests, receives, and validates piece data concurrently across all connected
   peers, writing completed blocks straight to disk (`downloads.go`).
5. Exits once every byte of the torrent's content has been written and verified.

## Architecture

### Concurrency model

Each peer gets its own goroutines:

- **`readPeerMessages`** — blocks on `peer.Read`, parses the length-prefixed wire
  protocol, and dispatches messages (`choke`, `unchoke`, `have`, `bitfield`, `piece`)
  to handlers.
- **`runPeerWorker`** — pulls jobs (piece/block requests) off a single shared,
  buffered channel (`Client.queue`) and sends the corresponding `request` message to
  that peer, provided the peer isn't choking and actually has the piece.

There is no polling loop in the request path. Instead:

- A block request arms a `time.AfterFunc` timer sized to the current measured RTT
  (see below). If the timer fires before the block arrives, the job is pushed back
  onto the queue for another peer to pick up.
- If the block arrives first, the timer is cancelled and the round-trip time is fed
  into that peer's rolling RTT average.
- In-flight requests are tracked in a single map (`Client.pending`, keyed by
  `"pieceIndex:begin"`) rather than scanned linearly, so matching an incoming block
  to its request is O(1).

Shutdown is coordinated with `context.Context`: cancelling `Client.ctx` stops every
peer worker and — via a small per-peer watcher goroutine — forces any blocked
`peer.Read()` to return by closing the connection, so nothing is left hanging on I/O
that a context alone can't interrupt.

### Adaptive request timeout

`rtt.go` maintains a capped, windowed average of observed round-trip times
(`rttTracker`, one per peer). The re-request timeout for a given peer is derived
from its own recent average RTT (`avg * 4`, clamped to `[2s, 20s]`) rather than a
single fixed constant for every peer — a peer with a slow but real 800ms RTT won't
be timed out and re-requested as aggressively as one with a healthy 50ms RTT.

### Piece and file layout

- `PieceState` tracks a piece's in-memory buffer, which blocks have been received
  (`Received []bool`, used to reject duplicate deliveries without double-counting),
  and whether the piece is complete.
- Blocks are written to disk as soon as they arrive (not batched per-piece); a
  completed piece's hash is verified against the torrent's SHA-1 piece hash list, and
  its in-memory buffer is released.
- For multi-file torrents, `FileState` entries describe each file's offset in the
  logical concatenated byte stream. `writeAtGlobal` translates a global piece/block
  offset into one or more `WriteAt` calls, splitting a single block's write across a
  file boundary if the block happens to straddle two files.

## Project layout

| File | Responsibility |
|---|---|
| `main.go` | CLI entry point, signal handling, top-level shutdown |
| `client.go` | `Client` struct, piece/file grid initialization |
| `torrent.go` | `.torrent` (bencode) parsing, info hash, piece hashes |
| `tracker.go` | UDP tracker connect/announce, peer address parsing |
| `handshake.go` | BitTorrent handshake exchange |
| `peers.go` | Peer connection lifecycle, main download loop |
| `downloads.go` | Peer message handling, request/response flow, disk writes |
| `queue.go` | Job queue, enqueue/requeue, timeout handling |
| `rtt.go` | Per-peer adaptive RTT tracking |
| `messages.go` | Wire protocol message encode/decode |
| `utils.go` | Bit packing, peer ID generation, retry helper |

## Usage

```sh
go run . -f path/to/file.torrent
```

Flags:

| Flag | Description |
|---|---|
| `-f` | Path to the `.torrent` file (required) |
| `-t` | Override tracker URL instead of using the torrent's own announce list |

Downloaded files are written under a directory named after the torrent (`Torrent.Name`),
mirroring the file paths declared in the torrent's info dictionary.

## Tests

```sh
go test ./...
```

| Test file | Covers |
|---|---|
| `messages_test.go` | Wire protocol message building/parsing |
| `utils_test.go` | Bit packing, peer ID generation, the retry helper |
| `rtt_test.go` | RTT averaging, windowing, timeout clamping |
| `torrent_test.go` | Bencode parsing, piece hashing, single/multi-file layout |
| `tracker_test.go` | UDP tracker request/response wire format |
| `queue_test.go` | Job enqueue/requeue, timeout handling, shutdown |
| `client_test.go` | Client/piece-grid construction, per-file setup |
| `blocksize_test.go` | Last-block sizing for regular and final pieces |
| `downloads_test.go` | Disk writes, including multi-file boundary splitting |
| `peers_test.go` | Peer lookup by address |

## Limitations

Known gaps between this and a client you'd trust in production:

- **UDP trackers only** — no HTTP tracker or DHT support.
- **No resume support** — restarting re-downloads from scratch; there's no on-disk
  record of completed pieces across runs.
- **No seeding** — the client only leeches, it never uploads to peers.
- **`openTorrent` swallows a real error** — `BencodeTorrent.ToTorrent`'s error is
  currently discarded, so a malformed `.torrent` file loads as an empty `Torrent`
  instead of failing loudly.
- **Simple peer/piece selection** — no rarest-first piece picking, tit-for-tat
  choking, or endgame-mode redundant requests.