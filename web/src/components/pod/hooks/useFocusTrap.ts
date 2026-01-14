import { useEffect, useRef, RefObject } from "react";

const FOCUSABLE_SELECTOR =
  'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])';

/**
 * Hook to implement focus trap for modal dialogs
 * - Traps Tab navigation within the container
 * - Handles Escape key to close
 * - Restores focus when closed
 *
 * @param enabled Whether the focus trap is active
 * @param onEscape Callback when Escape is pressed
 * @returns Ref to attach to the container element
 */
export function useFocusTrap<T extends HTMLElement>(
  enabled: boolean,
  onEscape: () => void
): RefObject<T | null> {
  const containerRef = useRef<T | null>(null);
  const previousActiveElement = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (!enabled) return;

    // Save currently focused element to restore later
    previousActiveElement.current = document.activeElement as HTMLElement;

    const container = containerRef.current;

    // Focus the first focusable element after a short delay
    const focusTimer = setTimeout(() => {
      if (container) {
        const firstFocusable = container.querySelector<HTMLElement>(FOCUSABLE_SELECTOR);
        firstFocusable?.focus();
      }
    }, 10);

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        onEscape();
        return;
      }

      // Focus trap: keep Tab navigation within container
      if (e.key === "Tab" && container) {
        const focusableElements = container.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR);
        if (focusableElements.length === 0) return;

        const firstElement = focusableElements[0];
        const lastElement = focusableElements[focusableElements.length - 1];

        if (e.shiftKey) {
          // Shift+Tab: if on first element, move to last
          if (document.activeElement === firstElement) {
            e.preventDefault();
            lastElement?.focus();
          }
        } else {
          // Tab: if on last element, move to first
          if (document.activeElement === lastElement) {
            e.preventDefault();
            firstElement?.focus();
          }
        }
      }
    };

    document.addEventListener("keydown", handleKeyDown);

    return () => {
      clearTimeout(focusTimer);
      document.removeEventListener("keydown", handleKeyDown);
      // Restore focus to previously focused element
      previousActiveElement.current?.focus();
    };
  }, [enabled, onEscape]);

  return containerRef;
}
