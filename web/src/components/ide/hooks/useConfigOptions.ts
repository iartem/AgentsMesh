import { useState, useEffect, useCallback } from "react";
import { agentApi, ConfigField } from "@/lib/api/client";

export interface ConfigOptionsState {
  fields: ConfigField[];
  loading: boolean;
  config: Record<string, unknown>;
  updateConfig: (fieldName: string, value: unknown) => void;
  resetConfig: () => void;
}

/**
 * Hook to manage agent config options and configuration
 * Loads config schema from Backend when agent type is selected
 *
 * Configuration priority (high to low):
 * 1. User overrides in the form
 * 2. Organization default config
 * 3. Backend ConfigSchema defaults
 */
export function useConfigOptions(
  runnerId: number | null,
  agentSlug: string,
  agentTypeId?: number | null
): ConfigOptionsState {
  const [fields, setFields] = useState<ConfigField[]>([]);
  const [loading, setLoading] = useState(false);
  const [config, setConfig] = useState<Record<string, unknown>>({});

  // Load config schema when agent type changes
  useEffect(() => {
    let cancelled = false;

    console.log("[useConfigOptions] agentTypeId:", agentTypeId, "agentSlug:", agentSlug);

    const loadOptions = async () => {
      if (!agentTypeId) {
        console.log("[useConfigOptions] Skipping - missing agentTypeId");
        setFields([]);
        setConfig({});
        return;
      }

      setLoading(true);
      try {
        // Load config schema from Backend
        const schemaResponse = await agentApi.getConfigSchema(agentTypeId);

        if (cancelled) return;

        const schema = schemaResponse.schema || { fields: [] };
        setFields(schema.fields || []);

        // Step 1: Initialize config with ConfigSchema defaults
        const mergedConfig: Record<string, unknown> = {};
        for (const field of schema.fields || []) {
          if (field.default !== undefined) {
            mergedConfig[field.name] = field.default;
          }
        }

        // Step 2: Load organization default config and merge (higher priority)
        try {
          const orgConfigResponse = await agentApi.getDefaultConfig(agentTypeId);
          if (!cancelled && orgConfigResponse.config?.config_values) {
            const orgConfig = orgConfigResponse.config.config_values;
            console.log("[useConfigOptions] Organization default config:", orgConfig);

            // Merge organization config into mergedConfig
            for (const field of schema.fields || []) {
              if (orgConfig[field.name] !== undefined) {
                console.log("[useConfigOptions] Merging org config:", field.name, "=", orgConfig[field.name]);
                mergedConfig[field.name] = orgConfig[field.name];
              }
            }
            console.log("[useConfigOptions] Final merged config:", mergedConfig);
          }
        } catch (err) {
          // Organization config not found or error - use ConfigSchema defaults only
          console.log("[useConfigOptions] No organization default config found, using ConfigSchema defaults", err);
        }

        if (!cancelled) {
          setConfig(mergedConfig);
        }
      } catch (err) {
        if (cancelled) return;
        console.error("Failed to load config schema:", err);
        setFields([]);
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
  }, [agentTypeId, agentSlug]);

  // Update a single config field
  const updateConfig = useCallback(
    (fieldName: string, value: unknown) => {
      setConfig((prev) => ({
        ...prev,
        [fieldName]: value,
      }));
    },
    []
  );

  // Reset config to empty
  const resetConfig = useCallback(() => {
    setConfig({});
    setFields([]);
  }, []);

  return { fields, loading, config, updateConfig, resetConfig };
}
