"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { ArrowLeft, Tag } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { createPromoCode, PromoCodeType } from "@/lib/api/admin";

export default function NewPromoCodePage() {
  const router = useRouter();
  const [isCreating, setIsCreating] = useState(false);
  const [formData, setFormData] = useState({
    code: "",
    name: "",
    description: "",
    type: "campaign" as PromoCodeType,
    plan_name: "pro",
    duration_months: 1,
    max_uses: "",
    max_uses_per_org: 1,
    expires_at: "",
  });

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    const data = {
      code: formData.code.toUpperCase(),
      name: formData.name,
      description: formData.description || undefined,
      type: formData.type,
      plan_name: formData.plan_name,
      duration_months: formData.duration_months,
      max_uses: formData.max_uses ? parseInt(formData.max_uses) : undefined,
      max_uses_per_org: formData.max_uses_per_org,
      expires_at: formData.expires_at
        ? new Date(formData.expires_at).toISOString()
        : undefined,
    };

    setIsCreating(true);
    try {
      await createPromoCode(data);
      toast.success("Promo code created successfully");
      router.push("/promo-codes");
    } catch (err: unknown) {
      toast.error((err as { error?: string })?.error || "Failed to create promo code");
    } finally {
      setIsCreating(false);
    }
  };

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      {/* Header */}
      <div className="flex items-center gap-4">
        <Link href="/promo-codes">
          <Button variant="ghost" size="icon">
            <ArrowLeft className="h-4 w-4" />
          </Button>
        </Link>
        <div>
          <h1 className="text-2xl font-bold">Create Promo Code</h1>
          <p className="text-sm text-muted-foreground">
            Create a new promotional code for subscriptions
          </p>
        </div>
      </div>

      {/* Form */}
      <form onSubmit={handleSubmit}>
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Tag className="h-5 w-5" />
              Promo Code Details
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-6">
            {/* Code */}
            <div className="space-y-2">
              <Label htmlFor="code">Code *</Label>
              <Input
                id="code"
                placeholder="e.g., SUMMER2024"
                value={formData.code}
                onChange={(e) =>
                  setFormData({ ...formData, code: e.target.value.toUpperCase() })
                }
                required
                minLength={4}
                maxLength={50}
                className="font-mono uppercase"
              />
              <p className="text-xs text-muted-foreground">
                4-50 characters, will be converted to uppercase
              </p>
            </div>

            {/* Name */}
            <div className="space-y-2">
              <Label htmlFor="name">Name *</Label>
              <Input
                id="name"
                placeholder="e.g., Summer Sale 2024"
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                required
                maxLength={100}
              />
            </div>

            {/* Description */}
            <div className="space-y-2">
              <Label htmlFor="description">Description</Label>
              <Textarea
                id="description"
                placeholder="Optional description..."
                value={formData.description}
                onChange={(e) =>
                  setFormData({ ...formData, description: e.target.value })
                }
                rows={3}
              />
            </div>

            {/* Type & Plan */}
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label>Type *</Label>
                <Select
                  value={formData.type}
                  onValueChange={(value) =>
                    setFormData({ ...formData, type: value as PromoCodeType })
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="media">Media</SelectItem>
                    <SelectItem value="partner">Partner</SelectItem>
                    <SelectItem value="campaign">Campaign</SelectItem>
                    <SelectItem value="internal">Internal</SelectItem>
                    <SelectItem value="referral">Referral</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>Plan *</Label>
                <Select
                  value={formData.plan_name}
                  onValueChange={(value) =>
                    setFormData({ ...formData, plan_name: value })
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="pro">Pro</SelectItem>
                    <SelectItem value="enterprise">Enterprise</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            {/* Duration */}
            <div className="space-y-2">
              <Label htmlFor="duration_months">Duration (months) *</Label>
              <Input
                id="duration_months"
                type="number"
                min={1}
                max={24}
                value={formData.duration_months}
                onChange={(e) =>
                  setFormData({
                    ...formData,
                    duration_months: parseInt(e.target.value) || 1,
                  })
                }
                required
              />
              <p className="text-xs text-muted-foreground">
                Number of months the subscription will be extended
              </p>
            </div>

            {/* Usage Limits */}
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="max_uses">Max Total Uses</Label>
                <Input
                  id="max_uses"
                  type="number"
                  min={1}
                  placeholder="Unlimited"
                  value={formData.max_uses}
                  onChange={(e) =>
                    setFormData({ ...formData, max_uses: e.target.value })
                  }
                />
                <p className="text-xs text-muted-foreground">
                  Leave empty for unlimited uses
                </p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="max_uses_per_org">Max Uses per Organization</Label>
                <Input
                  id="max_uses_per_org"
                  type="number"
                  min={1}
                  value={formData.max_uses_per_org}
                  onChange={(e) =>
                    setFormData({
                      ...formData,
                      max_uses_per_org: parseInt(e.target.value) || 1,
                    })
                  }
                />
              </div>
            </div>

            {/* Expiration */}
            <div className="space-y-2">
              <Label htmlFor="expires_at">Expiration Date</Label>
              <Input
                id="expires_at"
                type="datetime-local"
                value={formData.expires_at}
                onChange={(e) =>
                  setFormData({ ...formData, expires_at: e.target.value })
                }
              />
              <p className="text-xs text-muted-foreground">
                Leave empty for no expiration
              </p>
            </div>

            {/* Actions */}
            <div className="flex justify-end gap-3 pt-4">
              <Link href="/promo-codes">
                <Button type="button" variant="outline">
                  Cancel
                </Button>
              </Link>
              <Button type="submit" disabled={isCreating}>
                {isCreating ? "Creating..." : "Create Promo Code"}
              </Button>
            </div>
          </CardContent>
        </Card>
      </form>
    </div>
  );
}
