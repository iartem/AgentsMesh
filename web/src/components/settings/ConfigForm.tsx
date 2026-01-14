"use client";

import React from "react";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";
import type { ConfigField } from "@/lib/api/agent";

interface ConfigFormProps {
  fields: ConfigField[];
  values: Record<string, unknown>;
  onChange: (fieldName: string, value: unknown) => void;
  className?: string;
  disabled?: boolean;
}

/**
 * Dynamic form component that renders ConfigField[] from Backend ConfigSchema.
 * Supports field types: boolean, string, select, number, secret
 */
export function ConfigForm({
  fields,
  values,
  onChange,
  className,
  disabled = false,
}: ConfigFormProps) {
  if (!fields || fields.length === 0) {
    return null;
  }

  return (
    <div className={cn("space-y-4", className)}>
      {fields.map((field) => (
        <ConfigFieldRenderer
          key={field.name}
          field={field}
          value={values[field.name]}
          onChange={(value) => onChange(field.name, value)}
          disabled={disabled}
        />
      ))}
    </div>
  );
}

interface ConfigFieldRendererProps {
  field: ConfigField;
  value: unknown;
  onChange: (value: unknown) => void;
  disabled?: boolean;
}

function ConfigFieldRenderer({
  field,
  value,
  onChange,
  disabled,
}: ConfigFieldRendererProps) {
  const id = `config-field-${field.name}`;

  // Get effective value (use default if no value set)
  const effectiveValue = value !== undefined ? value : field.default;

  const renderField = () => {
    switch (field.type) {
      case "boolean":
        return (
          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor={id} className="cursor-pointer">
                {field.label}
              </Label>
              {field.description && (
                <p className="text-xs text-muted-foreground">
                  {field.description}
                </p>
              )}
            </div>
            <Switch
              id={id}
              checked={Boolean(effectiveValue)}
              onCheckedChange={onChange}
              disabled={disabled}
            />
          </div>
        );

      case "select":
        return (
          <div className="space-y-2">
            <Label htmlFor={id}>{field.label}</Label>
            <Select
              value={String(effectiveValue ?? "")}
              onValueChange={onChange}
              disabled={disabled}
            >
              <SelectTrigger id={id}>
                <SelectValue placeholder={field.placeholder || "Select..."} />
              </SelectTrigger>
              <SelectContent>
                {field.options?.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {field.description && (
              <p className="text-xs text-muted-foreground">{field.description}</p>
            )}
          </div>
        );

      case "number":
        return (
          <div className="space-y-2">
            <Label htmlFor={id}>{field.label}</Label>
            <Input
              id={id}
              type="number"
              value={effectiveValue !== undefined ? String(effectiveValue) : ""}
              onChange={(e) => {
                const val = e.target.value;
                onChange(val === "" ? undefined : Number(val));
              }}
              placeholder={field.placeholder}
              min={field.min}
              max={field.max}
              disabled={disabled}
            />
            {field.description && (
              <p className="text-xs text-muted-foreground">{field.description}</p>
            )}
          </div>
        );

      case "secret":
        return (
          <div className="space-y-2">
            <Label htmlFor={id}>{field.label}</Label>
            <Input
              id={id}
              type="password"
              value={String(effectiveValue ?? "")}
              onChange={(e) => onChange(e.target.value)}
              placeholder={field.placeholder}
              disabled={disabled}
            />
            {field.description && (
              <p className="text-xs text-muted-foreground">{field.description}</p>
            )}
          </div>
        );

      case "string":
      default:
        return (
          <div className="space-y-2">
            <Label htmlFor={id}>{field.label}</Label>
            <Input
              id={id}
              type="text"
              value={String(effectiveValue ?? "")}
              onChange={(e) => onChange(e.target.value)}
              placeholder={field.placeholder}
              disabled={disabled}
            />
            {field.description && (
              <p className="text-xs text-muted-foreground">{field.description}</p>
            )}
          </div>
        );
    }
  };

  return (
    <div
      className={cn(
        "rounded-lg border border-border p-4",
        field.type === "boolean" ? "" : "space-y-2"
      )}
    >
      {renderField()}
    </div>
  );
}

export default ConfigForm;
