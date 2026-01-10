"use client";

import { useEffect, useRef, useCallback, useState, forwardRef, useImperativeHandle } from "react";
import { Terminal as XTerm } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { WebLinksAddon } from "@xterm/addon-web-links";
import { SearchAddon } from "@xterm/addon-search";
import "@xterm/xterm/css/xterm.css";

interface TerminalProps {
  podKey: string;
  onData?: (data: string) => void;
  onResize?: (rows: number, cols: number) => void;
  className?: string;
}

export interface TerminalHandle {
  write: (data: string | Uint8Array) => void;
  clear: () => void;
  focus: () => void;
  fit: () => void;
  isReady: boolean;
}

export const Terminal = forwardRef<TerminalHandle, TerminalProps>(function Terminal(
  { podKey, onData, onResize, className },
  ref
) {
  const terminalRef = useRef<HTMLDivElement>(null);
  const xtermRef = useRef<XTerm | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const [isReady, setIsReady] = useState(false);

  // Initialize terminal
  useEffect(() => {
    if (!terminalRef.current || xtermRef.current) return;

    const term = new XTerm({
      cursorBlink: true,
      cursorStyle: "block",
      fontFamily: 'Menlo, Monaco, "Courier New", monospace',
      fontSize: 14,
      lineHeight: 1.2,
      theme: {
        background: "#1e1e1e",
        foreground: "#d4d4d4",
        cursor: "#d4d4d4",
        cursorAccent: "#1e1e1e",
        selectionBackground: "#264f78",
        black: "#000000",
        red: "#cd3131",
        green: "#0dbc79",
        yellow: "#e5e510",
        blue: "#2472c8",
        magenta: "#bc3fbc",
        cyan: "#11a8cd",
        white: "#e5e5e5",
        brightBlack: "#666666",
        brightRed: "#f14c4c",
        brightGreen: "#23d18b",
        brightYellow: "#f5f543",
        brightBlue: "#3b8eea",
        brightMagenta: "#d670d6",
        brightCyan: "#29b8db",
        brightWhite: "#e5e5e5",
      },
      allowProposedApi: true,
    });

    // Add addons
    const fitAddon = new FitAddon();
    const webLinksAddon = new WebLinksAddon();
    const searchAddon = new SearchAddon();

    term.loadAddon(fitAddon);
    term.loadAddon(webLinksAddon);
    term.loadAddon(searchAddon);

    // Open terminal
    term.open(terminalRef.current);
    fitAddon.fit();

    // Handle input
    term.onData((data) => {
      onData?.(data);
    });

    // Handle resize
    term.onResize(({ rows, cols }) => {
      onResize?.(rows, cols);
    });

    xtermRef.current = term;
    fitAddonRef.current = fitAddon;
    setIsReady(true);

    // Cleanup
    return () => {
      term.dispose();
      xtermRef.current = null;
      fitAddonRef.current = null;
    };
  }, [onData, onResize]);

  // Handle window resize
  useEffect(() => {
    const handleResize = () => {
      if (fitAddonRef.current) {
        fitAddonRef.current.fit();
      }
    };

    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, []);

  // Public method to write data to terminal
  const write = useCallback((data: string | Uint8Array) => {
    if (xtermRef.current) {
      xtermRef.current.write(data);
    }
  }, []);

  // Public method to clear terminal
  const clear = useCallback(() => {
    if (xtermRef.current) {
      xtermRef.current.clear();
    }
  }, []);

  // Public method to focus terminal
  const focus = useCallback(() => {
    if (xtermRef.current) {
      xtermRef.current.focus();
    }
  }, []);

  // Public method to fit terminal
  const fit = useCallback(() => {
    if (fitAddonRef.current) {
      fitAddonRef.current.fit();
    }
  }, []);

  // Expose methods via ref using useImperativeHandle
  useImperativeHandle(ref, () => ({
    write,
    clear,
    focus,
    fit,
    isReady,
  }), [write, clear, focus, fit, isReady]);

  return (
    <div
      ref={terminalRef}
      className={`w-full h-full min-h-[300px] bg-[#1e1e1e] ${className || ""}`}
      data-pod-key={podKey}
    />
  );
});

// Hook for terminal with WebSocket connection
export function useTerminal(podKey: string) {
  const terminalRef = useRef<TerminalHandle | null>(null);
  const wsRef = useRef<WebSocket | null>(null);

  const connect = useCallback(() => {
    if (!podKey) return;

    const wsUrl = process.env.NEXT_PUBLIC_WS_URL || "ws://localhost:8080";
    // Get token and org from Zustand persisted store
    let token = null;
    let orgSlug = null;
    try {
      const authData = localStorage.getItem("agentmesh-auth");
      if (authData) {
        const parsed = JSON.parse(authData);
        token = parsed.state?.token;
        orgSlug = parsed.state?.currentOrg?.slug;
      }
    } catch (e) {
      console.error("Failed to parse auth data:", e);
    }
    const ws = new WebSocket(`${wsUrl}/api/v1/orgs/${orgSlug}/ws/terminal/${podKey}?token=${token}`);

    ws.binaryType = "arraybuffer";

    ws.onopen = () => {
      console.log("Terminal WebSocket connected");
    };

    ws.onmessage = (event) => {
      const terminal = terminalRef.current;
      if (terminal && event.data) {
        if (event.data instanceof ArrayBuffer) {
          terminal.write(new Uint8Array(event.data));
        } else {
          terminal.write(event.data);
        }
      }
    };

    ws.onerror = (error) => {
      console.error("Terminal WebSocket error:", error);
    };

    ws.onclose = () => {
      console.log("Terminal WebSocket disconnected");
    };

    wsRef.current = ws;
  }, [podKey]);

  const disconnect = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
  }, []);

  const sendInput = useCallback((data: string) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: "input", data }));
    }
  }, []);

  const sendResize = useCallback((rows: number, cols: number) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: "resize", rows, cols }));
    }
  }, []);

  useEffect(() => {
    return () => {
      disconnect();
    };
  }, [disconnect]);

  return {
    terminalRef,
    connect,
    disconnect,
    sendInput,
    sendResize,
  };
}

export default Terminal;
