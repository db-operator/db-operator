#! /bin/bash
# ---------------------------------------------------------------------
# -- Setting permissions. The "postgres" user should be the one
# --  that has the permissions like a user provided by Azure
# ---------------------------------------------------------------------
export PGDATABASE=postgres
export PGUSER=aws_superuser
psql -c "CREATE ROLE postgres;"
psql -c "ALTER ROLE postgres WITH CREATEDB CREATEROLE REPLICATION;"
psql -c "ALTER ROLE aws_superuser WITH NOLOGIN;"
psql -c "CREATE ROLE myadmin WITH LOGIN PASSWORD 'test1234' CREATEDB CREATEROLE;"
psql -c "GRANT postgres TO myadmin;"
psql -c "REVOKE ALL ON pg_database FROM public;"
psql -c "REVOKE ALL ON pg_authid FROM public;"
