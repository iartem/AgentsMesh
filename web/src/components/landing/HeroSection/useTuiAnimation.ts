"use client";

import { useState, useEffect, useMemo } from "react";
import { getTuiFrames } from "./tuiFrames";

/**
 * Custom hook for managing TUI animation state
 */
export function useTuiAnimation(t: (key: string) => string) {
  const [currentFrameIndex, setCurrentFrameIndex] = useState(0);
  const [displayedLines, setDisplayedLines] = useState<number>(0);

  // Memoize the translated frames
  const tuiFrames = useMemo(() => getTuiFrames(t), [t]);

  // Cycle through frames
  useEffect(() => {
    const frame = tuiFrames[currentFrameIndex];
    const nextFrame = tuiFrames[currentFrameIndex + 1];

    if (nextFrame) {
      const delay = nextFrame.time - frame.time;
      const timer = setTimeout(() => {
        setCurrentFrameIndex(prev => prev + 1);
        setDisplayedLines(0);
      }, delay);
      return () => clearTimeout(timer);
    } else {
      // Reset after last frame
      const timer = setTimeout(() => {
        setCurrentFrameIndex(0);
        setDisplayedLines(0);
      }, 4000);
      return () => clearTimeout(timer);
    }
  }, [currentFrameIndex, tuiFrames]);

  // Animate lines appearing
  useEffect(() => {
    const frame = tuiFrames[currentFrameIndex];
    const totalLines = frame.content.mainContent.length;

    if (displayedLines < totalLines) {
      const timer = setTimeout(() => {
        setDisplayedLines(prev => prev + 1);
      }, 150);
      return () => clearTimeout(timer);
    }
  }, [currentFrameIndex, displayedLines, tuiFrames]);

  const currentFrame = tuiFrames[currentFrameIndex];

  return {
    currentFrame,
    currentFrameIndex,
    displayedLines,
    isTyping: displayedLines < currentFrame.content.mainContent.length,
  };
}

/**
 * Get CSS class for a line type
 */
export function getLineStyle(type: string): string {
  switch (type) {
    case "user": return "text-blue-500 dark:text-blue-400";
    case "assistant": return "text-foreground";
    case "system": return "text-muted-foreground";
    case "tool": return "text-yellow-500 dark:text-yellow-400";
    case "success": return "text-green-500 dark:text-green-400";
    case "observe-header": return "text-primary font-bold";
    case "observe": return "text-cyan-500 dark:text-cyan-300";
    case "message-sent": return "text-purple-500 dark:text-purple-400";
    default: return "text-muted-foreground";
  }
}
