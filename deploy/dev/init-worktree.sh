#!/bin/bash
# =============================================================================
# AgentMesh Worktree 初始化脚本
# =============================================================================
#
# 自动为当前 worktree 生成独立的 Docker Compose 配置，避免多 worktree 冲突。
#
# 功能：
#   1. 检测当前 worktree/分支名称
#   2. 计算唯一的端口偏移量
#   3. 生成 .env 配置文件
#
# 使用方法：
#   ./init-worktree.sh          # 自动检测并生成配置
#   ./init-worktree.sh --force  # 强制覆盖现有配置
#   ./init-worktree.sh --info   # 显示当前配置信息
#   ./init-worktree.sh --clean  # 清理当前 worktree 的 Docker 资源
#
# =============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="$SCRIPT_DIR/.env"
ENV_EXAMPLE="$SCRIPT_DIR/.env.example"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印带颜色的消息
info() { echo -e "${BLUE}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 获取 worktree 名称
get_worktree_name() {
    local git_dir
    local worktree_name

    # 检查是否在 git worktree 中
    git_dir=$(git rev-parse --git-dir 2>/dev/null)

    if [[ "$git_dir" == *".git/worktrees/"* ]]; then
        # 在 worktree 中，提取 worktree 名称
        worktree_name=$(basename "$(dirname "$git_dir")")
    else
        # 在主仓库中，使用分支名
        worktree_name=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "main")
    fi

    # 清理名称，只保留字母数字和连字符
    echo "$worktree_name" | sed 's/[^a-zA-Z0-9-]/-/g' | tr '[:upper:]' '[:lower:]'
}

# 计算端口偏移量（基于名称的哈希）
calculate_port_offset() {
    local name="$1"
    local hash

    # 使用名称的哈希值计算偏移量
    # 范围: 0-99，每个 worktree 使用 100 个端口的块
    # macOS 兼容性: 优先使用 md5sum，回退到 md5
    if command -v md5sum &>/dev/null; then
        hash=$(echo -n "$name" | md5sum | cut -c1-4)
    else
        hash=$(echo -n "$name" | md5 | cut -c1-4)
    fi
    offset=$((16#$hash % 100))

    # 确保偏移量至少为 0（main 分支）或 1（其他分支）
    if [[ "$name" == "main" || "$name" == "master" ]]; then
        echo 0
    else
        # 偏移量范围 1-99
        echo $(( (offset % 99) + 1 ))
    fi
}

# 计算端口
calculate_port() {
    local base_port="$1"
    local offset="$2"
    echo $((base_port + offset * 100))
}

# 显示配置信息
show_info() {
    if [[ ! -f "$ENV_FILE" ]]; then
        error ".env 文件不存在，请先运行 ./init-worktree.sh"
        exit 1
    fi

    echo ""
    echo "=========================================="
    echo "  AgentMesh 开发环境配置"
    echo "=========================================="
    echo ""

    # 读取配置
    source "$ENV_FILE"

    echo "项目名称:     $COMPOSE_PROJECT_NAME"
    echo ""
    echo "服务端口:"
    echo "  HTTP (Nginx):    http://localhost:$HTTP_PORT"
    echo "  PostgreSQL:      localhost:$POSTGRES_PORT"
    echo "  Redis:           localhost:$REDIS_PORT"
    echo "  MinIO API:       localhost:$MINIO_API_PORT"
    echo "  MinIO Console:   http://localhost:$MINIO_CONSOLE_PORT"
    echo "  Adminer:         http://localhost:$ADMINER_PORT"
    echo ""
    echo "启动命令:"
    echo "  docker compose up -d"
    echo ""
    echo "停止命令:"
    echo "  docker compose down"
    echo ""
}

# 生成 .env 文件
generate_env() {
    local force="$1"
    local worktree_name
    local port_offset

    # 检查是否已存在 .env 文件
    if [[ -f "$ENV_FILE" && "$force" != "true" ]]; then
        warn ".env 文件已存在"
        echo ""
        read -p "是否覆盖现有配置？[y/N] " -n 1 -r
        echo ""
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            info "保留现有配置"
            show_info
            exit 0
        fi
    fi

    # 获取 worktree 名称和端口偏移量
    worktree_name=$(get_worktree_name)
    port_offset=$(calculate_port_offset "$worktree_name")

    info "检测到 worktree/分支: $worktree_name"
    info "端口偏移量: $port_offset (每个 worktree 使用 100 端口块)"

    # 计算端口
    http_port=$(calculate_port 80 $port_offset)
    postgres_port=$(calculate_port 5432 $port_offset)
    redis_port=$(calculate_port 6379 $port_offset)
    minio_api_port=$(calculate_port 9000 $port_offset)
    minio_console_port=$(calculate_port 9001 $port_offset)
    adminer_port=$(calculate_port 8081 $port_offset)

    # 项目名称
    project_name="agentmesh-${worktree_name}"

    # 生成 .env 文件
    cat > "$ENV_FILE" << EOF
# =============================================================================
# AgentMesh Development Environment Configuration
# =============================================================================
# 自动生成于: $(date)
# Worktree/分支: $worktree_name
# 端口偏移量: $port_offset
# =============================================================================

# -----------------------------------------------------------------------------
# 项目隔离配置
# -----------------------------------------------------------------------------

COMPOSE_PROJECT_NAME=$project_name

# -----------------------------------------------------------------------------
# 端口配置
# -----------------------------------------------------------------------------

HTTP_PORT=$http_port
POSTGRES_PORT=$postgres_port
REDIS_PORT=$redis_port
MINIO_API_PORT=$minio_api_port
MINIO_CONSOLE_PORT=$minio_console_port
ADMINER_PORT=$adminer_port

# -----------------------------------------------------------------------------
# 数据库配置
# -----------------------------------------------------------------------------

POSTGRES_PASSWORD=agentmesh_dev

# -----------------------------------------------------------------------------
# 认证配置
# -----------------------------------------------------------------------------

JWT_SECRET=dev-jwt-secret-change-in-production

# -----------------------------------------------------------------------------
# MinIO (S3 兼容存储)
# -----------------------------------------------------------------------------

MINIO_ROOT_USER=minioadmin
MINIO_ROOT_PASSWORD=minioadmin

# -----------------------------------------------------------------------------
# GitHub OAuth (可选)
# -----------------------------------------------------------------------------

GITHUB_CLIENT_ID=
GITHUB_CLIENT_SECRET=

# -----------------------------------------------------------------------------
# Runner 配置
# -----------------------------------------------------------------------------

RUNNER_TOKEN=dev-runner-token
EOF

    success ".env 文件已生成"
    show_info
}

# 清理 Docker 资源
clean_resources() {
    if [[ ! -f "$ENV_FILE" ]]; then
        error ".env 文件不存在，无法确定要清理的资源"
        exit 1
    fi

    source "$ENV_FILE"
    local project_name="${COMPOSE_PROJECT_NAME:-agentmesh}"

    warn "即将清理以下资源："
    echo "  - 项目名称: $project_name"
    echo "  - 容器、网络、卷"
    echo ""

    read -p "确认清理？此操作不可恢复 [y/N] " -n 1 -r
    echo ""

    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        info "取消清理"
        exit 0
    fi

    info "停止并删除容器..."
    docker compose down -v --remove-orphans 2>/dev/null || true

    info "清理悬空资源..."
    docker system prune -f --filter "label=com.docker.compose.project=$project_name" 2>/dev/null || true

    success "清理完成"
}

# 主函数
main() {
    case "${1:-}" in
        --info|-i)
            show_info
            ;;
        --force|-f)
            generate_env "true"
            ;;
        --clean|-c)
            clean_resources
            ;;
        --help|-h)
            echo "用法: $0 [选项]"
            echo ""
            echo "选项:"
            echo "  --info, -i    显示当前配置信息"
            echo "  --force, -f   强制覆盖现有配置"
            echo "  --clean, -c   清理当前 worktree 的 Docker 资源"
            echo "  --help, -h    显示帮助信息"
            echo ""
            echo "示例:"
            echo "  $0            # 自动检测并生成配置"
            echo "  $0 --force    # 强制覆盖现有配置"
            echo "  $0 --info     # 显示当前配置"
            echo "  $0 --clean    # 清理 Docker 资源"
            ;;
        *)
            generate_env "false"
            ;;
    esac
}

main "$@"
