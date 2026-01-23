/**
 * 环境变量工具函数
 *
 * =============================================================================
 * Unified Domain Configuration (推荐)
 * =============================================================================
 * 只需配置两个变量，所有 URL 自动派生：
 * - NEXT_PUBLIC_PRIMARY_DOMAIN → 主域名 (e.g., "localhost:10000" 或 "agentsmesh.com")
 * - NEXT_PUBLIC_USE_HTTPS → 是否使用 HTTPS (true/false)
 *
 * 派生的 URL：
 * - OAuth URL = http(s)://{PRIMARY_DOMAIN}
 * - WebSocket URL = ws(s)://{PRIMARY_DOMAIN}
 *
 * =============================================================================
 * 传统配置方式（向后兼容）
 * =============================================================================
 * 本地开发时（使用 dev.sh）：
 * - NEXT_PUBLIC_API_URL="" → 使用相对路径，由 Next.js rewrites 代理
 * - NEXT_PUBLIC_OAUTH_URL → OAuth 浏览器跳转使用
 * - NEXT_PUBLIC_WS_URL → WebSocket 连接使用
 *
 * Docker/生产环境：
 * - NEXT_PUBLIC_API_URL → 完整的后端 URL
 */

// =============================================================================
// Unified Domain Configuration Helpers
// =============================================================================

/**
 * 获取主域名配置
 */
function getPrimaryDomain(): string | undefined {
  return process.env.NEXT_PUBLIC_PRIMARY_DOMAIN;
}

/**
 * 是否使用 HTTPS
 */
function isHttpsEnabled(): boolean {
  return process.env.NEXT_PUBLIC_USE_HTTPS === "true";
}

/**
 * 从 PRIMARY_DOMAIN 派生 HTTP(S) URL
 */
function deriveHttpUrl(): string | undefined {
  const domain = getPrimaryDomain();
  if (!domain) return undefined;
  const protocol = isHttpsEnabled() ? "https" : "http";
  return `${protocol}://${domain}`;
}

/**
 * 从 PRIMARY_DOMAIN 派生 WS(S) URL
 */
function deriveWsUrl(): string | undefined {
  const domain = getPrimaryDomain();
  if (!domain) return undefined;
  const protocol = isHttpsEnabled() ? "wss" : "ws";
  return `${protocol}://${domain}`;
}

// =============================================================================
// Public API
// =============================================================================

/**
 * 获取 API 基础 URL
 * - 本地开发：返回空字符串（使用相对路径）
 * - Docker/生产：返回完整 URL
 */
export function getApiBaseUrl(): string {
  // NEXT_PUBLIC_API_URL="" 表示使用相对路径
  // NEXT_PUBLIC_API_URL=undefined 表示未配置，尝试从 PRIMARY_DOMAIN 派生
  if (typeof process.env.NEXT_PUBLIC_API_URL === "string") {
    return process.env.NEXT_PUBLIC_API_URL;
  }

  // Try deriving from PRIMARY_DOMAIN
  const derived = deriveHttpUrl();
  if (derived) return derived;

  return "http://localhost:10000";
}

/**
 * 获取 OAuth 基础 URL（用于浏览器跳转）
 * OAuth 必须使用完整 URL，因为是浏览器直接跳转到后端
 */
export function getOAuthBaseUrl(): string {
  // Explicit configuration takes priority
  if (process.env.NEXT_PUBLIC_OAUTH_URL) {
    return process.env.NEXT_PUBLIC_OAUTH_URL;
  }
  if (process.env.NEXT_PUBLIC_API_URL) {
    return process.env.NEXT_PUBLIC_API_URL;
  }

  // Try deriving from PRIMARY_DOMAIN
  const derived = deriveHttpUrl();
  if (derived) return derived;

  return "http://localhost:10000";
}

/**
 * 获取 WebSocket 基础 URL
 * WebSocket 必须使用完整 URL，因为不能通过 Next.js rewrites 代理
 */
export function getWsBaseUrl(): string {
  // Explicit configuration takes priority
  if (process.env.NEXT_PUBLIC_WS_URL) {
    return process.env.NEXT_PUBLIC_WS_URL;
  }

  // Try deriving from PRIMARY_DOMAIN
  const derived = deriveWsUrl();
  if (derived) return derived;

  // Derive from API URL
  const apiUrl = process.env.NEXT_PUBLIC_API_URL;
  if (apiUrl) {
    return apiUrl.replace(/^http/, "ws");
  }

  // Client-side: derive from current page
  if (typeof window !== "undefined") {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const host = window.location.host;
    return `${protocol}//${host}`;
  }

  return "ws://localhost:10000";
}

// Default server URL for SSR and production
const DEFAULT_SERVER_URL = "https://api.agentsmesh.ai";

/**
 * 获取服务器部署 URL（SSR-safe 版本）
 * 返回在服务端和客户端初始渲染时相同的值，避免 hydration mismatch
 *
 * @returns 服务器 URL（基于环境变量配置）
 */
export function getServerUrlSSR(): string {
  // 使用环境变量或默认值（服务端和客户端一致）
  if (process.env.NEXT_PUBLIC_API_URL) {
    return process.env.NEXT_PUBLIC_API_URL;
  }
  return DEFAULT_SERVER_URL;
}

/**
 * 获取服务器部署 URL（用于 Runner 注册等外部访问）
 * - 客户端：使用当前页面的 origin
 * - 服务端：使用 NEXT_PUBLIC_API_URL 或默认值
 *
 * ⚠️ 注意：此函数在 SSR 组件中使用会导致 hydration mismatch
 * 对于 SSR 组件，请使用 getServerUrlSSR() 获取初始值，
 * 然后在 useEffect 中调用 getServerUrl() 更新
 *
 * @returns 完整的服务器 URL（如 https://api.agentsmesh.ai）
 */
export function getServerUrl(): string {
  // 客户端：使用当前页面的 origin
  if (typeof window !== "undefined") {
    return window.location.origin;
  }

  // 服务端：使用环境变量或默认值
  return getServerUrlSSR();
}

