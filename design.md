#### Enterprise/Home Shared Drive Model

1. Setup Drive -> Create Master user (each drive has a unique id)
2. Create further users -> Approved by master, read/write access
3. Master user can list users and remove or add them



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
1. API handlers
2. Backup service
3. Syncer service (outside backend)
4. Indexing, photo, and maybe even search indexing service
