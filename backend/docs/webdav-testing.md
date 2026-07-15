# Testing the WebDAV server

Archivus exposes each drive as a WebDAV mount so you can browse and edit files
from Finder / Windows Explorer / rclone, alongside the existing REST API.

- **Endpoint:** `http://<host>:8080/webdav/{driveSlug}/...`
- **Auth:** HTTP Basic Auth — your Archivus **username + password** (not the JWT).
- **Layout:** one drive per mount. The mount root (`/webdav/{slug}/`) is that
  drive's top-level contents.
- **Verbs supported:** `OPTIONS`, `PROPFIND`, `GET`, `PUT`, `MKCOL`, `DELETE`,
  `MOVE`, `COPY`, `LOCK`/`UNLOCK`.

---

## 1. Build & run the server

```bash
cd backend
go build ./...                 # sanity check
go run ./cmd/archivus server -m home
```

The server listens on `:8080`.

> **Config note (DEBUG mode):** while `config.DEBUG = true`
> ([internal/config/config.go](../internal/config/config.go)), the config dir is
> resolved from the **current working directory**, not `$HOME`. So the config
> lives at `./.archivus/config.yaml` and files are stored under `./archivus/`,
> relative to wherever you launched the process.
>
> - `-m home` → disk backend (`s3_enabled: false`).
> - `-m biz`  → S3 backend; requires valid S3 credentials (see step 6).
>
> If a `.archivus/config.yaml` already exists with `s3_enabled: true` but you
> have no S3 creds, startup panics. To get a clean disk setup, run from an empty
> directory (a fresh `.archivus/` is generated), e.g.:
> ```bash
> mkdir -p /tmp/archivus-dev && cd /tmp/archivus-dev
> go run <path-to>/backend/cmd/archivus server -m home
> ```

---

## 2. Create a user and drive

Registering a **personal** user also creates a drive named after the username.

```bash
curl -s -X POST http://localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"password123","pin":"123456","email":"a@b.com","user_type":"personal"}'
# -> {"message":"user created"}
```

## 3. Find the drive slug

The slug is `<username>-<8-hex>` (e.g. `alice-1a2b3c4d`), not just the username.
Grab it any of these ways:

```bash
# From disk (fastest): the drive dir under ArchivusHome (./archivus by default)
ls ./archivus | grep -v '^users$'

# Or via the API: login for a token, then read drive info
TOKEN=$(curl -s -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"password123"}' | sed 's/.*"token":"\([^"]*\)".*/\1/')
curl -s http://localhost:8080/auth/drive/info -H "Authorization: Bearer $TOKEN"
```

Export it for the commands below:

```bash
SLUG=alice-1a2b3c4d          # <- replace with your real slug
BASE=http://localhost:8080/webdav/$SLUG
AUTH='-u alice:password123'
```

---

## 4. Exercise every verb with curl

```bash
# OPTIONS — capability discovery. Expect 200 + "DAV: 1, 2" header.
curl -s -D - -o /dev/null $AUTH -X OPTIONS $BASE/ | grep -iE 'HTTP/|^DAV:|^Allow:'

# PROPFIND — list the root (Depth: 1). Expect 207 Multi-Status.
curl -s -o /dev/null -w 'status=%{http_code}\n' $AUTH -X PROPFIND -H 'Depth: 1' $BASE/

# PUT — upload a file. Expect 201 Created.
echo "hello webdav" > /tmp/hello.txt
curl -s -o /dev/null -w 'status=%{http_code}\n' $AUTH -T /tmp/hello.txt $BASE/hello.txt

# GET — download it back. Expect "hello webdav".
curl -s $AUTH $BASE/hello.txt

# MKCOL — create a folder. Expect 201.
curl -s -o /dev/null -w 'status=%{http_code}\n' $AUTH -X MKCOL $BASE/docs

# MOVE — move the file into the folder. Expect 201/204.
curl -s -o /dev/null -w 'status=%{http_code}\n' \
  $AUTH -X MOVE -H "Destination: $BASE/docs/hello.txt" $BASE/hello.txt

# PROPFIND the folder — should now list hello.txt.
curl -s $AUTH -X PROPFIND -H 'Depth: 1' $BASE/docs/ | grep -oE '<D:href>[^<]*</D:href>'

# COPY — duplicate it. Expect 201.
curl -s -o /dev/null -w 'status=%{http_code}\n' \
  $AUTH -X COPY -H "Destination: $BASE/docs/hello-copy.txt" $BASE/docs/hello.txt

# DELETE — remove the whole folder. Expect 204.
curl -s -o /dev/null -w 'status=%{http_code}\n' $AUTH -X DELETE $BASE/docs

# GET the deleted file — expect 404.
curl -s -o /dev/null -w 'status=%{http_code}\n' $AUTH $BASE/docs/hello.txt
```

### Cross-check consistency
WebDAV and the REST API share the same storage + metadata DB, so changes made
over WebDAV should be visible via REST and on disk:

```bash
# REST view of the same files (uses the JWT from step 3)
curl -s -X POST http://localhost:8080/storage/files \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d "{\"path\":\"\",\"driveId\":\"<driveId>\"}"

# On-disk view (disk backend)
find ./archivus/$SLUG
```

### Auth checks
```bash
# No credentials -> 401 with a WWW-Authenticate challenge.
curl -s -D - -o /dev/null -X PROPFIND -H 'Depth: 0' $BASE/ | grep -iE 'HTTP/|WWW-Authenticate'

# Wrong password -> 401.
curl -s -o /dev/null -w 'status=%{http_code}\n' -u alice:wrong -X PROPFIND -H 'Depth: 0' $BASE/
```

---

## 5. Mount it natively

**macOS Finder:** Go → Connect to Server (`⌘K`) →
`http://localhost:8080/webdav/<slug>/` → enter username/password.

**Windows Explorer:** Map network drive → Folder:
`http://localhost:8080/webdav/<slug>/`. (Windows prefers HTTPS; for plain HTTP
you may need the `BasicAuthLevel` registry tweak.)

**Linux (davfs2):**
```bash
sudo mount -t davfs http://localhost:8080/webdav/<slug>/ /mnt/archivus
```

**rclone (any OS):**
```bash
rclone config create arc webdav \
  url=http://localhost:8080/webdav/<slug>/ vendor=other \
  user=alice pass="$(rclone obscure password123)"
rclone lsd arc:
rclone copy /tmp/hello.txt arc:
```

Then browse, upload, download, rename, and delete like a normal folder.

---

## 6. S3 backend

The same tests apply with the S3 backend; only bytes storage differs (metadata
still lives in the local DB).

```bash
go run ./cmd/archivus server -m biz    # requires valid S3 credentials
```

S3 credentials are loaded via `config.DefaultS3Paths()` /
[internal/config/s3_config.go](../internal/config/s3_config.go). Re-run the
curl and native-mount tests from steps 4–5 against a drive on this instance.

> Note: over S3, `MOVE`/`COPY`/`DELETE` on a directory are O(number of objects)
> (list + per-object copy/delete), so large folders take proportionally longer.

---

## 7. Troubleshooting

| Symptom | Likely cause |
|---|---|
| `drive not found` (404) on every request | Wrong slug — it's `username-xxxxxxxx`, not the bare username (step 3). |
| Startup panic mentioning S3 | Existing `.archivus/config.yaml` has `s3_enabled: true` but no creds. Run from a clean cwd (step 1 note). |
| `401` even with correct creds | Using the JWT instead of Basic Auth — WebDAV uses **username:password**. |
| Finder/Explorer won't mount, curl works | Client did an `OPTIONS` preflight and needs the `DAV` header; confirm `OPTIONS` returns `DAV: 1, 2` (step 4). |
| File sizes look slightly off in directory listings | Directory listings report size derived from stored MB; a per-file `PROPFIND` (Depth 0) reports the exact byte size. |
