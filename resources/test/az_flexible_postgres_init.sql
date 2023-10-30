CREATE USER "postgres" WITH PASSWORD 'test1234' CREATEDB CREATEROLE;
GRANT pg_monitor TO "az_admin";
GRANT az_admin TO "postgres";
GRANT pg_read_all_settings TO "postgres";
GRANT pg_read_all_stats TO "postgres";
GRANT pg_stat_scan_tables to "postgres";