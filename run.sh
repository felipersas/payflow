#!/bin/bash
set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()  { echo -e "${GREEN}[payflow]${NC} $1"; }
warn() { echo -e "${YELLOW}[payflow]${NC} $1"; }

cleanup() {
    warn "Shutting down..."
    [ -n "$USER_PID" ] && kill $USER_PID 2>/dev/null
    [ -n "$ACCOUNT_PID" ] && kill $ACCOUNT_PID 2>/dev/null
    [ -n "$TRANSFER_PID" ] && kill $TRANSFER_PID 2>/dev/null
    wait 2>/dev/null
    log "Stopped."
    exit 0
}
trap cleanup SIGINT SIGTERM

# 1. Infra
log "Starting infrastructure..."
docker-compose up -d

log "Waiting for Postgres..."
until docker exec payflow-postgres pg_isready -U payflow &>/dev/null; do sleep 1; done

log "Creating databases if not exists..."
docker exec payflow-postgres psql -U payflow -tc "SELECT 1 FROM pg_database WHERE datname = 'payflow_users'" | grep -q 1 || \
    docker exec payflow-postgres psql -U payflow -c "CREATE DATABASE payflow_users"
docker exec payflow-postgres psql -U payflow -tc "SELECT 1 FROM pg_database WHERE datname = 'payflow_accounts'" | grep -q 1 || \
    docker exec payflow-postgres psql -U payflow -c "CREATE DATABASE payflow_accounts"
docker exec payflow-postgres psql -U payflow -tc "SELECT 1 FROM pg_database WHERE datname = 'payflow_transfers'" | grep -q 1 || \
    docker exec payflow-postgres psql -U payflow -c "CREATE DATABASE payflow_transfers"

log "Waiting for RabbitMQ..."
until docker exec payflow-rabbitmq rabbitmq-diagnostics check_port_connectivity &>/dev/null; do sleep 2; done

log "Infrastructure ready."

# 2. User Service (bg)
log "Starting user-service on :8082..."
DB_NAME=payflow_users SERVICE_PORT=8082 SERVICE_NAME=user-service go run cmd/user-service/main.go &
USER_PID=$!

# 3. Account Service (bg)
log "Starting account-service on :8080..."
DB_NAME=payflow_accounts SERVICE_PORT=8080 SERVICE_NAME=account-service go run cmd/account-service/main.go &
ACCOUNT_PID=$!

# 4. Transfer Service (bg)
log "Starting transfer-service on :8081..."
DB_NAME=payflow_transfers SERVICE_PORT=8081 SERVICE_NAME=transfer-service go run cmd/transfer-service/main.go &
TRANSFER_PID=$!

# 5. Wait for health
sleep 3
curl -sf http://localhost:8082/health >/dev/null && log "User service     -> http://localhost:8082" || warn "User service not responding yet"
curl -sf http://localhost:8080/health >/dev/null && log "Account service  -> http://localhost:8080" || warn "Account service not responding yet"
curl -sf http://localhost:8081/health >/dev/null && log "Transfer service -> http://localhost:8081" || warn "Transfer service not responding yet"

log "RabbitMQ Management -> http://localhost:15672 (payflow/payflow123)"
log "Press Ctrl+C to stop all services."

wait
