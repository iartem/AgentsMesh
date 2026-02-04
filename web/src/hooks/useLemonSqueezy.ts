"use client";

import { useEffect, useCallback, useRef, useState } from "react";

// LemonSqueezy global types
declare global {
  interface Window {
    createLemonSqueezy?: () => void;
    LemonSqueezy?: {
      Setup: (config: { eventHandler: (event: LemonSqueezyEvent) => void }) => void;
      Url: {
        Open: (url: string) => void;
        Close: () => void;
      };
      Refresh: () => void;
    };
  }
}

// LemonSqueezy event types
export interface LemonSqueezyEvent {
  event:
    | "Checkout.Success"
    | "Checkout.ViewCart"
    | "Checkout.PaymentMethodUpdate"
    | "Checkout.Close"
    | "GA.ViewItem"
    | "PaymentMethodUpdate.Mounted"
    | "PaymentMethodUpdate.Closed"
    | "PaymentMethodUpdate.Updated";
  data?: {
    order?: {
      id: string;
      order_number: string;
      status: string;
    };
    checkout?: {
      id: string;
    };
  };
}

export interface UseLemonSqueezyOptions {
  onCheckoutSuccess?: (event: LemonSqueezyEvent) => void;
  onCheckoutClose?: (event: LemonSqueezyEvent) => void;
}

const LEMON_JS_URL = "https://app.lemonsqueezy.com/js/lemon.js";

/**
 * Hook to load and use LemonSqueezy overlay checkout
 *
 * @example
 * ```tsx
 * const { openCheckout, isLoaded } = useLemonSqueezy({
 *   onCheckoutSuccess: (event) => {
 *     console.log("Payment successful:", event.data?.order);
 *     router.refresh();
 *   },
 *   onCheckoutClose: () => {
 *     console.log("Checkout closed");
 *   },
 * });
 *
 * // Open overlay checkout
 * openCheckout("https://checkout.lemonsqueezy.com/...");
 * ```
 */
// Check if script is already loaded (outside of component for lazy init)
function checkScriptLoaded(): boolean {
  if (typeof window === "undefined") return false;
  return !!(window.LemonSqueezy || document.querySelector(`script[src="${LEMON_JS_URL}"]`));
}

export function useLemonSqueezy(options: UseLemonSqueezyOptions = {}) {
  const { onCheckoutSuccess, onCheckoutClose } = options;
  // Use lazy initialization to avoid effect setState issue
  const [isLoaded, setIsLoaded] = useState(checkScriptLoaded);
  const callbacksRef = useRef({ onCheckoutSuccess, onCheckoutClose });

  // Update callbacks ref when they change
  useEffect(() => {
    callbacksRef.current = { onCheckoutSuccess, onCheckoutClose };
  }, [onCheckoutSuccess, onCheckoutClose]);

  // Load Lemon.js script
  useEffect(() => {
    // Skip if already loaded (checked via state init)
    if (isLoaded) {
      return;
    }

    // Double-check in case state init missed it (SSR)
    // This is a valid synchronization pattern - we're syncing React state with external script state
    if (window.LemonSqueezy || document.querySelector(`script[src="${LEMON_JS_URL}"]`)) {
       
      setIsLoaded(true);
      return;
    }

    const script = document.createElement("script");
    script.src = LEMON_JS_URL;
    script.defer = true;
    script.onload = () => {
      // Initialize LemonSqueezy after script loads
      if (window.createLemonSqueezy) {
        window.createLemonSqueezy();
      }
      setIsLoaded(true);
    };
    script.onerror = () => {
      console.error("Failed to load LemonSqueezy script");
    };

    document.head.appendChild(script);

    return () => {
      // Clean up is not needed as the script should persist
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- isLoaded is intentionally not in deps
  }, []);

  // Setup event handler
  useEffect(() => {
    if (!window.LemonSqueezy) return;

    window.LemonSqueezy.Setup({
      eventHandler: (event: LemonSqueezyEvent) => {
        switch (event.event) {
          case "Checkout.Success":
            callbacksRef.current.onCheckoutSuccess?.(event);
            break;
          case "Checkout.Close":
            callbacksRef.current.onCheckoutClose?.(event);
            break;
        }
      },
    });
  }, []);

  // Open checkout overlay
  const openCheckout = useCallback((url: string) => {
    if (!window.LemonSqueezy) {
      // Fallback to redirect if LemonSqueezy is not loaded
      console.warn("LemonSqueezy not loaded, falling back to redirect");
      window.location.href = url;
      return;
    }

    // Setup event handler before opening
    window.LemonSqueezy.Setup({
      eventHandler: (event: LemonSqueezyEvent) => {
        switch (event.event) {
          case "Checkout.Success":
            callbacksRef.current.onCheckoutSuccess?.(event);
            break;
          case "Checkout.Close":
            callbacksRef.current.onCheckoutClose?.(event);
            break;
        }
      },
    });

    window.LemonSqueezy.Url.Open(url);
  }, []);

  // Close checkout overlay
  const closeCheckout = useCallback(() => {
    if (window.LemonSqueezy) {
      window.LemonSqueezy.Url.Close();
    }
  }, []);

  // Refresh checkout data (useful after updating customer info)
  const refreshCheckout = useCallback(() => {
    if (window.LemonSqueezy) {
      window.LemonSqueezy.Refresh();
    }
  }, []);

  return {
    openCheckout,
    closeCheckout,
    refreshCheckout,
    isLoaded,
  };
}
