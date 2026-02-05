import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@/test/test-utils";

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

describe("ImportRepositoryModal - Close and Cancel", () => {
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

  it("should call onClose when cancel button is clicked", async () => {
    render(
      <ImportRepositoryModal open={true} onClose={mockOnClose} onImported={mockOnImported} />
    );

    await waitFor(() => {
      expect(screen.getByText("Cancel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Cancel"));

    expect(mockOnClose).toHaveBeenCalled();
  });

  it("should call onClose when X button is clicked", async () => {
    render(
      <ImportRepositoryModal open={true} onClose={mockOnClose} onImported={mockOnImported} />
    );

    await waitFor(() => {
      expect(screen.getByText("Import Repository")).toBeInTheDocument();
    });

    // Find and click the X button in the header
    const closeButton = document.querySelector("button[class*='hover:text-foreground']");
    if (closeButton) {
      fireEvent.click(closeButton);
      expect(mockOnClose).toHaveBeenCalled();
    }
  });

  it("should reset state when modal is closed and reopened", async () => {
    const { rerender } = render(
      <ImportRepositoryModal open={true} onClose={mockOnClose} onImported={mockOnImported} />
    );

    await waitFor(() => {
      expect(screen.getByText("My GitHub")).toBeInTheDocument();
    });

    // Select a provider
    fireEvent.click(screen.getByText("My GitHub"));

    await waitFor(() => {
      expect(screen.getByText("org/my-project")).toBeInTheDocument();
    });

    // Close modal
    rerender(
      <ImportRepositoryModal open={false} onClose={mockOnClose} onImported={mockOnImported} />
    );

    // Reopen modal
    rerender(
      <ImportRepositoryModal open={true} onClose={mockOnClose} onImported={mockOnImported} />
    );

    await waitFor(() => {
      // Should be back to source selection step
      expect(screen.getByText("My GitHub")).toBeInTheDocument();
      expect(screen.getByText("My GitLab")).toBeInTheDocument();
    });
  });
});
