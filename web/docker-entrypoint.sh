#!/bin/sh
# =============================================================================
# Next.js Runtime Environment Variable Injection
# =============================================================================
#
# Next.js 的 NEXT_PUBLIC_* 变量在构建时内联到 JS 中。
# 此脚本在容器启动时将占位符替换为实际环境变量，实现运行时配置。
#
# 占位符格式: __PLACEHOLDER_NAME__
# =============================================================================

set -e

# Replace placeholders in all JS files
echo "Injecting runtime environment variables..."

# Find and replace in .next directory
find /app/.next -type f -name "*.js" -exec sed -i \
  -e "s|__PRIMARY_DOMAIN__|${PRIMARY_DOMAIN:-}|g" \
  -e "s|__USE_HTTPS__|${USE_HTTPS:-false}|g" \
  -e "s|__POSTHOG_KEY__|${POSTHOG_KEY:-}|g" \
  -e "s|__POSTHOG_HOST__|${POSTHOG_HOST:-}|g" \
  {} \;

echo "Environment variables injected:"
echo "  PRIMARY_DOMAIN=${PRIMARY_DOMAIN:-<empty>}"
echo "  USE_HTTPS=${USE_HTTPS:-false}"
echo "  POSTHOG_KEY=$([ -n "${POSTHOG_KEY}" ] && echo '<set>' || echo '<empty>')"
echo "  POSTHOG_HOST=${POSTHOG_HOST:-<empty>}"

# Execute the main command
exec "$@"
