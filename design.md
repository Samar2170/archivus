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
1. API handlers
2. Backup service
3. Syncer service (outside backend)
4. Indexing, photo, and maybe even search indexing service
