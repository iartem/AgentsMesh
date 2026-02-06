import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// Mock must be at module level for Vitest hoisting
vi.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const translations: Record<string, string> = {
      "title": "Webhook Settings",
      "loading": "Loading...",
      "status.registered": "Registered",
      "delete": "Delete",
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

describe("WebhookSettings - Props", () => {
  const mockOnUpdate = vi.fn();

  beforeEach(() => {
    resetAllMocks();
  });

  afterEach(() => {
    vi.resetAllMocks();
  });

  it("should work without onUpdate callback", async () => {
    mockGetWebhookStatus.mockResolvedValue({ webhook_status: registeredStatus });
    mockDeleteWebhook.mockResolvedValue({ message: "Deleted" });

    render(<WebhookSettings repository={mockRepository} />);

    await waitFor(() => {
      expect(screen.getByText("Delete")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Delete"));

    await waitFor(() => {
      expect(mockDeleteWebhook).toHaveBeenCalled();
    });

    // Should not throw even without onUpdate
  });

  it("should reload status when repository id changes", async () => {
    mockGetWebhookStatus.mockResolvedValue({ webhook_status: registeredStatus });

    const { rerender } = render(<WebhookSettings repository={mockRepository} onUpdate={mockOnUpdate} />);

    await waitFor(() => {
      expect(screen.getByText("Registered")).toBeInTheDocument();
    });

    expect(mockGetWebhookStatus).toHaveBeenCalledTimes(1);

    // Change repository
    const newRepo = { ...mockRepository, id: 2 };
    rerender(<WebhookSettings repository={newRepo} onUpdate={mockOnUpdate} />);

    await waitFor(() => {
      expect(mockGetWebhookStatus).toHaveBeenCalledTimes(2);
    });
  });
});
