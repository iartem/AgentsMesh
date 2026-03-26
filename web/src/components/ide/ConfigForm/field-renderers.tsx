"use client";

import React, { memo, useState } from "react";
import type { ConfigField } from "@/lib/api/agent";
import { useTranslations } from "next-intl";
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  DragEndEvent,
} from "@dnd-kit/core";
import {
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { GripVertical, X, Plus } from "lucide-react";
import { cn } from "@/lib/utils";

/**
 * Props for field renderer components
 */
export interface FieldRendererProps {
  fieldKey: string;
  field: ConfigField;
  value: unknown;
  onChange: (value: unknown) => void;
  /** Agent slug for i18n translation key construction */
  agentSlug: string;
  /** All current values in the form - used for dynamic options */
  values?: Record<string, unknown>;
}

/**
 * Hook for getting translated field labels and descriptions
 * Uses the pattern: agent.{agentSlug}.fields.{fieldName}.label/description
 * Falls back to humanized field name if translation key is missing.
 */
function useFieldTranslation(agentSlug: string, fieldName: string) {
  const t = useTranslations();
  const basePath = `agent.${agentSlug}.fields.${fieldName}`;

  const humanized = fieldName.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());

  return {
    label: t.has(`${basePath}.label`) ? t(`${basePath}.label`) : humanized,
    description: t.has(`${basePath}.description`) ? t(`${basePath}.description`) : "",
    getOptionLabel: (optionValue: string) => {
      const key = optionValue === "" ? `${basePath}.options.` : `${basePath}.options.${optionValue}`;
      return t.has(key) ? t(key) : optionValue || humanized;
    },
  };
}

/**
 * Boolean field renderer (checkbox)
 */
function BooleanField({
  fieldKey,
  label,
  description,
  value,
  onChange,
}: {
  fieldKey: string;
  label: string;
  description: string;
  value: unknown;
  onChange: (value: unknown) => void;
}) {
  return (
    <div className="flex items-center gap-2">
      <input
        type="checkbox"
        id={fieldKey}
        checked={Boolean(value)}
        onChange={(e) => onChange(e.target.checked)}
        className="h-4 w-4 rounded border-border"
        aria-describedby={description ? `${fieldKey}-desc` : undefined}
      />
      <label htmlFor={fieldKey} className="text-sm">
        {label}
      </label>
      {description && (
        <span id={`${fieldKey}-desc`} className="text-xs text-muted-foreground ml-auto">
          {description}
        </span>
      )}
    </div>
  );
}

/**
 * String field renderer (text input)
 */
function StringField({
  fieldKey,
  label,
  description,
  value,
  onChange,
  required,
}: {
  fieldKey: string;
  label: string;
  description: string;
  value: unknown;
  onChange: (value: unknown) => void;
  required?: boolean;
}) {
  return (
    <div>
      <label htmlFor={fieldKey} className="block text-sm font-medium mb-1">
        {label}
        {required && <span className="text-destructive ml-1">*</span>}
      </label>
      <input
        type="text"
        id={fieldKey}
        value={String(value ?? "")}
        onChange={(e) => onChange(e.target.value)}
        className="w-full px-3 py-2 text-sm border border-border rounded-md bg-background"
        aria-describedby={description ? `${fieldKey}-desc` : undefined}
        aria-required={required}
      />
      {description && (
        <p id={`${fieldKey}-desc`} className="text-xs text-muted-foreground mt-1">
          {description}
        </p>
      )}
    </div>
  );
}

/**
 * Secret field renderer (password input)
 */
function SecretField({
  fieldKey,
  label,
  description,
  value,
  onChange,
  required,
}: {
  fieldKey: string;
  label: string;
  description: string;
  value: unknown;
  onChange: (value: unknown) => void;
  required?: boolean;
}) {
  return (
    <div>
      <label htmlFor={fieldKey} className="block text-sm font-medium mb-1">
        {label}
        {required && <span className="text-destructive ml-1">*</span>}
      </label>
      <input
        type="password"
        id={fieldKey}
        value={String(value ?? "")}
        onChange={(e) => onChange(e.target.value)}
        className="w-full px-3 py-2 text-sm border border-border rounded-md bg-background"
        aria-describedby={description ? `${fieldKey}-desc` : undefined}
        aria-required={required}
      />
      {description && (
        <p id={`${fieldKey}-desc`} className="text-xs text-muted-foreground mt-1">
          {description}
        </p>
      )}
    </div>
  );
}

/**
 * Number field renderer
 */
function NumberField({
  fieldKey,
  label,
  description,
  value,
  onChange,
  required,
  min,
  max,
}: {
  fieldKey: string;
  label: string;
  description: string;
  value: unknown;
  onChange: (value: unknown) => void;
  required?: boolean;
  min?: number;
  max?: number;
}) {
  return (
    <div>
      <label htmlFor={fieldKey} className="block text-sm font-medium mb-1">
        {label}
        {required && <span className="text-destructive ml-1">*</span>}
      </label>
      <input
        type="number"
        id={fieldKey}
        value={value != null ? Number(value) : ""}
        min={min}
        max={max}
        onChange={(e) => onChange(e.target.value ? Number(e.target.value) : null)}
        className="w-full px-3 py-2 text-sm border border-border rounded-md bg-background"
        aria-describedby={description ? `${fieldKey}-desc` : undefined}
        aria-required={required}
      />
      {description && (
        <p id={`${fieldKey}-desc`} className="text-xs text-muted-foreground mt-1">
          {description}
        </p>
      )}
    </div>
  );
}

/**
 * Select field renderer (dropdown)
 */
function SelectField({
  fieldKey,
  label,
  description,
  value,
  onChange,
  required,
  options,
  getOptionLabel,
}: {
  fieldKey: string;
  label: string;
  description: string;
  value: unknown;
  onChange: (value: unknown) => void;
  required?: boolean;
  options?: { value: string }[];
  getOptionLabel: (value: string) => string;
}) {
  return (
    <div>
      <label htmlFor={fieldKey} className="block text-sm font-medium mb-1">
        {label}
        {required && <span className="text-destructive ml-1">*</span>}
      </label>
      <select
        id={fieldKey}
        value={String(value ?? "")}
        onChange={(e) => onChange(e.target.value)}
        className="w-full px-3 py-2 text-sm border border-border rounded-md bg-background"
        aria-describedby={description ? `${fieldKey}-desc` : undefined}
        aria-required={required}
      >
        {!required && !value && !options?.some((o) => o.value === "") && (
          <option value="" disabled>
            Select {label.toLowerCase()}...
          </option>
        )}
        {options?.map((option) => (
          <option key={option.value} value={option.value}>
            {getOptionLabel(option.value)}
          </option>
        ))}
      </select>
      {description && (
        <p id={`${fieldKey}-desc`} className="text-xs text-muted-foreground mt-1">
          {description}
        </p>
      )}
    </div>
  );
}

/**
 * Sortable item for model list
 */
function SortableModelItem({
  model,
  onRemove,
}: {
  model: string;
  onRemove: () => void;
}) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: model });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={cn(
        "flex items-center gap-2 px-3 py-2 bg-background border border-border rounded-md",
        isDragging && "opacity-50 z-50 shadow-md"
      )}
    >
      <button
        type="button"
        {...attributes}
        {...listeners}
        className="cursor-grab active:cursor-grabbing text-muted-foreground hover:text-foreground"
        aria-label="Drag to reorder"
      >
        <GripVertical className="h-4 w-4" />
      </button>
      <span className="flex-1 text-sm truncate">{model}</span>
      <button
        type="button"
        onClick={onRemove}
        className="text-muted-foreground hover:text-destructive"
        aria-label={`Remove ${model}`}
      >
        <X className="h-4 w-4" />
      </button>
    </div>
  );
}

/**
 * Model list field renderer (drag-and-drop list of models)
 */
function ModelListField({
  fieldKey,
  label,
  description,
  value,
  onChange,
}: {
  fieldKey: string;
  label: string;
  description: string;
  value: unknown;
  onChange: (value: unknown) => void;
}) {
  const [newModel, setNewModel] = useState("");

  const models = Array.isArray(value) ? value.filter((m): m is string => typeof m === "string") : [];

  const sensors = useSensors(
    useSensor(PointerSensor),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    })
  );

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;

    const oldIndex = models.indexOf(String(active.id));
    const newIndex = models.indexOf(String(over.id));
    if (oldIndex === -1 || newIndex === -1) return;

    const newModels = [...models];
    newModels.splice(oldIndex, 1);
    newModels.splice(newIndex, 0, models[oldIndex]);
    onChange(newModels);
  };

  const handleAddModel = () => {
    const trimmed = newModel.trim();
    if (!trimmed || models.includes(trimmed)) {
      setNewModel("");
      return;
    }
    onChange([...models, trimmed]);
    setNewModel("");
  };

  const handleRemoveModel = (modelToRemove: string) => {
    onChange(models.filter((m) => m !== modelToRemove));
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      e.preventDefault();
      handleAddModel();
    }
  };

  return (
    <div>
      <label htmlFor={fieldKey} className="block text-sm font-medium mb-1">
        {label}
      </label>

      <div className="flex gap-2 mb-2">
        <input
          type="text"
          id={fieldKey}
          value={newModel}
          onChange={(e) => setNewModel(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="provider/model (e.g., anthropic/claude-sonnet-4)"
          className="flex-1 px-3 py-2 text-sm border border-border rounded-md bg-background"
        />
        <button
          type="button"
          onClick={handleAddModel}
          disabled={!newModel.trim() || models.includes(newModel.trim())}
          className="px-3 py-2 text-sm bg-primary text-primary-foreground rounded-md hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          <Plus className="h-4 w-4" />
        </button>
      </div>

      {models.length > 0 && (
        <DndContext
          sensors={sensors}
          collisionDetection={closestCenter}
          onDragEnd={handleDragEnd}
        >
          <SortableContext items={models} strategy={verticalListSortingStrategy}>
            <div className="space-y-2">
              {models.map((model) => (
                <SortableModelItem
                  key={model}
                  model={model}
                  onRemove={() => handleRemoveModel(model)}
                />
              ))}
            </div>
          </SortableContext>
        </DndContext>
      )}

      {description && (
        <p id={`${fieldKey}-desc`} className="text-xs text-muted-foreground mt-2">
          {description}
        </p>
      )}
    </div>
  );
}

/**
 * Unified field renderer component
 * Uses switch statement internally to select the appropriate field type rendering
 * This pattern avoids dynamic component creation during render (react-compiler compliant)
 */
export const FieldRenderer = memo(function FieldRenderer({
  fieldKey,
  field,
  value,
  onChange,
  agentSlug,
  values,
}: FieldRendererProps) {
  const { label, description, getOptionLabel } = useFieldTranslation(agentSlug, field.name);

  switch (field.type) {
    case "boolean":
      return (
        <BooleanField
          fieldKey={fieldKey}
          label={label}
          description={description}
          value={value}
          onChange={onChange}
        />
      );

    case "string":
      return (
        <StringField
          fieldKey={fieldKey}
          label={label}
          description={description}
          value={value}
          onChange={onChange}
          required={field.required}
        />
      );

    case "secret":
      return (
        <SecretField
          fieldKey={fieldKey}
          label={label}
          description={description}
          value={value}
          onChange={onChange}
          required={field.required}
        />
      );

    case "number":
      return (
        <NumberField
          fieldKey={fieldKey}
          label={label}
          description={description}
          value={value}
          onChange={onChange}
          required={field.required}
          min={field.validation?.min}
          max={field.validation?.max}
        />
      );

    case "select": {
      let options = field.options;
      if (field.name === "model" && values?.models) {
        const modelList = Array.isArray(values.models) ? values.models.filter((m): m is string => typeof m === "string") : [];
        if (modelList.length > 0) {
          options = modelList.map((m) => ({ value: m }));
        }
      }
      return (
        <SelectField
          fieldKey={fieldKey}
          label={label}
          description={description}
          value={value}
          onChange={onChange}
          required={field.required}
          options={options}
          getOptionLabel={getOptionLabel}
        />
      );
    }

    case "model_list":
      return (
        <ModelListField
          fieldKey={fieldKey}
          label={label}
          description={description}
          value={value}
          onChange={onChange}
        />
      );

    default:
      return (
        <div className="text-sm text-muted-foreground">
          Unknown field type: {field.type} ({fieldKey})
        </div>
      );
  }
});
