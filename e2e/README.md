# E2E 测试用例

本目录包含 AgentsMesh 的端到端测试用例，采用结构化 YAML 格式编写，可供 Claude Code 或其他自动化测试工具执行。

## 目录结构

```
e2e/
├── README.md                          # 本文件
└── billing/                           # 账单模块
    ├── subscription/                  # 订阅管理
    │   ├── TC-SUB-001-status-display.yaml
    │   ├── TC-SUB-002-plans-dialog.yaml
    │   ├── TC-SUB-003-cancel-at-period-end.yaml
    │   ├── TC-SUB-004-cancel-immediately.yaml
    │   ├── TC-SUB-005-reactivate.yaml
    │   └── TC-SUB-006-plan-upgrade.yaml
    ├── seats/                         # 席位管理
    │   ├── TC-SEAT-001-display.yaml
    │   ├── TC-SEAT-002-add-dialog.yaml
    │   └── TC-SEAT-003-free-plan-limit.yaml
    ├── billing-cycle/                 # 计费周期
    │   ├── TC-CYCLE-001-display.yaml
    │   ├── TC-CYCLE-002-monthly-to-yearly.yaml
    │   └── TC-CYCLE-003-yearly-to-monthly.yaml
    ├── promo-code/                    # 优惠码
    │   ├── TC-PROMO-001-display.yaml
    │   ├── TC-PROMO-002-valid-code.yaml
    │   └── TC-PROMO-003-invalid-code.yaml
    └── quota/                         # 配额检查
        ├── TC-QUOTA-001-users.yaml
        ├── TC-QUOTA-002-runners.yaml
        └── TC-QUOTA-003-repositories.yaml
```

## 测试用例格式

```yaml
id: TC-XXX-001
name: 测试用例名称
description: 测试用例描述
priority: high | medium | low
module: billing/subscription

preconditions:
  - 前置条件

setup:
  sql: |
    -- 可选的数据库初始化 SQL

steps:
  - action: 操作描述
    expected: 预期结果
    verification:
      type: ui | api | database
      details: 验证详情

cleanup:
  - sql: |
      -- 清理 SQL
```

## 执行测试

### 使用 Claude Code 执行

```
请执行 e2e/billing/subscription/TC-SUB-001-status-display.yaml 测试用例
```

或执行整个模块：

```
请执行 e2e/billing/subscription/ 目录下的所有测试用例
```

### 验证方式

| 类型 | 说明 | 示例 |
|------|------|------|
| `ui` | 浏览器快照验证 | 检查页面元素、文本、按钮状态 |
| `api` | API 调用验证 | curl 请求，验证状态码和响应 |
| `database` | 数据库查询验证 | psql 执行 SQL，验证数据状态 |

## 测试数据

| 数据 | 值 |
|------|-----|
| 测试用户邮箱 | dev@agentsmesh.local |
| 测试用户密码 | devpass123 |
| 测试组织 slug | dev-org |
| 默认订阅计划 | pro |
| 账单页面路径 | /dev-org/settings?scope=organization&tab=billing |
