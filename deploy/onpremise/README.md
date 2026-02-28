# AgentsMesh OnPremise Deployment

私有部署方案，支持 IP 直连，无需域名，适用于内网环境。

## 系统要求

- Docker 20.10+
- Docker Compose V2
- 4GB+ RAM
- 20GB+ 磁盘空间

## 快速开始

### 方式一：使用安装脚本（推荐）

```bash
# 解压部署包
tar -xzf agentsmesh-onpremise-*.tar.gz
cd agentsmesh-onpremise

# 一键安装
./scripts/install.sh --ip 192.168.1.100

# 自定义端口
./scripts/install.sh --ip 192.168.1.100 --http-port 8080 --grpc-port 9443
```

### 方式二：手动安装

```bash
# 1. 加载 Docker 镜像
./scripts/load-images.sh

# 2. 生成 SSL 证书
./scripts/generate-certs.sh 192.168.1.100

# 3. 创建配置文件
cp .env.template .env
# 编辑 .env 填写配置

# 4. 启动服务
docker compose up -d

# 5. 运行数据库迁移
docker compose exec backend migrate -path /app/migrations \
    -database "postgres://agentsmesh:<DB_PASSWORD>@postgres:5432/agentsmesh?sslmode=disable" up

# 6. 导入初始数据
docker compose exec -T postgres psql -U agentsmesh -d agentsmesh < seed/onpremise-seed.sql
```

## 访问地址

| 服务 | URL | 说明 |
|------|-----|------|
| 前端 | http://{SERVER_IP} | 用户界面 |
| 管理后台 | http://{SERVER_IP}/admin | 系统管理 |
| MinIO 控制台 | http://{SERVER_IP}:9001 | 对象存储管理 |

## 默认账户

| 账户类型 | 邮箱 | 密码 |
|----------|------|------|
| 管理员 | admin@local | Admin@123 |

> ⚠️ **重要**: 首次登录后请立即修改密码！

## Runner 注册

在需要执行 AI Agent 任务的机器上：

```bash
# 下载 Runner
# Linux
curl -LO https://github.com/AgentsMesh/agentsmesh/releases/latest/download/runner-linux-amd64
chmod +x runner-linux-amd64

# macOS
curl -LO https://github.com/AgentsMesh/agentsmesh/releases/latest/download/runner-darwin-amd64
chmod +x runner-darwin-amd64

# 注册 Runner
./runner register --server http://192.168.1.100 --token onpremise-runner-token

# 启动 Runner
./runner run
```

## 目录结构

```
deploy/onpremise/
├── docker-compose.yml      # Docker Compose 配置
├── .env.template           # 环境变量模板
├── .env                    # 实际配置（install.sh 生成）
├── traefik/
│   ├── traefik.yml         # Traefik 静态配置
│   └── dynamic/
│       └── grpc.yml        # gRPC mTLS 配置
├── ssl/                    # SSL 证书（generate-certs.sh 生成）
│   ├── ca.crt              # CA 证书
│   ├── ca.key              # CA 私钥
│   ├── server.crt          # 服务器证书
│   └── server.key          # 服务器私钥
├── seed/
│   └── onpremise-seed.sql  # 初始数据
├── images/                 # Docker 镜像（离线部署）
│   ├── backend.tar
│   ├── web.tar
│   ├── web-admin.tar
│   ├── relay.tar
│   ├── postgres.tar
│   ├── redis.tar
│   ├── minio.tar
│   └── traefik.tar
└── scripts/
    ├── install.sh          # 一键安装
    ├── load-images.sh      # 加载镜像
    └── generate-certs.sh   # 生成证书
```

## 常用命令

```bash
# 查看服务状态
docker compose ps

# 查看日志
docker compose logs -f              # 所有服务
docker compose logs -f backend      # 仅 Backend
docker compose logs -f web          # 仅 Web

# 重启服务
docker compose restart backend

# 停止服务
docker compose down

# 停止并清除数据
docker compose down -v

# 更新配置后重启
docker compose up -d
```

## 数据备份

### 备份数据库

```bash
# 备份
docker compose exec postgres pg_dump -U agentsmesh agentsmesh > backup.sql

# 恢复
docker compose exec -T postgres psql -U agentsmesh -d agentsmesh < backup.sql
```

### 备份所有数据

```bash
# 停止服务
docker compose down

# 备份 volumes
docker run --rm -v agentsmesh_postgres_data:/data -v $(pwd):/backup alpine \
    tar czf /backup/postgres_backup.tar.gz -C /data .

docker run --rm -v agentsmesh_minio_data:/data -v $(pwd):/backup alpine \
    tar czf /backup/minio_backup.tar.gz -C /data .

# 启动服务
docker compose up -d
```

## 故障排查

### 服务无法启动

```bash
# 检查 Docker 状态
docker info

# 查看详细日志
docker compose logs --tail=100

# 检查端口占用
netstat -tlnp | grep -E ':(80|9443|5432|6379|9000)\s'
```

### 数据库连接失败

```bash
# 检查 PostgreSQL 状态
docker compose exec postgres pg_isready -U agentsmesh

# 查看数据库日志
docker compose logs postgres
```

### Runner 无法连接

1. 确认防火墙开放端口：80 (HTTP)、9443 (gRPC)
2. 检查证书是否包含服务器 IP
3. 查看 Backend 日志：`docker compose logs backend | grep -i grpc`

## 安全建议

1. **修改默认密码**: 首次登录后立即修改管理员密码
2. **防火墙配置**: 仅开放必要端口（80, 9443）
3. **定期备份**: 定期备份数据库和对象存储
4. **更新证书**: SSL 证书有效期 1 年，到期前需要更新
5. **日志审计**: 定期检查 Admin 审计日志

## 版本升级

```bash
# 1. 备份数据
./scripts/backup.sh  # (如果有)

# 2. 加载新版本镜像
./scripts/load-images.sh

# 3. 重新创建容器
docker compose up -d

# 4. 运行迁移（如果需要）
docker compose exec backend migrate -path /app/migrations \
    -database "postgres://agentsmesh:${DB_PASSWORD}@postgres:5432/agentsmesh?sslmode=disable" up
```

## 技术支持

如有问题，请联系技术支持或提交 Issue。
