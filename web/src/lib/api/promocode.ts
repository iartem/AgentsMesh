import { request, orgPath } from "./base";

// Promo code types
export type PromoCodeType = "media" | "partner" | "campaign" | "internal" | "referral";

// Validate response
export interface ValidatePromoCodeResponse {
  valid: boolean;
  code: string;
  plan_name?: string;
  plan_display_name?: string;
  duration_months?: number;
  expires_at?: string;
  message_code?: string;
}

// Redeem response
export interface RedeemPromoCodeResponse {
  success: boolean;
  plan_name?: string;
  duration_months?: number;
  new_period_end?: string;
  message_code?: string;
}

// Redemption record
export interface PromoCodeRedemption {
  id: number;
  promo_code_id: number;
  organization_id: number;
  user_id: number;
  plan_name: string;
  duration_months: number;
  previous_plan_name?: string;
  previous_period_end?: string;
  new_period_end: string;
  created_at: string;
  promo_code?: {
    code: string;
    name: string;
  };
}

// Promo Code API
export const promoCodeApi = {
  // Validate a promo code
  validate: (code: string) =>
    request<ValidatePromoCodeResponse>(orgPath("/billing/promo-codes/validate"), {
      method: "POST",
      body: { code },
    }),

  // Redeem a promo code
  redeem: (code: string) =>
    request<RedeemPromoCodeResponse>(orgPath("/billing/promo-codes/redeem"), {
      method: "POST",
      body: { code },
    }),

  // Get redemption history
  getHistory: () =>
    request<{ redemptions: PromoCodeRedemption[] }>(
      orgPath("/billing/promo-codes/history")
    ),
};
