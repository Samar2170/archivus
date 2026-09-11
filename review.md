# Code Review: `folder_deletion` vs `main`

## Verification performed

- Read all four commits individually; traced the new folder-recycle flow end-to-end (handler → service → disk/S3 managers → store).
- Compared against prior behavior: `DeleteDir` previously did `os.RemoveAll` + single metadata-row delete; files already had the recycle-bin flow this branch mirrors.
- Checked all `Store.Transaction` callers for the nested-transaction change (only the new `HideFolderSubtree` call runs nested — the savepoint path is correct for SQLite).
- Verified prefix/key conventions: S3 dir `PathKey` has no trailing slash (`EnsureDirectoryMetadata`), S3 file `Prefix` has one (`UploadFileV2`), disk uses absolute paths — the `prefix = X OR X/ OR X/%` matching in `folder_recycle_store.go` covers both correctly, and the trailing slash in the LIKE pattern prevents `reports` matching `reports-backup`.
- Confirmed path traversal is blocked by the drive-scoped `GetDirectoryByDrivePathKey` lookup before any filesystem/S3 work.
- `go build`, `go vet`, full backend test suite: pass. `svelte-check`: 0 errors. `IsDir` column is handled by AutoMigrate.

## Findings

### Medium — S3 `restoreFolder` mid-failure leaves a partial folder at the destination, permanently blocking restore
`backend/internal/services/storagemanager/s3manager/dirmanager.go:209-226`

`unwindRestores` copies already-restored objects *back* to the recycle prefix but never deletes them from the destination keys (unlike `copyRecycledBackToBin` at :246-259, which does delete after copying). If any `CopyObject` in the restore loop fails, the destination prefix keeps partial content, the bin copy stays intact, and the item row survives — but every retry now hits the `len(existing) > 0` check at :204 and returns "a folder already exists at the original location". The folder is stuck: it can't be restored, and it can't be re-deleted because its metadata rows are soft-deleted (so `GetDirectoryByDrivePathKey` finds nothing).

**Failure scenario:** R2 throttles/transiently errors on the 3rd of 500 object copies during restore → folder permanently un-restorable via the API.
**Fix:** In `unwindRestores`, delete the restored originals after copying them back (mirror `copyRecycledBackToBin`).

### Medium — Restoring an older recycle item resurrects rows belonging to a newer item of the same path
`backend/internal/store/folder_recycle_store.go:80-94` (`UnhideFolderSubtree`, same issue in `HardDeleteHiddenFolderSubtree` :99-113)

The `deleted_at >= hiddenSince` guard only excludes rows hidden *before* the item was created; it cannot distinguish generations hidden *after*. Soft-deleted rows keep the same `path_key`/`prefix`, so all generations match.

**Failure scenario (disk or S3):** delete folder `D` (item1) → recreate `D`, upload files → delete `D` again (item2) → the bin now shows two identically named `D` entries → user restores the *older* one. `UnhideFolderSubtree(…, item1.CreatedAt)` resurrects **both** generations' rows: duplicate directory/file rows for the same keys appear in listings, `GetDirectoryByDrivePathKey`'s `.First()` becomes nondeterministic, and purging item2 afterwards removes nothing (rows are live/NULL again), so the duplicates persist with no API to fix them.
**Fix:** Tag the hidden rows with the recycle item (e.g. a `recycle_batch_id` column set during `HideFolderSubtree` and filtered on in unhide/hard-delete), or refuse restore while a *newer* recycle item exists for the same `OriginalPathKey` + drive.

### Medium — Uploads racing a folder delete get hidden but not recycled → restore permanently blocked, bytes orphaned
`backend/internal/services/storagemanager/diskmanager/dirmanager.go:236-246`, `backend/internal/services/storagemanager/s3manager/dirmanager.go:134-167`

The folder's rows are hidden *after* the bytes are moved, and neither step excludes objects uploaded in between:
- **Disk:** an upload between `os.Rename` (:236) and `HideFolderSubtree` recreates the directory and inserts a row, which the hide then soft-deletes. The recycled tree doesn't contain the new file. On restore, `os.Stat(dstPathKey)` at `movedelete.go:260` finds the recreated dir → "folder already exists" forever; on purge the row is hard-deleted and the file's bytes stay on disk untracked.
- **S3:** the window is much wider — objects uploaded after `ListObjects` (:134) but before `HideFolderSubtree` are hidden yet never copied to the bin nor deleted from the origin, producing the same stuck/orphan outcome.

**Fix:** Do the visibility flip first (hide rows inside a transaction that also inserts the recycle item), then relocate bytes, or re-list/verify after hiding and fold stragglers into the item; at minimum, make restore tolerate/reconcile existing content at the destination instead of hard-failing.

### Medium — S3 folder delete copies objects one-by-one, synchronously, inside the HTTP request
`backend/internal/services/storagemanager/s3manager/dirmanager.go:146-153`

`DeleteDir` runs in the request path and now issues one sequential `CopyObject` per object (plus the batched delete). The old code did one `ListObjects` + batched `DeleteObjects`. A folder with a few thousand objects means thousands of sequential R2 round-trips in a single request — multi-minute handler, gateway timeouts, and client retries that re-enter `DeleteDir` while the first attempt is still running (rows are only hidden at the end, so a retry lists the same originals and produces duplicate bin copies/items). Note the server-side work isn't cancellable (`context.Background()`), so the operation completes after the client has given up.
**Fix:** Run the copy phase in a background job (the pending-upload machinery is a precedent), or at least bound/batch it and reject oversized folders up front.

### Low — Restore's destination-conflict check proceeds on ListObjects error
`backend/internal/services/storagemanager/s3manager/dirmanager.go:203-206`

`if err == nil && len(existing) > 0` means a transient list failure silently skips the "already exists" guard, and the subsequent `CopyObject`s overwrite whatever a recreated same-named folder holds at those keys. Abort when the check can't be performed.

### Low — `CreateDirV2` ancestor repair leaves phantom rows if the leaf insert fails
`backend/internal/services/storagemanager/diskmanager/dirmanager.go:131-163`

The new loop creates ancestor metadata rows before the leaf is created; if `CreateDirectoryMetadataV2` for the leaf then fails, cleanup `os.RemoveAll(pathKey)` deletes the whole on-disk chain but the ancestor rows persist, so listings show folders that don't exist. Self-healing on retry, but wrap ancestor creation in the same failure cleanup (or create them after the leaf succeeds).

## Overall assessment

The core design is sound: metadata is soft-deleted in place (keys preserved) so restore needs no path rewriting, byte relocation is rollback-guarded on both backends, the delete batch is correctly sized to 1000, and auth/ownership checks on the new purge endpoint match the existing restore path. The integration tests cover the main happy paths well. The issues are concentrated in failure/race paths of the new S3 folder operations and in generation ambiguity of the soft-delete scheme.

**Findings:** 0 Critical · 0 High · 4 Medium · 2 Low

**Areas reviewed:** recycle flow for folders (disk + S3): delete/restore/purge/immediate-purge; `folder_recycle_store.go` query semantics; nested `Transaction` change and all callers; `CreateDirV2` ancestor creation; models/AutoMigrate; handler + route wiring; frontend delete/purge flows and API client; integration tests; build/vet/tests/svelte-check.

**Remaining risks:** the S3 manager has no test coverage (R2-dependent); the S3 `DeleteObjects` wrapper ignores per-key errors (pre-existing, but folder purge now amplifies its blast radius — failed keys leave orphaned objects after purge); no tests for restore-conflict, cross-generation restore, or concurrent-upload-during-delete; restoring a folder whose parent chain was recycled recreates parent dirs without metadata rows (pre-existing pattern shared with file restore).
