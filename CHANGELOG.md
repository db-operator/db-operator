# Changelog

- feat: Add SSL_MODE to the Database ConfigMap | https://github.com/db-operator/db-operator/pull/44

## v1.13.1
- fix: Correct public schema access in Postgres 15 | https://github.com/db-operator/db-operator/pull/45 
- fix: Set an access type to the main user when database is removed | https://github.com/db-operator/db-operator/pull/46

## v1.13.0
- feat: Create a new CRD for user management | https://github.com/db-operator/db-operator/pull/18
- feat: Make field that should not be changed, immutable by webhook | https://github.com/db-operator/db-operator/pull/32
- feat: Add a validating webhook for `secretsTemplates` | https://github.com/db-operator/db-operator/pull/31
- feat: Some cleanup after the project migration | https://github.com/db-operator/db-operator/pull/19
- build: Add more tests for checking databases compatibility | https://github.com/db-operator/db-operator/pull/37
- build: Update Dockerfile and GO version | https://github.com/db-operator/db-operator/pull/21
- docs: Fix documentation for `Databases` | https://github.com/db-operator/db-operator/pull/30
- dev: Add a db-operator binary to the `.gitignore` | https://github.com/db-operator/db-operator/pull/20
- dev: Add entries to Makefile, so it's possible to use nerdctl | https://github.com/db-operator/db-operator/pull/27

## v1.12.0
- feat: Migrate jobs from v1beta1 to v1 | https://github.com/db-operator/db-operator/pull/16
- fix: Secrets are not recreated anymore | https://github.com/db-operator/db-operator/pull/5

## v1.11.0
- feat: Add the Postgres tempalte feature support | https://github.com/db-operator/db-operator/pull/1

## v1.10.1
- The project migration to another organization | https://github.com/db-operator/db-operator/pull/3

