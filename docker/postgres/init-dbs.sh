#!/bin/bash
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-EOSQL
    SELECT 'CREATE DATABASE payflow_accounts' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'payflow_accounts')\gexec
    SELECT 'CREATE DATABASE payflow_transfers' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'payflow_transfers')\gexec
EOSQL
