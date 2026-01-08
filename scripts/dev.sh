#!/bin/bash

# Development environment startup script

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting AgentMesh Development Environment${NC}"

# Check required tools
check_tool() {
    if ! command -v $1 &> /dev/null; then
        echo -e "${RED}Error: $1 is not installed${NC}"
        exit 1
    fi
}

check_tool docker
check_tool docker-compose

# Create .env file if not exists
if [ ! -f "$PROJECT_ROOT/.env" ]; then
    echo -e "${YELLOW}Creating .env file from template...${NC}"
    cat > "$PROJECT_ROOT/.env" << ENVEOF
# Database
POSTGRES_PASSWORD=agentmesh_dev

# Redis
REDIS_PASSWORD=

# JWT Secret (generate a secure one for production)
JWT_SECRET=dev-jwt-secret-change-in-production

# Stripe (optional for development)
STRIPE_SECRET_KEY=

# CORS Origins
CORS_ORIGINS=http://localhost:3000

# Gin Mode
GIN_MODE=debug
ENVEOF
    echo -e "${GREEN}.env file created. Please review and update as needed.${NC}"
fi

# Start infrastructure services
echo -e "${YELLOW}Starting infrastructure services (PostgreSQL, Redis)...${NC}"
docker-compose -f "$PROJECT_ROOT/docker-compose.yml" up -d postgres redis

# Wait for services to be healthy
echo -e "${YELLOW}Waiting for services to be ready...${NC}"
sleep 5

echo -e "${GREEN}Infrastructure services are ready!${NC}"

# Print instructions
echo ""
echo -e "${GREEN}Development environment is ready!${NC}"
echo ""
echo "To start the backend:"
echo "  cd backend && go run ./cmd/server"
echo ""
echo "To start the web frontend:"
echo "  cd web && npm run dev"
echo ""
echo "To start the mobile app:"
echo "  cd mobile && flutter run"
echo ""
echo "Service URLs:"
echo "  - Backend API: http://localhost:8080"
echo "  - Web Frontend: http://localhost:3000"
echo "  - PostgreSQL: localhost:5432"
echo "  - Redis: localhost:6379"
echo ""
echo "To stop all services:"
echo "  docker-compose down"
