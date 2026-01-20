import { useState, useEffect } from "react";
import { getServerUrl, getServerUrlSSR } from "@/lib/env";

/**
 * Hook to get server URL in a SSR-safe way.
 *
 * Returns SSR-safe URL initially to avoid hydration mismatch,
 * then updates to actual client URL after hydration.
 */
export function useServerUrl(): string {
  const [serverUrl, setServerUrl] = useState(getServerUrlSSR);

  useEffect(() => {
    // Update to actual client URL after hydration
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setServerUrl(getServerUrl());
  }, []);

  return serverUrl;
}
