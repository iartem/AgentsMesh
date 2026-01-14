import { useState, useEffect, useCallback } from "react";
import { runnerApi, agentApi, PluginCapability } from "@/lib/api/client";

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
 *
 * Configuration priority (high to low):
 * 1. User overrides in the form
 * 2. Organization default config
 * 3. Runner Plugin defaults
 */
export function usePluginOptions(
  runnerId: number | null,
  agentSlug: string,
  agentTypeId?: number | null
): PluginOptionsState {
  const [plugins, setPlugins] = useState<PluginCapability[]>([]);
  const [loading, setLoading] = useState(false);
  const [config, setConfig] = useState<Record<string, unknown>>({});

  // Load plugin options when runner or agent changes
  useEffect(() => {
    let cancelled = false;

    console.log("[usePluginOptions] runnerId:", runnerId, "agentSlug:", agentSlug, "agentTypeId:", agentTypeId);

    const loadOptions = async () => {
      if (!runnerId || !agentSlug) {
        console.log("[usePluginOptions] Skipping - missing runnerId or agentSlug");
        setPlugins([]);
        setConfig({});
        return;
      }

      setLoading(true);
      try {
        // Load plugin options from runner
        const response = await runnerApi.getPluginOptions(runnerId, agentSlug);

        if (cancelled) return;

        setPlugins(response.plugins || []);

        // Step 1: Initialize config with Plugin defaults
        const mergedConfig: Record<string, unknown> = {};
        for (const plugin of response.plugins || []) {
          for (const field of plugin.ui?.fields || []) {
            if (field.default !== undefined) {
              mergedConfig[`${plugin.name}.${field.name}`] = field.default;
            }
          }
        }

        // Step 2: Load organization default config and merge (higher priority)
        if (agentTypeId) {
          try {
            const orgConfigResponse = await agentApi.getDefaultConfig(agentTypeId);
            if (!cancelled && orgConfigResponse.config?.config_values) {
              const orgConfig = orgConfigResponse.config.config_values;
              console.log("[usePluginOptions] Organization default config:", orgConfig);
              console.log("[usePluginOptions] Available plugins:", response.plugins?.map(p => p.name));

              // Merge organization config into mergedConfig
              // Organization config uses simple field names, need to map to plugin.field format
              for (const plugin of response.plugins || []) {
                console.log("[usePluginOptions] Processing plugin:", plugin.name, "fields:", plugin.ui?.fields?.map(f => f.name));
                for (const field of plugin.ui?.fields || []) {
                  // Check if organization config has this field
                  if (orgConfig[field.name] !== undefined) {
                    const key = `${plugin.name}.${field.name}`;
                    console.log("[usePluginOptions] Merging org config:", field.name, "->", key, "=", orgConfig[field.name]);
                    mergedConfig[key] = orgConfig[field.name];
                  }
                }
              }
              console.log("[usePluginOptions] Final merged config:", mergedConfig);
            }
          } catch (err) {
            // Organization config not found or error - use Plugin defaults only
            console.log("[usePluginOptions] No organization default config found, using Plugin defaults", err);
          }
        }

        if (!cancelled) {
          setConfig(mergedConfig);
        }
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
  }, [runnerId, agentSlug, agentTypeId]);

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
