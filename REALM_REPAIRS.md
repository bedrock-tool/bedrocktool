# Realm recording repair review

## Scope and evidence

This review bundle repairs typed Realm NetherNet routing, protocol codecs,
connection cancellation/cleanup, fatal diagnostics, raw journaling, biome
registration and exact block-state export. It also protects recording from
optional map-rendering failures and initializes missing block-entity NBT maps.

A prior live recording exported **4,341 chunks and 832 entities** (recorder-reported
counts). The user confirmed flying the recorded route in a separate imported
world. This establishes only the visited capture, not complete Realm coverage,
perfect entity reproduction, custom-addon compatibility or uncaptured terrain.
The later nil-map block-entity merge fix is **unit/DB-round-trip tested only**;
it has not been established by a later live recording.
No account cache, private world, raw capture or live log is included.

## Reproducible build

Use Go **1.26.8**. The root go.mod and go.work both request that version.
A patched Windows toolchain is required for overlapped UDP safety (golang/go#77975).
The workspace uses the local dragonfly, go-raknet and gophertunnel modules;
root-only checkout/build without those exact dependencies is not equivalent.

After dependency publication and parent gitlink updates, a fresh checkout must use:

```sh
git clone --recurse-submodules <published-parent-fork-url>
cd bedrocktool
git checkout fix/realm-capture-integrity
git submodule update --init --recursive
go version # go1.26.8

go test ./handlers ./handlers/worlds ./handlers/worlds/worldstate ./utils/connectinfo ./utils/proxy ./utils/proxy/resourcepacks ./utils/behaviourpack -count=1
go test ./utils -run 'Test.*(Panic|Fatal|Redact)' -count=1
(cd gophertunnel && go test ./minecraft ./minecraft/protocol ./minecraft/protocol/packet -count=1)
(cd dragonfly && go test ./server/world/chunk -count=1)
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build \
  -ldflags '-X github.com/bedrock-tool/bedrocktool/utils.Version=realm-community-review -X github.com/bedrock-tool/bedrocktool/utils.CmdName=bedrocktool' \
  -o /tmp/bedrocktool-review.exe ./cmd/bedrocktool
```

This uncommitted staging snapshot is **not yet remotely reproducible**. Dependency
fork commits must be reviewed, committed and published first, then root gitlinks
and gophertunnel/dragonfly .gitmodules URLs updated to fetchable fork locations.
Do not publish a parent-only commit referencing the unchanged dependency bases.

## Palette provenance and license

`handlers/worlds/block_states_1_26_50.nbt` is byte-identical to the public asset:

https://raw.githubusercontent.com/df-mc/dragonfly/4c7b5074be94fa83a1cd98e9c752083ad04a6e21/server/world/block_states.nbt

- Upstream commit: `4c7b5074be94fa83a1cd98e9c752083ad04a6e21`.
- Asset length: 3,102,889 bytes.
- SHA-256: `f0784a6284d6ca7d98cc3472f4ce84241a11e11b18ed16f6591dfd5e6da6fbd6`.
- MIT License, Copyright (c) 2019 Dragonfly Tech; full notice retained in
  `handlers/worlds/block_states_1_26_50.LICENSE`.
- `TestVanillaPaletteProvenance` checks the embedded asset checksum offline.

## Capture confidentiality and supported palettes

Both raw and normal `.pcap2` captures retain pre-login authentication/login
traffic and other private player/world data. They are sensitive local artifacts:
**never attach them to public issues, PRs, release assets or public shares**.
Keep replay data private; provide only synthetic fixtures and redacted diagnostics.
Raw journals still preserve original packet bytes for offline replay; resource
packs are retained by the normal capture after a successful join.

Capture files are created exclusively (never overwrite an existing journal).
On POSIX-permission filesystems the file mode is `0600`; the capture directory
is created/tightened to `0700`. On Windows the implementation instead creates
files with an explicit protected DACL granting full access only to the current
process user, and creates/tightens the capture directory with that same DACL.
Windows `chmod` alone is **not** an ACL security boundary. ACL failures fail
capture setup rather than falling back to a permissive file. Filesystems that
ignore POSIX modes (for example some mounted Windows volumes under WSL) need
separate native ACL protection; do not assume the numeric modes secure them.
Privileged administrators and backups can still access these artifacts. This
change does not rewrite ACLs or modes on pre-existing capture files, including
files already in the capture directory. Secure or remove older captures
separately; on Windows, directory ACLs alone do not revoke access to those files.

Only **hashed** block network IDs are supported for the current 1.26.50 palette.
Non-hashed StartGame sessions (including retries) fail closed and mark the world
unexportable: the older local palette order does not match the current wire
palette. No successful `.mcworld` is produced using that unsupported mapping.
Supporting non-hashed IDs requires the exact current wire palette ordering and
a wire-to-disk state regression; it is not fixed by adding missing state names.
Hashed unsigned IDs continue to be checked through decode and DB reopen tests.
Chunk storage errors from LevelChunk and SubChunk processing are propagated,
recorded in world integrity state, and prevent `.mcworld` creation.

## Verification boundaries

The targeted suites and Windows cross-build are distinct from the full suite.
Confirmed baseline full-suite blockers include `TestInvalidJsonFix`, embedded
NUL bytes in `subcommands/resourcepack-d/resourcepack-d.go`, and missing GUI
pkg-config libraries. The gophertunnel full suite also has the baseline
`minecraft/resource` `TestVersion` failure for `1.0.0-test`.
Do not claim that `go test ./...` passes. A cross-build is not a native Windows
storage-stress run, and synthetic temporary DB tests are not another live capture.
