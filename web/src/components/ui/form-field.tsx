"use client";

import * as React from "react";
import { Label } from "./label";
import { cn } from "@/lib/utils";

export interface FormFieldProps {
  /** Field label text */
  label: string;
  /** Unique identifier for the field, used for htmlFor attribute */
  htmlFor?: string;
  /** Error message to display below the field */
  error?: string;
  /** Helper text to display below the field */
  hint?: string;
  /** Whether the field is required (adds visual indicator) */
  required?: boolean;
  /** Whether the field is disabled */
  disabled?: boolean;
  /** Additional class name for the container */
  className?: string;
  /** The form control element (Input, Select, Textarea, etc.) */
  children: React.ReactNode;
}

/**
 * FormField - Consistent layout wrapper for form controls
 *
 * Provides standardized label, error, and hint styling for form fields.
 *
 * @example Basic usage with Input
 * ```tsx
 * <FormField label="Email" htmlFor="email" required>
 *   <Input id="email" type="email" placeholder="you@example.com" />
 * </FormField>
 * ```
 *
 * @example With error and hint
 * ```tsx
 * <FormField
 *   label="Password"
 *   htmlFor="password"
 *   error={errors.password}
 *   hint="Must be at least 8 characters"
 * >
 *   <Input id="password" type="password" />
 * </FormField>
 * ```
 *
 * @example With Select
 * ```tsx
 * <FormField label="Country" htmlFor="country">
 *   <Select>
 *     <SelectTrigger id="country">
 *       <SelectValue placeholder="Select country" />
 *     </SelectTrigger>
 *     <SelectContent>...</SelectContent>
 *   </Select>
 * </FormField>
 * ```
 */
export function FormField({
  label,
  htmlFor,
  error,
  hint,
  required,
  disabled,
  className,
  children,
}: FormFieldProps) {
  return (
    <div className={cn("space-y-2", className)}>
      <Label
        htmlFor={htmlFor}
        className={cn(
          "block text-sm font-medium",
          disabled && "opacity-50 cursor-not-allowed"
        )}
      >
        {label}
        {required && <span className="text-destructive ml-1">*</span>}
      </Label>
      {children}
      {error && (
        <p className="text-xs text-destructive" role="alert">
          {error}
        </p>
      )}
      {hint && !error && (
        <p className="text-xs text-muted-foreground">{hint}</p>
      )}
    </div>
  );
}

export interface FormFieldGroupProps {
  /** Section title */
  title?: string;
  /** Section description */
  description?: string;
  /** Additional class name */
  className?: string;
  /** Form fields */
  children: React.ReactNode;
}

/**
 * FormFieldGroup - Groups related form fields with optional title and description
 *
 * @example
 * ```tsx
 * <FormFieldGroup title="Personal Information" description="Enter your details">
 *   <FormField label="Name" htmlFor="name">
 *     <Input id="name" />
 *   </FormField>
 *   <FormField label="Email" htmlFor="email">
 *     <Input id="email" type="email" />
 *   </FormField>
 * </FormFieldGroup>
 * ```
 */
export function FormFieldGroup({
  title,
  description,
  className,
  children,
}: FormFieldGroupProps) {
  return (
    <div className={cn("space-y-4", className)}>
      {(title || description) && (
        <div className="space-y-1">
          {title && <h3 className="text-sm font-medium">{title}</h3>}
          {description && (
            <p className="text-sm text-muted-foreground">{description}</p>
          )}
        </div>
      )}
      <div className="space-y-4">{children}</div>
    </div>
  );
}

export interface FormRowProps {
  /** Additional class name */
  className?: string;
  /** Gap between items (default: 4) */
  gap?: 2 | 3 | 4 | 6 | 8;
  /** Form fields to display in a row */
  children: React.ReactNode;
}

/**
 * FormRow - Displays form fields in a horizontal row
 *
 * @example
 * ```tsx
 * <FormRow>
 *   <FormField label="First Name" htmlFor="firstName" className="flex-1">
 *     <Input id="firstName" />
 *   </FormField>
 *   <FormField label="Last Name" htmlFor="lastName" className="flex-1">
 *     <Input id="lastName" />
 *   </FormField>
 * </FormRow>
 * ```
 */
export function FormRow({ className, gap = 4, children }: FormRowProps) {
  const gapClass = {
    2: "gap-2",
    3: "gap-3",
    4: "gap-4",
    6: "gap-6",
    8: "gap-8",
  }[gap];

  return (
    <div className={cn("flex flex-col sm:flex-row", gapClass, className)}>
      {children}
    </div>
  );
}
