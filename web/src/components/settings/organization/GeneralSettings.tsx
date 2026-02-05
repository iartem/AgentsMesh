"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { FormField } from "@/components/ui/form-field";
import { organizationApi } from "@/lib/api";

export type TranslationFn = (key: string, params?: Record<string, string | number>) => string;

interface GeneralSettingsProps {
  org: { name: string; slug: string } | null;
  t: TranslationFn;
}

export function GeneralSettings({ org, t }: GeneralSettingsProps) {
  const [name, setName] = useState(org?.name || "");
  const [saving, setSaving] = useState(false);

  const handleSave = async () => {
    setSaving(true);
    try {
      await organizationApi.update(org!.slug, { name });
    } catch (error) {
      console.error("Failed to save:", error);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="border border-border rounded-lg p-6">
        <h2 className="text-lg font-semibold mb-4">{t("settings.organizationDetails.title")}</h2>
        <div className="space-y-4">
          <FormField
            label={t("settings.organizationDetails.nameLabel")}
            htmlFor="org-name"
          >
            <Input
              id="org-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={t("settings.organizationDetails.namePlaceholder")}
            />
          </FormField>

          <FormField
            label={t("settings.organizationDetails.slugLabel")}
            htmlFor="org-slug"
            hint={t("settings.organizationDetails.slugHint")}
          >
            <Input id="org-slug" value={org?.slug || ""} disabled />
          </FormField>
        </div>
        <div className="mt-6">
          <Button onClick={handleSave} disabled={saving}>
            {saving ? t("settings.organizationDetails.saving") : t("settings.organizationDetails.saveChanges")}
          </Button>
        </div>
      </div>

      <div className="border border-destructive rounded-lg p-6">
        <h2 className="text-lg font-semibold text-destructive mb-4">
          {t("settings.dangerZone.title")}
        </h2>
        <p className="text-sm text-muted-foreground mb-4">
          {t("settings.dangerZone.description")}
        </p>
        <Button variant="destructive">{t("settings.dangerZone.deleteOrg")}</Button>
      </div>
    </div>
  );
}
