#!/bin/bash

# Database migration script

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Load environment variables
if [ -f "$PROJECT_ROOT/.env" ]; then
    export $(grep -v '^#' "$PROJECT_ROOT/.env" | xargs)
fi

DATABASE_URL="${DATABASE_URL:-postgres://agentmesh:agentmesh_dev@localhost:5432/agentmesh?sslmode=disable}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Check for golang-migrate
if ! command -v migrate &> /dev/null; then
    echo -e "${YELLOW}golang-migrate not found. Installing...${NC}"
    go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
fi

MIGRATIONS_PATH="$PROJECT_ROOT/backend/migrations"

usage() {
    echo "Usage: $0 <command> [args]"
    echo ""
    echo "Commands:"
    echo "  up            Apply all pending migrations"
    echo "  down [N]      Rollback N migrations (default: 1)"
    echo "  force VERSION Force set migration version"
    echo "  version       Show current migration version"
    echo "  create NAME   Create a new migration"
    echo ""
}

case "${1:-}" in
    up)
        echo -e "${GREEN}Applying migrations...${NC}"
        migrate -path "$MIGRATIONS_PATH" -database "$DATABASE_URL" up
        echo -e "${GREEN}Migrations applied successfully!${NC}"
        ;;
    down)
        N="${2:-1}"
        echo -e "${YELLOW}Rolling back $N migration(s)...${NC}"
        migrate -path "$MIGRATIONS_PATH" -database "$DATABASE_URL" down "$N"
        echo -e "${GREEN}Rollback complete!${NC}"
        ;;
    force)
        if [ -z "$2" ]; then
            echo -e "${RED}Error: VERSION required${NC}"
            usage
            exit 1
        fi
        echo -e "${YELLOW}Forcing version to $2...${NC}"
        migrate -path "$MIGRATIONS_PATH" -database "$DATABASE_URL" force "$2"
        echo -e "${GREEN}Version forced!${NC}"
        ;;
    version)
        migrate -path "$MIGRATIONS_PATH" -database "$DATABASE_URL" version
        ;;
    create)
        if [ -z "$2" ]; then
            echo -e "${RED}Error: NAME required${NC}"
            usage
            exit 1
        fi
        echo -e "${GREEN}Creating migration: $2${NC}"
        migrate create -ext sql -dir "$MIGRATIONS_PATH" -seq "$2"
        echo -e "${GREEN}Migration files created!${NC}"
        ;;
    *)
        usage
        exit 1
        ;;
esac
