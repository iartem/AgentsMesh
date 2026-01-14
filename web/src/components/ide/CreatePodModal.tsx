"use client";

import React, { useEffect, useRef } from "react";
import { PodData } from "@/lib/api/client";
import { useTranslations } from "@/lib/i18n/client";
import { CreatePodForm } from "@/components/pod/CreatePodForm";
import { useFocusTrap } from "./hooks";

interface CreatePodModalProps {
  open: boolean;
  onClose: () => void;
  onCreated: (pod?: PodData) => void;
}

export function CreatePodModal({ open, onClose, onCreated }: CreatePodModalProps) {
  const t = useTranslations();

  // Focus trap for modal accessibility
  const modalRef = useFocusTrap<HTMLDivElement>(open, onClose);

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="create-pod-title"
    >
      <div
        ref={modalRef}
        className="bg-background border border-border rounded-lg w-full max-w-md p-4 md:p-6 max-h-[90vh] overflow-y-auto"
      >
        <h2 id="create-pod-title" className="text-lg md:text-xl font-semibold mb-4">
          {t("ide.createPod.title")}
        </h2>

        <CreatePodForm
          enabled={open}
          config={{
            scenario: "workspace",
            onSuccess: (pod) => {
              onCreated(pod);
              onClose();
            },
            onCancel: onClose,
          }}
        />
      </div>
    </div>
  );
}

export default CreatePodModal;
