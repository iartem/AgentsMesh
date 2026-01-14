"use client";

import React, { memo, useCallback } from "react";
import type { ConfigField } from "@/lib/api/agent";
import { FieldRenderer, getFieldRenderer } from "./field-renderers";

interface ConfigFormProps {
  fields: ConfigField[];
  values: Record<string, unknown>;
  onChange: (fieldName: string, value: unknown) => void;
}

interface FieldWrapperProps {
  field: ConfigField;
  value: unknown;
  onChange: (fieldName: string, value: unknown) => void;
}

/**
 * Wrapper component for individual fields
 * Handles the field key generation and change propagation
 */
const FieldWrapper = memo(function FieldWrapper({
  field,
  value,
  onChange,
}: FieldWrapperProps) {
  const fieldKey = field.name;
  const Renderer = getFieldRenderer(field.type);

  const handleChange = useCallback(
    (newValue: unknown) => {
      onChange(field.name, newValue);
    },
    [field.name, onChange]
  );

  return (
    <Renderer
      fieldKey={fieldKey}
      field={field}
      value={value}
      onChange={handleChange}
    />
  );
});

/**
 * Dynamic form renderer for agent configuration.
 * Uses strategy pattern for field rendering - each field type has its own renderer.
 *
 * To add a new field type:
 * 1. Create a new renderer component in field-renderers.tsx
 * 2. Register it in the FIELD_RENDERERS map
 */
export const ConfigForm = memo(function ConfigForm({
  fields,
  values,
  onChange,
}: ConfigFormProps) {
  if (!fields || fields.length === 0) {
    return null;
  }

  return (
    <div className="space-y-4">
      <div className="border border-border rounded-md p-3">
        <div className="space-y-3">
          {fields.map((field) => {
            const currentValue = values[field.name] ?? field.default;

            return (
              <FieldWrapper
                key={field.name}
                field={field}
                value={currentValue}
                onChange={onChange}
              />
            );
          })}
        </div>
      </div>
    </div>
  );
});

// Re-export types for external use
export type { FieldRenderer } from "./field-renderers";

export default ConfigForm;
