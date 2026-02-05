import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@/test/test-utils";

// Mock API module
vi.mock("@/lib/api", () => ({
  repositoryApi: { create: vi.fn() },
  userRepositoryProviderApi: { list: vi.fn(), listRepositories: vi.fn() },
}));

import { ImportRepositoryModal } from "../ImportRepositoryModal";
import { userRepositoryProviderApi } from "@/lib/api";
import {
  mockProvider,
  mockGitLabProvider,
  createListRepositoriesResponse,
} from "./ImportRepositoryModal.utils";

describe("ImportRepositoryModal - Rendering", () => {
  const mockOnClose = vi.fn();
  const mockOnImported = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(userRepositoryProviderApi.list).mockResolvedValue({
      providers: [mockProvider, mockGitLabProvider],
    });
    vi.mocked(userRepositoryProviderApi.listRepositories).mockResolvedValue(
      createListRepositoriesResponse()
    );
  });

  it("should not render when open is false", () => {
    render(
      <ImportRepositoryModal open={false} onClose={mockOnClose} onImported={mockOnImported} />
    );
    expect(screen.queryByText("Import Repository")).not.toBeInTheDocument();
  });

  it("should render when open is true", async () => {
    render(
      <ImportRepositoryModal open={true} onClose={mockOnClose} onImported={mockOnImported} />
    );

    await waitFor(() => {
      expect(screen.getByText("Import Repository")).toBeInTheDocument();
    });
  });

  it("should show loading state while fetching providers", async () => {
    vi.mocked(userRepositoryProviderApi.list).mockImplementation(
      () => new Promise((resolve) => setTimeout(() => resolve({ providers: [] }), 100))
    );

    render(
      <ImportRepositoryModal open={true} onClose={mockOnClose} onImported={mockOnImported} />
    );

    // Loading spinner should be visible initially
    const spinners = document.querySelectorAll(".animate-spin");
    expect(spinners.length).toBeGreaterThan(0);
  });

  it("should show provider connections after loading", async () => {
    render(
      <ImportRepositoryModal open={true} onClose={mockOnClose} onImported={mockOnImported} />
    );

    await waitFor(() => {
      expect(screen.getByText("My GitHub")).toBeInTheDocument();
      expect(screen.getByText("My GitLab")).toBeInTheDocument();
    });
  });

  it("should show no connections message when no providers", async () => {
    vi.mocked(userRepositoryProviderApi.list).mockResolvedValue({ providers: [] });

    render(
      <ImportRepositoryModal open={true} onClose={mockOnClose} onImported={mockOnImported} />
    );

    await waitFor(() => {
      expect(screen.getByText(/No Git connections configured/)).toBeInTheDocument();
    });
  });

  it("should show manual entry option", async () => {
    render(
      <ImportRepositoryModal open={true} onClose={mockOnClose} onImported={mockOnImported} />
    );

    await waitFor(() => {
      expect(screen.getByText("Enter Manually")).toBeInTheDocument();
    });
  });
});
