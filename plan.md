## Shared Folder Implementation Plan (Low-Risk, Incremental)

### Goal
Implement shared-folder access without breaking existing `/storage/*` flows, while keeping design and operations simple.

### Scope for v1
- Share folders only with existing Archivus users (no public links, no guests).
- Add a dedicated shared API surface (`/storage/shared/*`) so current drive APIs remain unchanged.
- Start read-only (`list`, `download`), then add write later.

### Data Model
1. Add a new model/table: `shared_info`
   - `id` (uuid)
   - `drive_id` (uuid, indexed)
   - `root_path_key` (string, indexed)  // shared folder root path
   - `shared_with_user_id` (uuid, indexed)
   - `access_level` (`read` | `write`)
   - `granted_by_user_id` (uuid)
   - timestamps
2. Add unique constraint:
   - `(drive_id, root_path_key, shared_with_user_id)`
3. Do not add `is_shared` to `DirectoryMetadata`/`FileMetadata` in v1.
   - Reason: avoids recursive subtree updates and stale flags.

### Backend Design
1. Store layer
   - Create CRUD/query methods for `shared_info`:
     - grant share (upsert)
     - revoke share
     - list roots shared with user
     - resolve matching share for a requested path (ancestor match)

2. Permission helper
   - Add a shared-folder access resolver for `/storage/shared/*` only:
     - user must have a matching share root for requested path
     - `read` allows list/download
     - `write` reserved for phase 2 mutations

3. New API endpoints (`/storage/shared/*`)
   - `GET /storage/shared/roots` -> roots shared with current user
   - `POST /storage/shared/list` body `{ driveId, rootPath, path, page, pageSize, category, sortBy, sortOrder }`
   - `GET /storage/shared/file/download?driveId=...&fileId=...&rootPath=...`
   - Admin/owner/manager management endpoints:
     - `POST /storage/shared/grant`
     - `POST /storage/shared/revoke`
     - `POST /storage/shared/list-users`

4. Service implementation strategy
   - Reuse existing file/folder listing and download internals where possible.
   - Add pre-checks that constrain all operations to `root_path_key` subtree.
   - Keep current `/storage/files`, `/storage/file/download`, etc. untouched.

### Frontend Design
1. Add new UI entry: `Shared with me`
   - Separate from current drive browsing flow.
2. Add shared API client methods under `archivus-svelte/src/lib/api/`.
3. Shared browser page/state
   - Fetch shared roots first.
   - Open root and navigate only within allowed subtree.
4. Folder context menu (owner/manager)
   - Add `Share folder` action (grant/revoke users + access level).
5. Keep existing main file browser and drive selector behavior unchanged.

### Rollout Phases
1. Phase 1: Schema + store + read-only shared APIs
   - deliver `roots`, `list`, `download`
2. Phase 2: UI for `Shared with me`
   - read-only browsing and downloads
3. Phase 3: Share management UI
   - grant/revoke and list shared users
4. Phase 4: Optional write support for shared folders
   - upload/create/move/delete under shared subtree with `write` access

### Testing Plan
1. Backend integration tests
   - shared user can list/download inside shared root
   - shared user cannot access sibling/unshared paths
   - revoked user loses access immediately
   - invalid root/path combinations are rejected
2. Regression tests
   - existing `/storage/*` tests pass unchanged
   - invite/drive permissions remain unaffected
3. Frontend checks
   - shared roots load correctly
   - navigation cannot escape shared subtree
   - existing drive browsing flows unchanged

### Risk Controls
- Keep shared access in a separate API namespace first.
- Avoid denormalized `is_shared` flags for v1.
- Reuse existing authorization context + storage methods to reduce code duplication.
- Gate write operations behind phase 4 after read-only path is stable.

### Deliverables
1. DB migration/model for `shared_info`
2. Store/service methods for shared access resolution
3. `/storage/shared/*` handlers and routes
4. Frontend `Shared with me` page + API clients
5. Integration/regression test coverage
