import { useState, useEffect, useCallback } from "react";
import { runnerApi, PluginCapability } from "@/lib/api/client";

export interface PluginOptionsState {
  plugins: PluginCapability[];
  loading: boolean;
  config: Record<string, unknown>;
  updateConfig: (pluginName: string, fieldName: string, value: unknown) => void;
  resetConfig: () => void;
}

/**
 * Hook to manage plugin options and configuration
 * Loads plugin options when runner and agent are selected
 */
export function usePluginOptions(
  runnerId: number | null,
  agentSlug: string
): PluginOptionsState {
  const [plugins, setPlugins] = useState<PluginCapability[]>([]);
  const [loading, setLoading] = useState(false);
  const [config, setConfig] = useState<Record<string, unknown>>({});

  // Load plugin options when runner or agent changes
  useEffect(() => {
    let cancelled = false;

    console.log("[usePluginOptions] runnerId:", runnerId, "agentSlug:", agentSlug);

    const loadOptions = async () => {
      if (!runnerId || !agentSlug) {
        console.log("[usePluginOptions] Skipping - missing runnerId or agentSlug");
        setPlugins([]);
        setConfig({});
        return;
      }

      setLoading(true);
      try {
        const response = await runnerApi.getPluginOptions(runnerId, agentSlug);

        if (cancelled) return;

        setPlugins(response.plugins || []);

        // Initialize config with defaults
        const defaults: Record<string, unknown> = {};
        for (const plugin of response.plugins || []) {
          for (const field of plugin.ui?.fields || []) {
            if (field.default !== undefined) {
              defaults[`${plugin.name}.${field.name}`] = field.default;
            }
          }
        }
        setConfig(defaults);
      } catch (err) {
        if (cancelled) return;
        console.error("Failed to load plugin options:", err);
        setPlugins([]);
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    loadOptions();

    return () => {
      cancelled = true;
    };
  }, [runnerId, agentSlug]);

  // Update a single config field
  const updateConfig = useCallback(
    (pluginName: string, fieldName: string, value: unknown) => {
      setConfig((prev) => ({
        ...prev,
        [`${pluginName}.${fieldName}`]: value,
      }));
    },
    []
  );

  // Reset config to empty
  const resetConfig = useCallback(() => {
    setConfig({});
    setPlugins([]);
  }, []);

  return { plugins, loading, config, updateConfig, resetConfig };
}
