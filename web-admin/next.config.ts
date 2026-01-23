import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",

  // =============================================================================
  // Unified Domain Configuration
  // 将 PRIMARY_DOMAIN / USE_HTTPS 映射为 NEXT_PUBLIC_* 变量
  // 这样配置文件中可以统一使用 PRIMARY_DOMAIN，与 Backend/Relay 保持一致
  // =============================================================================
  env: {
    NEXT_PUBLIC_PRIMARY_DOMAIN: process.env.PRIMARY_DOMAIN || "",
    NEXT_PUBLIC_USE_HTTPS: process.env.USE_HTTPS || "false",
  },

  // Allow images from any source during development
  images: {
    remotePatterns: [
      {
        protocol: "https",
        hostname: "**",
      },
      {
        protocol: "http",
        hostname: "**",
      },
    ],
  },
};

export default nextConfig;
