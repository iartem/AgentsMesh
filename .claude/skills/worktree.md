---
name: worktree
description: |
  创建 Git worktree 用于隔离开发新功能或修复 bug。
  自动处理分支创建、worktree 设置和目录切换。
user-invocable: true
---

# Git Worktree 创建

创建独立的 worktree 用于并行开发，避免污染主工作目录。

## 使用流程

### 1. 确认参数

需要以下信息：
- **分支名称**: 新功能/修复的分支名（如 `feature/add-login`, `fix/user-auth`）
- **基础分支**: 从哪个分支创建（默认 `main`）
- **worktree 目录**: 放置位置（默认 `../<repo>-<branch>`）

### 2. 执行创建

```bash
# 1. 获取最新代码
git fetch origin

# 2. 创建 worktree 和新分支
git worktree add -b <branch-name> <worktree-path> origin/<base-branch>

# 3. 进入 worktree 目录
cd <worktree-path>

# 4. 验证状态
git status
git log --oneline -3
```

### 3. 完成后提示

创建完成后，告知用户：
- Worktree 路径
- 当前分支名
- 如何切换回主目录
- 如何清理 worktree（`git worktree remove <path>`）

## 示例

用户说："创建一个 worktree 开发用户认证功能"

执行：
```bash
git fetch origin
git worktree add -b feature/user-auth ../AgentMesh-feature-user-auth origin/main
cd ../AgentMesh-feature-user-auth
git status
```

输出：
```
已创建 worktree:
- 路径: /Users/xxx/Works/AIO/AgentMesh-feature-user-auth
- 分支: feature/user-auth (基于 origin/main)

完成开发后：
- 提交代码: git add . && git commit -m "..."
- 推送分支: git push -u origin feature/user-auth
- 清理 worktree: cd .. && git worktree remove AgentMesh-feature-user-auth
```

## 注意事项

- 分支名遵循约定：`feature/*`, `fix/*`, `refactor/*`, `docs/*`
- Worktree 目录默认放在仓库同级目录
- 如果分支已存在，使用 `git worktree add <path> <existing-branch>`
- 清理前确保所有更改已提交或推送
