"use client";

import React, { memo } from "react";
import type { UIField } from "@/lib/api/runner";

/**
 * Props for field renderer components
 */
export interface FieldRendererProps {
  fieldKey: string;
  field: UIField;
  value: unknown;
  onChange: (value: unknown) => void;
}

/**
 * Type for field renderer components
 */
export type FieldRenderer = React.FC<FieldRendererProps>;

/**
 * Boolean field renderer (checkbox)
 * Memoized to prevent unnecessary re-renders
 */
const BooleanFieldRenderer: FieldRenderer = memo(function BooleanFieldRenderer({
  fieldKey,
  field,
  value,
  onChange,
}: FieldRendererProps) {
  return (
    <div className="flex items-center gap-2">
      <input
        type="checkbox"
        id={fieldKey}
        checked={Boolean(value)}
        onChange={(e) => onChange(e.target.checked)}
        className="h-4 w-4 rounded border-border"
        aria-describedby={field.description ? `${fieldKey}-desc` : undefined}
      />
      <label htmlFor={fieldKey} className="text-sm">
        {field.label}
      </label>
      {field.description && (
        <span id={`${fieldKey}-desc`} className="text-xs text-muted-foreground ml-auto">
          {field.description}
        </span>
      )}
    </div>
  );
});

/**
 * String field renderer (text input)
 * Memoized to prevent unnecessary re-renders
 */
const StringFieldRenderer: FieldRenderer = memo(function StringFieldRenderer({
  fieldKey,
  field,
  value,
  onChange,
}: FieldRendererProps) {
  return (
    <div>
      <label htmlFor={fieldKey} className="block text-sm font-medium mb-1">
        {field.label}
        {field.required && <span className="text-destructive ml-1">*</span>}
      </label>
      <input
        type="text"
        id={fieldKey}
        value={String(value ?? "")}
        placeholder={field.placeholder}
        onChange={(e) => onChange(e.target.value)}
        className="w-full px-3 py-2 text-sm border border-border rounded-md bg-background"
        aria-describedby={field.description ? `${fieldKey}-desc` : undefined}
        aria-required={field.required}
      />
      {field.description && (
        <p id={`${fieldKey}-desc`} className="text-xs text-muted-foreground mt-1">
          {field.description}
        </p>
      )}
    </div>
  );
});

/**
 * Secret field renderer (password input)
 * Memoized to prevent unnecessary re-renders
 */
const SecretFieldRenderer: FieldRenderer = memo(function SecretFieldRenderer({
  fieldKey,
  field,
  value,
  onChange,
}: FieldRendererProps) {
  return (
    <div>
      <label htmlFor={fieldKey} className="block text-sm font-medium mb-1">
        {field.label}
        {field.required && <span className="text-destructive ml-1">*</span>}
      </label>
      <input
        type="password"
        id={fieldKey}
        value={String(value ?? "")}
        placeholder={field.placeholder}
        onChange={(e) => onChange(e.target.value)}
        className="w-full px-3 py-2 text-sm border border-border rounded-md bg-background"
        aria-describedby={field.description ? `${fieldKey}-desc` : undefined}
        aria-required={field.required}
      />
      {field.description && (
        <p id={`${fieldKey}-desc`} className="text-xs text-muted-foreground mt-1">
          {field.description}
        </p>
      )}
    </div>
  );
});

/**
 * Number field renderer
 * Memoized to prevent unnecessary re-renders
 */
const NumberFieldRenderer: FieldRenderer = memo(function NumberFieldRenderer({
  fieldKey,
  field,
  value,
  onChange,
}: FieldRendererProps) {
  return (
    <div>
      <label htmlFor={fieldKey} className="block text-sm font-medium mb-1">
        {field.label}
        {field.required && <span className="text-destructive ml-1">*</span>}
      </label>
      <input
        type="number"
        id={fieldKey}
        value={value != null ? Number(value) : ""}
        min={field.min}
        max={field.max}
        onChange={(e) => onChange(e.target.value ? Number(e.target.value) : null)}
        className="w-full px-3 py-2 text-sm border border-border rounded-md bg-background"
        aria-describedby={field.description ? `${fieldKey}-desc` : undefined}
        aria-required={field.required}
      />
      {field.description && (
        <p id={`${fieldKey}-desc`} className="text-xs text-muted-foreground mt-1">
          {field.description}
        </p>
      )}
    </div>
  );
});

/**
 * Select field renderer (dropdown)
 * Memoized to prevent unnecessary re-renders
 */
const SelectFieldRenderer: FieldRenderer = memo(function SelectFieldRenderer({
  fieldKey,
  field,
  value,
  onChange,
}: FieldRendererProps) {
  return (
    <div>
      <label htmlFor={fieldKey} className="block text-sm font-medium mb-1">
        {field.label}
        {field.required && <span className="text-destructive ml-1">*</span>}
      </label>
      <select
        id={fieldKey}
        value={String(value ?? "")}
        onChange={(e) => onChange(e.target.value)}
        className="w-full px-3 py-2 text-sm border border-border rounded-md bg-background"
        aria-describedby={field.description ? `${fieldKey}-desc` : undefined}
        aria-required={field.required}
      >
        {!field.required && !value && (
          <option value="" disabled>
            Select {field.label.toLowerCase()}...
          </option>
        )}
        {field.options?.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
      {field.description && (
        <p id={`${fieldKey}-desc`} className="text-xs text-muted-foreground mt-1">
          {field.description}
        </p>
      )}
    </div>
  );
});

/**
 * Fallback renderer for unknown field types
 */
const UnknownFieldRenderer: FieldRenderer = memo(function UnknownFieldRenderer({
  fieldKey,
  field,
}: FieldRendererProps) {
  return (
    <div className="text-sm text-muted-foreground">
      Unknown field type: {field.type} ({fieldKey})
    </div>
  );
});

/**
 * Field renderer registry - maps field types to their renderer components
 * This is an immutable registry. Use createFieldRendererRegistry() for custom registries.
 */
const FIELD_RENDERERS: Readonly<Record<string, FieldRenderer>> = {
  boolean: BooleanFieldRenderer,
  string: StringFieldRenderer,
  secret: SecretFieldRenderer,
  number: NumberFieldRenderer,
  select: SelectFieldRenderer,
};

/**
 * Get the appropriate renderer for a field type
 * Returns a fallback renderer if the type is unknown
 */
export function getFieldRenderer(type: string): FieldRenderer {
  return FIELD_RENDERERS[type] || UnknownFieldRenderer;
}

/**
 * Create a custom field renderer registry with additional field types
 * This allows extending the default renderers without modifying global state.
 *
 * @example
 * const customRegistry = createFieldRendererRegistry({
 *   textarea: MyTextareaRenderer,
 * });
 * const renderer = customRegistry.get('textarea');
 */
export function createFieldRendererRegistry(
  customRenderers: Record<string, FieldRenderer> = {}
): { get: (type: string) => FieldRenderer } {
  const registry = { ...FIELD_RENDERERS, ...customRenderers };
  return {
    get: (type: string) => registry[type] || UnknownFieldRenderer,
  };
}
