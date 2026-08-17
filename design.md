#### Enterprise/Home Shared Drive Model

1. Private Users -> signup -> create drive for them 
2. Private users (home hosted) -> master user created during installation -> create one big drive

2. Business Users -> signup as admin user -> create drive manually -> invite other users
3. Business Users can be admins or non-admins
4. Admins can invite other users to their drive


#### Design
1. Upload,Move,Copy files
2. Delete only for master users
3. Web UI for anywhere file access
4. connect to filesystem via WebDAV

5. Deduplication
6. Versioning
7. Encrypted Backups


#### Workflow
1. Create Master User -> Create New Drive
2. Create New User -> Add to Drive


#### Services
1. API handlers [X]
2. Backup service [X] Automated (outside backend)
3. Syncer service (outside backend) [X] (outside backend)
5. Move/Delete file. Move/delete folder. [X]

4. Indexing, photo, and maybe even search indexing service
6. Docs parse and save content, indexing, search
7. Thumbnail fix [X]
8. chunk uploads for syncer, large uploads [X]
9. filter/sort
10. search
11. encrypted folders
12. both s3 and diskmanager working together (maybe)
13. cold storage mode



Client arch
1. saving to local lan machine
2. syncer client picks up form it and uploads to archivus

#### Syncer architecture changes
1. Work only when enough CPU and memory available else pause
2. Create a dashboard to show no. of files in the current dir structure, how many are uploaded, how many are skipped, how many failed
3. 


4. Stream assembly directly into the storage write instead of materializing assembled-*.bin, or at least make disk headroom a monitored precondition. (Disk headroom precondition satisfied )
5. sqlite.Open(dbFile + "?_journal_mode=WAL&_busy_timeout=5000").
6. Parallelize the client: N files concurrently, and pipeline chunks within a file.
7. Checkpoint state periodically (every N files or T seconds), prune entries for files that no longer exist, and switch off MarshalIndent.
8. Skip >2 GB files as Skipped, not Failed, so the exit code stays meaningful.
