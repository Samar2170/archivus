# archivus-sync

A standalone desktop client for an [archivus](../) server. It uploads files and
directories, and keeps local folders continuously synced up into a drive —
re-uploading only files whose **content has changed** since the last run.

It is a separate Go module with **no external dependencies** (standard library
only, no CGO), so it builds and installs cleanly on any client machine
independent of the server.

## How it works

The client talks to the server over the same HTTP API the web frontend uses:

- `POST /auth/login` → bearer token
- `POST /storage/file/upload` → multipart upload (`driveId`, `folderPath`, `files`)

Config and sync state live under `~/.archivus-sync` (override with
`ARCHIVUS_SYNC_HOME`):

- `config.json` — server URL, bearer token, tracked folders
- `state.json` — per-file `{size, modTime, sha256}` used for change detection

**Change detection.** On each run the client stats every file. If size and
mtime match the recorded state it's skipped without hashing (fast path).
Otherwise it computes a SHA-256 and compares it to the stored checksum; the file
is uploaded only if the hash actually differs. A `touch` that doesn't change
content therefore uploads nothing.

## Install

```sh
cd sync-client
go build -o archivus-sync ./cmd/archivus-sync
# then move it onto your PATH, e.g.
install -m 0755 archivus-sync ~/.local/bin/
```

## Usage

```sh
# 1. Authenticate (prompts for password + PIN; or set ARCHIVUS_SYNC_PASSWORD / ARCHIVUS_SYNC_PIN)
archivus-sync login -server http://localhost:8080 -username alice

# 2a. One-shot upload of a file or a whole directory
archivus-sync upload ~/Documents/report.pdf -drive <DRIVE_ID> -dest reports
archivus-sync upload ~/Pictures            -drive <DRIVE_ID> -dest photos

# 2b. Or register folders to keep synced
archivus-sync track add ~/Pictures  -drive <DRIVE_ID> -dest photos
archivus-sync track add ~/Documents -drive <DRIVE_ID> -dest docs
archivus-sync track list
archivus-sync track remove ~/Pictures

# 3. Sync all tracked folders once (this is what you schedule)
archivus-sync sync
```

`-dest` is a folder path within the drive (forward slashes; omit for the drive
root). Subdirectories within an uploaded/tracked directory are preserved under
`-dest`. `upload` accepts `-force` to re-upload even unchanged files. Find a
`DRIVE_ID` in the web UI or via `GET /auth/drive/info`.

## Scheduling the sync

Point any scheduler at `archivus-sync sync`, or use the built-in daemon.

**cron** (every 15 minutes):

```cron
*/15 * * * * /home/alice/.local/bin/archivus-sync sync >> /home/alice/.archivus-sync/sync.log 2>&1
```

**systemd timer** — `~/.config/systemd/user/archivus-sync.service`:

```ini
[Service]
Type=oneshot
ExecStart=%h/.local/bin/archivus-sync sync
```

`~/.config/systemd/user/archivus-sync.timer`:

```ini
[Timer]
OnBootSec=2min
OnUnitActiveSec=15min

[Install]
WantedBy=timers.target
```

```sh
systemctl --user enable --now archivus-sync.timer
```

**Built-in daemon** (no external scheduler):

```sh
archivus-sync daemon -interval 15m
```

## Conflict handling (server-side)

When the client re-uploads a file that already exists at the same drive path, the
server decides what to do based on its run mode:

- **home mode** (single user, disk backend): the file is **overwritten in place**
  and its existing metadata row is updated — no duplicate rows.
- **biz mode** (multi-contributor, S3 backend): the previous copy is **kept as a
  version**. The current bytes and metadata are archived under a timestamped key
  (e.g. `report.v<timestamp>.pdf`) before the new content is written to the
  canonical key, so the canonical path is always the latest and no prior copy is
  lost.

The client's change detection means only genuinely-changed files are uploaded, so
biz mode does not accumulate redundant versions on unchanged syncs.


##### 