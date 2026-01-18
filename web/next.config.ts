import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  allowedDevOrigins: ["192.168.100.189"],

  // 本地开发时代理 API 请求，避免跨域问题
  // API_PROXY_TARGET 由 dev.sh 生成到 .env.local
  // 前端使用相对路径 /api/*，Next.js rewrites 代理到后端
  async rewrites() {
    // API_PROXY_TARGET 是服务端变量（不带 NEXT_PUBLIC_ 前缀）
    const proxyTarget = process.env.API_PROXY_TARGET;

    // 仅在本地开发且配置了代理目标时启用
    if (process.env.NODE_ENV === "development" && proxyTarget) {
      console.log(`[Next.js] API proxy enabled: /api/* → ${proxyTarget}/api/*`);
      return [
        {
          source: "/api/:path*",
          destination: `${proxyTarget}/api/:path*`,
        },
        {
          source: "/health",
          destination: `${proxyTarget}/health`,
        },
      ];
    }

    return [];
  },
};

export default nextConfig;
