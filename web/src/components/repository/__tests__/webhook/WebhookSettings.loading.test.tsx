import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";

// Mock must be at module level for Vitest hoisting
vi.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const translations: Record<string, string> = {
      "title": "Webhook Settings",
      "loading": "Loading...",
    };
    return translations[key] || key;
  },
}));

// Import mock functions before mocking the module
import {
  mockGetWebhookStatus,
  mockGetWebhookSecret,
  mockRegisterWebhook,
  mockDeleteWebhook,
  mockMarkWebhookConfigured,
} from "./testSetup";

vi.mock("@/lib/api", () => ({
  repositoryApi: {
    getWebhookStatus: (...args: unknown[]) => mockGetWebhookStatus(...args),
    getWebhookSecret: (...args: unknown[]) => mockGetWebhookSecret(...args),
    registerWebhook: (...args: unknown[]) => mockRegisterWebhook(...args),
    deleteWebhook: (...args: unknown[]) => mockDeleteWebhook(...args),
    markWebhookConfigured: (...args: unknown[]) => mockMarkWebhookConfigured(...args),
  },
}));

import { render, screen } from "@testing-library/react";
import { WebhookSettings } from "../../webhook";
import {
  mockRepository,
  resetAllMocks,
} from "./testSetup";

describe("WebhookSettings - Loading State", () => {
  const mockOnUpdate = vi.fn();

  beforeEach(() => {
    resetAllMocks();
  });

  afterEach(() => {
    vi.resetAllMocks();
  });

  it("should show loading state initially", () => {
    mockGetWebhookStatus.mockImplementation(() => new Promise(() => {})); // Never resolves

    render(<WebhookSettings repository={mockRepository} onUpdate={mockOnUpdate} />);

    expect(screen.getByText("Loading...")).toBeInTheDocument();
  });
});
