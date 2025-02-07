#! /bin/bash
# ---------------------------------------------------------------------
# -- Setting permissions. The "postgres" user should be the one
# --  that has the permissions like a user provided by Azure
# ---------------------------------------------------------------------
export PGDATABASE=postgres
export PGUSER=postgres
psql -c "CREATE ROLE rds_superuser;"
psql -c "ALTER ROLE rds_superuser WITH CREATEDB CREATEROLE REPLICATION;"
psql -c "ALTER ROLE postgres WITH NOLOGIN;"
psql -c "CREATE ROLE myadmin WITH LOGIN PASSWORD 'securepassword' CREATEDB CREATEROLE;"
psql -c "GRANT rds_superuser TO myadmin;"
psql -c "REVOKE ALL ON pg_database FROM public;"
psql -c "REVOKE ALL ON pg_authid FROM public;"
