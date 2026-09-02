# Ziftorrent

A BitTorrent client implemented in Go from the wire protocol up: raw TCP peer
connections, a hand-rolled UDP tracker client, and a concurrent piece-download
engine. No third-party torrent library is used for any protocol logic — the only
dependency is `bencode-go`, used strictly for parsing `.torrent` file metadata.

## Motivation

Most "build your own torrent client" projects wrap an existing library and call
it a day. Ziftorrent implements the [BitTorrent protocol spec](https://www.bittorrent.org/beps/bep_0003.html)
directly — the peer wire protocol, the UDP tracker handshake, piece selection,
block-level request scheduling, adaptive timeout/retry logic, and disk I/O across
multi-file torrents are all written from scratch, as a way to work with Go's
concurrency primitives (goroutines, channels, `context.Context`) on a real,
non-trivial network protocol rather than a toy problem.

A few things worth knowing before diving in:

- **Concurrent-by-design download engine**: one reader + one worker goroutine per
  peer, a shared job queue, and an O(1) request-matching map, coordinated end-to-end
  with `context.Context` for clean shutdown (including forcing blocked reads to
  unblock on cancellation).
- **Adaptive networking**: per-peer retry timeouts derived from a live, windowed
  RTT average (`avg * 4`, clamped to `[2s, 20s]`) instead of one fixed timeout for
  every peer — slow-but-healthy peers aren't punished the same as actually-dead
  ones.
- **Correct multi-file handling**: blocks are streamed to disk as they arrive and
  transparently split across file boundaries when a block straddles two files in
  the torrent's logical byte stream.

## Quick Start

Requires Go installed locally (see [go.dev/dl](https://go.dev/dl/)).

```sh
git clone https://github.com/<your-username>/ziftorrent.git
cd ziftorrent
go run . -f path/to/file.torrent
```

That's it — the client parses the torrent, announces to its tracker, connects to
peers, and starts downloading. Downloaded files are written under a directory
named after the torrent (`Torrent.Name`), mirroring the file paths declared in
the torrent's info dictionary.

## Usage

```sh
go run . -f path/to/file.torrent
```

Flags:

| Flag | Description |
|---|---|
| `-f` | Path to the `.torrent` file (required) |
| `-t` | Override tracker URL instead of using the torrent's own announce list |

The client exits once every byte of the torrent's content has been written and
verified against the torrent's SHA-1 piece hashes.

## Contributing

Contributions and issues are welcome. This section is a map of the codebase and
the open work, for anyone looking to dig in.

### Project layout

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

### Architecture notes

**Concurrency model.** Each peer gets its own goroutines: `readPeerMessages`
blocks on `peer.Read`, parses the length-prefixed wire protocol, and dispatches
messages (`choke`, `unchoke`, `have`, `bitfield`, `piece`) to handlers.
`runPeerWorker` pulls jobs off a single shared, buffered channel (`Client.queue`)
and sends the corresponding `request` message to that peer, provided it isn't
choking and actually has the piece. There is no polling loop in the request
path: a block request arms a `time.AfterFunc` timer sized to the peer's current
measured RTT, and if it fires before the block arrives, the job is pushed back
onto the queue for another peer. In-flight requests are tracked in a single map
(`Client.pending`, keyed by `"pieceIndex:begin"`) for O(1) matching. Shutdown is
coordinated with `context.Context`: cancelling `Client.ctx` stops every peer
worker and, via a small per-peer watcher goroutine, forces any blocked
`peer.Read()` to return by closing the connection.

**Piece and file layout.** `PieceState` tracks a piece's in-memory buffer, which
blocks have been received (`Received []bool`, to reject duplicate deliveries
without double-counting), and whether the piece is complete. Blocks are written
to disk as soon as they arrive rather than batched per-piece; a completed
piece's hash is verified against the torrent's SHA-1 piece hash list, and its
in-memory buffer is released. For multi-file torrents, `FileState` entries
describe each file's offset in the logical concatenated byte stream, and
`writeAtGlobal` translates a global piece/block offset into one or more
`WriteAt` calls, splitting a single block's write across a file boundary if it
straddles two files.

### Running tests

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

### Good areas to contribute

The core download path — parsing, tracker communication, peer wire protocol,
concurrent piece retrieval, and multi-file disk writes — is complete and tested.
Open items if you want to send a PR:

- **HTTP tracker + DHT support**, alongside the existing UDP tracker client.
- **Resume support** — persist completed-piece state to disk so interrupted
  downloads don't restart from zero.
- **Seeding** — upload to peers instead of leech-only behavior.
- **Smarter piece/peer selection** — rarest-first piece picking, tit-for-tat
  choking, and endgame-mode redundant requests.
- **Surface `BencodeTorrent.ToTorrent` errors** instead of discarding them, so a
  malformed `.torrent` file fails loudly rather than loading as an empty `Torrent`.
