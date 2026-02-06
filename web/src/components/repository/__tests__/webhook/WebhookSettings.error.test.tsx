import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// Mock must be at module level for Vitest hoisting
vi.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const translations: Record<string, string> = {
      "title": "Webhook Settings",
      "loading": "Loading...",
      "status.registered": "Registered",
      "error.load": "Failed to load webhook status",
      "retry": "Retry",
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

import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { WebhookSettings } from "../../webhook";
import {
  mockRepository,
  registeredStatus,
  resetAllMocks,
} from "./testSetup";

describe("WebhookSettings - Error State", () => {
  const mockOnUpdate = vi.fn();

  beforeEach(() => {
    resetAllMocks();
    mockGetWebhookStatus.mockRejectedValue(new Error("Network error"));
  });

  afterEach(() => {
    vi.resetAllMocks();
  });

  it("should display error state when status fetch fails", async () => {
    render(<WebhookSettings repository={mockRepository} onUpdate={mockOnUpdate} />);

    await waitFor(() => {
      expect(screen.getByText("Failed to load webhook status")).toBeInTheDocument();
    });
  });

  it("should show retry button in error state", async () => {
    render(<WebhookSettings repository={mockRepository} onUpdate={mockOnUpdate} />);

    await waitFor(() => {
      expect(screen.getByText("Retry")).toBeInTheDocument();
    });
  });

  it("should retry loading status when retry clicked", async () => {
    mockGetWebhookStatus
      .mockRejectedValueOnce(new Error("Network error"))
      .mockResolvedValue({ webhook_status: registeredStatus });

    render(<WebhookSettings repository={mockRepository} onUpdate={mockOnUpdate} />);

    await waitFor(() => {
      expect(screen.getByText("Retry")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Retry"));

    await waitFor(() => {
      expect(mockGetWebhookStatus).toHaveBeenCalledTimes(2);
    });

    await waitFor(() => {
      expect(screen.getByText("Registered")).toBeInTheDocument();
    });
  });
});
