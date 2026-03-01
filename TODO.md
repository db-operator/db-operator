TODO:

- [ ] DbBackup and DbRestore CRDs
- [ ] Implement a cleanup for the DbBackup (Remove backup from the storage)
- [ ] DbBackupGroup CRD - creates DbBackup on a cron based schedule
- [ ] DbBackup should be engine aware (probably we can get the data from the status)
- [ ] DbRestore should be engine aware
- [ ] Prepare a list of env vars that are passed to the db-backup-cli 
- [ ] E2e tests need to be implemented (maybe in the helmfile somehow)
- [ ] Docs need to be written
    - How to use default
    - How to create custom backup containers
