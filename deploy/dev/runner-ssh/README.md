# Runner SSH Keys

此目录包含 dev runner 用于访问 Git 仓库的 SSH 密钥。

## 文件说明

- `id_ed25519` - 私钥（**不要提交到 Git**）
- `id_ed25519.pub` - 公钥（可以提交，用于参考）
- `config` - SSH 客户端配置
- `known_hosts` - 已知主机列表

## 生成新密钥

如果私钥丢失或需要重新生成：

```bash
# 生成新的 ED25519 密钥
ssh-keygen -t ed25519 -C "agentsmesh-dev-runner@local" -f ./id_ed25519 -N ""

# 生成 known_hosts
ssh-keyscan -p 2222 gitlab.corp.signalrender.com > known_hosts
```

## 在 GitLab 上配置

将公钥添加为项目的 Deploy Key：

```bash
PUBKEY=$(cat id_ed25519.pub)
GITLAB_HOST=gitlab.corp.signalrender.com glab api -X POST projects/12/deploy_keys \
  -f title="AgentsMesh Dev Runner" \
  -f key="$PUBKEY" \
  -f can_push=true
```

或者在 GitLab Web UI 中：
1. 进入项目 Settings > Repository > Deploy Keys
2. 添加公钥内容
3. 勾选 "Grant write permissions to this key"

## 测试连接

```bash
# 在 runner 容器中测试
docker compose exec runner ssh -T git@gitlab.corp.signalrender.com
```
