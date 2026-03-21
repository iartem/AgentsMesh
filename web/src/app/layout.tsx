// AgentsMesh Web Frontend
// Build version marker: 2025-01-20-ci-test
import type { Metadata, Viewport } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import { ThemeProvider, ThemeColorMeta } from "@/components/theme";
import { PWAProvider } from "@/components/pwa";
import { PostHogProvider } from "@/providers/PostHogProvider";
import { NextIntlClientProvider } from "next-intl";
import { getLocale, getMessages } from "next-intl/server";
import { Toaster } from "sonner";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  metadataBase: new URL("https://agentsmesh.ai"),
  title: {
    default: "AgentsMesh - AI Agent Fleet Command Center",
    template: "%s | AgentsMesh",
  },
  description: "Don't let humans bottleneck AI agents. Agent-centered development starts here. Orchestrate fleets of Claude Code, Codex, and more — each in its own isolated workspace, collaborating autonomously.",
  keywords: [
    "agentsmesh", "agentmesh", "agents mesh",
    "agent fleet command center", "agent swarm", "agent orchestration",
    "harness engineering", "AI agent platform",
    "AI agents", "AI coding", "Claude Code", "Codex CLI", "Gemini CLI", "Aider",
    "multi-agent collaboration", "agent coordination", "terminal AI", "code automation",
    "developer tools", "enterprise development", "self-hosted", "agent fleet",
    "vibe coding", "AI developer tools", "coding agents", "agent management",
    "multi-agent orchestration", "AI swarm", "agent fleet management",
  ],
  manifest: "/manifest.json",
  appleWebApp: {
    capable: true,
    statusBarStyle: "default",
    title: "AgentsMesh",
  },
  formatDetection: {
    telephone: false,
  },
  openGraph: {
    type: "website",
    siteName: "AgentsMesh",
    title: "AgentsMesh - AI Agent Fleet Command Center",
    description: "Don't let humans bottleneck AI agents. Agent-centered development starts here. Orchestrate fleets of Claude Code, Codex, and more — each in its own isolated workspace, collaborating autonomously.",
    url: "https://agentsmesh.ai",
  },
  twitter: {
    card: "summary_large_image",
    title: "AgentsMesh - AI Agent Fleet Command Center",
    description: "Don't let humans bottleneck AI agents. Agent-centered development starts here. Orchestrate Claude Code, Codex CLI, Gemini CLI, Aider and more.",
  },
  alternates: {
    canonical: "https://agentsmesh.ai",
  },
};

export const viewport: Viewport = {
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#ffffff" },
    { media: "(prefers-color-scheme: dark)", color: "#0a0a0a" },
  ],
  width: "device-width",
  initialScale: 1,
  maximumScale: 1,
  userScalable: false,
};

export default async function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const locale = await getLocale();
  const messages = await getMessages();

  return (
    <html lang={locale} suppressHydrationWarning>
      <body
        className={`${geistSans.variable} ${geistMono.variable} antialiased bg-background text-foreground`}
      >
        <ThemeProvider
          attribute="class"
          defaultTheme="system"
          enableSystem
          disableTransitionOnChange
          themes={["light", "dark", "solarized-light", "solarized-dark"]}
        >
          <PostHogProvider>
            <NextIntlClientProvider locale={locale} messages={messages}>
              <PWAProvider>
                {children}
              </PWAProvider>
            </NextIntlClientProvider>
          </PostHogProvider>
          <ThemeColorMeta />
          <Toaster richColors position="top-right" />
        </ThemeProvider>
      </body>
    </html>
  );
}
