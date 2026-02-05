"use client";

import * as React from "react";
import { useState, useCallback } from "react";
import { AlertTriangle, Info, AlertCircle, CheckCircle } from "lucide-react";
import { Dialog, DialogContent, DialogFooter, DialogBody } from "./dialog";
import { Button } from "./button";
import { cn } from "@/lib/utils";

export type ConfirmDialogVariant = "default" | "destructive" | "warning" | "success";

export interface ConfirmDialogProps {
  /** Whether the dialog is open */
  open: boolean;
  /** Callback when the open state changes */
  onOpenChange: (open: boolean) => void;
  /** Dialog title */
  title: string;
  /** Optional description below the title */
  description?: string;
  /** Callback when the confirm button is clicked. Can be async. */
  onConfirm: () => void | Promise<void>;
  /** Text for the confirm button. Default: "Confirm" */
  confirmText?: string;
  /** Text for the cancel button. Default: "Cancel" */
  cancelText?: string;
  /** Visual variant of the dialog. Default: "default" */
  variant?: ConfirmDialogVariant;
  /** Whether to show an icon. Default: true */
  showIcon?: boolean;
  /** Whether the confirm button should show loading state while onConfirm is executing */
  loading?: boolean;
  /** Disable the confirm button */
  confirmDisabled?: boolean;
  /** Additional content to render in the dialog body */
  children?: React.ReactNode;
}

const variantConfig: Record<
  ConfirmDialogVariant,
  {
    icon: React.ElementType;
    iconClass: string;
    confirmVariant: "default" | "destructive" | "outline" | "secondary" | "ghost" | "link";
  }
> = {
  default: {
    icon: Info,
    iconClass: "text-primary bg-primary/10",
    confirmVariant: "default",
  },
  destructive: {
    icon: AlertTriangle,
    iconClass: "text-destructive bg-destructive/10",
    confirmVariant: "destructive",
  },
  warning: {
    icon: AlertCircle,
    iconClass: "text-yellow-600 dark:text-yellow-400 bg-yellow-100 dark:bg-yellow-900/30",
    confirmVariant: "default",
  },
  success: {
    icon: CheckCircle,
    iconClass: "text-green-600 dark:text-green-400 bg-green-100 dark:bg-green-900/30",
    confirmVariant: "default",
  },
};

/**
 * ConfirmDialog - A standardized confirmation dialog component
 *
 * @example Basic usage
 * ```tsx
 * const [open, setOpen] = useState(false);
 *
 * <ConfirmDialog
 *   open={open}
 *   onOpenChange={setOpen}
 *   title="Confirm Action"
 *   description="Are you sure you want to proceed?"
 *   onConfirm={() => doSomething()}
 * />
 * ```
 *
 * @example Destructive variant
 * ```tsx
 * <ConfirmDialog
 *   open={open}
 *   onOpenChange={setOpen}
 *   title="Delete Item"
 *   description="This action cannot be undone."
 *   variant="destructive"
 *   confirmText="Delete"
 *   onConfirm={() => deleteItem()}
 * />
 * ```
 *
 * @example With async confirm handler
 * ```tsx
 * <ConfirmDialog
 *   open={open}
 *   onOpenChange={setOpen}
 *   title="Save Changes"
 *   onConfirm={async () => {
 *     await saveChanges();
 *     // Dialog will show loading state
 *   }}
 * />
 * ```
 *
 * @example With custom content
 * ```tsx
 * <ConfirmDialog
 *   open={open}
 *   onOpenChange={setOpen}
 *   title="Confirm Details"
 *   onConfirm={handleConfirm}
 * >
 *   <div className="space-y-2">
 *     <p>You are about to change:</p>
 *     <ul className="list-disc pl-4">
 *       <li>Item 1</li>
 *       <li>Item 2</li>
 *     </ul>
 *   </div>
 * </ConfirmDialog>
 * ```
 */
export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  onConfirm,
  confirmText = "Confirm",
  cancelText = "Cancel",
  variant = "default",
  showIcon = true,
  loading: externalLoading,
  confirmDisabled,
  children,
}: ConfirmDialogProps) {
  const [internalLoading, setInternalLoading] = useState(false);
  const loading = externalLoading ?? internalLoading;

  const config = variantConfig[variant];
  const Icon = config.icon;

  const handleConfirm = useCallback(async () => {
    try {
      setInternalLoading(true);
      await onConfirm();
      onOpenChange(false);
    } catch (error) {
      // Error handling is delegated to the caller
      console.error("ConfirmDialog onConfirm error:", error);
    } finally {
      setInternalLoading(false);
    }
  }, [onConfirm, onOpenChange]);

  const handleCancel = useCallback(() => {
    if (!loading) {
      onOpenChange(false);
    }
  }, [loading, onOpenChange]);

  return (
    <Dialog open={open} onOpenChange={handleCancel}>
      <DialogContent className="max-w-sm">
        <DialogBody>
          <div className="flex flex-col items-center text-center">
            {showIcon && (
              <div
                className={cn(
                  "w-12 h-12 rounded-full flex items-center justify-center mb-4",
                  config.iconClass
                )}
              >
                <Icon className="w-6 h-6" />
              </div>
            )}
            <h3 className="text-lg font-semibold">{title}</h3>
            {description && (
              <p className="text-sm text-muted-foreground mt-2">{description}</p>
            )}
            {children && <div className="mt-4 w-full text-left">{children}</div>}
          </div>
        </DialogBody>
        <DialogFooter className="justify-center sm:justify-center gap-3">
          <Button
            variant="outline"
            onClick={handleCancel}
            disabled={loading}
            className="min-w-[100px]"
          >
            {cancelText}
          </Button>
          <Button
            variant={config.confirmVariant}
            onClick={handleConfirm}
            disabled={loading || confirmDisabled}
            className="min-w-[100px]"
          >
            {loading ? (
              <span className="flex items-center gap-2">
                <span className="w-4 h-4 border-2 border-current border-t-transparent rounded-full animate-spin" />
                Loading...
              </span>
            ) : (
              confirmText
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/**
 * Hook for managing confirm dialog state
 *
 * @example Static configuration
 * ```tsx
 * const { dialogProps, confirm } = useConfirmDialog({
 *   title: "Delete Item",
 *   description: "Are you sure?",
 *   variant: "destructive",
 * });
 *
 * // Trigger the dialog
 * const handleDelete = async () => {
 *   const confirmed = await confirm();
 *   if (confirmed) {
 *     // User confirmed
 *     await deleteItem();
 *   }
 * };
 *
 * return (
 *   <>
 *     <Button onClick={handleDelete}>Delete</Button>
 *     <ConfirmDialog {...dialogProps} />
 *   </>
 * );
 * ```
 *
 * @example Dynamic configuration (no default options)
 * ```tsx
 * const { dialogProps, confirm } = useConfirmDialog();
 *
 * // Trigger with dynamic options
 * const handleDelete = async (itemName: string) => {
 *   const confirmed = await confirm({
 *     title: "Delete Item",
 *     description: `Are you sure you want to delete "${itemName}"?`,
 *     variant: "destructive",
 *     confirmText: "Delete",
 *   });
 *   if (confirmed) {
 *     await deleteItem(itemName);
 *   }
 * };
 *
 * return (
 *   <>
 *     <Button onClick={() => handleDelete("Item 1")}>Delete</Button>
 *     <ConfirmDialog {...dialogProps} />
 *   </>
 * );
 * ```
 */
export interface UseConfirmDialogOptions {
  title: string;
  description?: string;
  confirmText?: string;
  cancelText?: string;
  variant?: ConfirmDialogVariant;
}

export function useConfirmDialog(defaultOptions?: UseConfirmDialogOptions) {
  const [open, setOpen] = useState(false);
  const [currentOptions, setCurrentOptions] = useState<UseConfirmDialogOptions>(
    defaultOptions ?? { title: "" }
  );
  const resolveRef = React.useRef<((value: boolean) => void) | null>(null);

  const confirm = useCallback((options?: UseConfirmDialogOptions) => {
    if (options) {
      setCurrentOptions(options);
    } else if (defaultOptions) {
      setCurrentOptions(defaultOptions);
    }
    setOpen(true);
    return new Promise<boolean>((resolve) => {
      resolveRef.current = resolve;
    });
  }, [defaultOptions]);

  const handleConfirm = useCallback(() => {
    resolveRef.current?.(true);
    resolveRef.current = null;
  }, []);

  const handleOpenChange = useCallback((newOpen: boolean) => {
    setOpen(newOpen);
    if (!newOpen) {
      resolveRef.current?.(false);
      resolveRef.current = null;
    }
  }, []);

  const dialogProps: ConfirmDialogProps = {
    open,
    onOpenChange: handleOpenChange,
    onConfirm: handleConfirm,
    ...currentOptions,
  };

  return {
    dialogProps,
    confirm,
    isOpen: open,
  };
}
