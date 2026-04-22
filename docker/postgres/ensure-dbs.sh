#!/bin/bash
set -e

echo "Creating databases if not exists..."
psql -v ON_ERROR_STOP=1 -U payflow -tc "SELECT 1 FROM pg_database WHERE datname = 'payflow_accounts'" | grep -q 1 || \
    psql -v ON_ERROR_STOP=1 -U payflow -c "CREATE DATABASE payflow_accounts"
echo "payflow_accounts: OK"

psql -v ON_ERROR_STOP=1 -U payflow -tc "SELECT 1 FROM pg_database WHERE datname = 'payflow_transfers'" | grep -q 1 || \
    psql -v ON_ERROR_STOP=1 -U payflow -c "CREATE DATABASE payflow_transfers"
echo "payflow_transfers: OK"
