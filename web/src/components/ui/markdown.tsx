"use client";

import ReactMarkdown from "react-markdown";
import { cn } from "@/lib/utils";

interface MarkdownProps {
  content: string;
  className?: string;
  /** Compact mode with smaller text for embedded use */
  compact?: boolean;
}

/**
 * Markdown renderer component using react-markdown
 */
export function Markdown({ content, className, compact = false }: MarkdownProps) {
  return (
    <div
      className={cn(
        "prose prose-neutral dark:prose-invert max-w-none",
        compact && "prose-sm",
        // Override prose defaults for compact mode
        compact && "[&_p]:my-1 [&_ul]:my-1 [&_ol]:my-1 [&_li]:my-0.5 [&_h1]:text-base [&_h2]:text-sm [&_h3]:text-xs",
        className
      )}
    >
      <ReactMarkdown>{content}</ReactMarkdown>
    </div>
  );
}

export default Markdown;
