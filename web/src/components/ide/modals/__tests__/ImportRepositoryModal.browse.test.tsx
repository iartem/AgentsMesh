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

describe("ImportRepositoryModal - Provider Selection and Browse", () => {
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

  it("should navigate to browse step when provider is selected", async () => {
    render(
      <ImportRepositoryModal open={true} onClose={mockOnClose} onImported={mockOnImported} />
    );

    await waitFor(() => {
      expect(screen.getByText("My GitHub")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("My GitHub"));

    await waitFor(() => {
      expect(userRepositoryProviderApi.listRepositories).toHaveBeenCalledWith(1, expect.any(Object));
    });
  });

  it("should display repositories after selecting provider", async () => {
    render(
      <ImportRepositoryModal open={true} onClose={mockOnClose} onImported={mockOnImported} />
    );

    await waitFor(() => {
      expect(screen.getByText("My GitHub")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("My GitHub"));

    await waitFor(() => {
      expect(screen.getByText("org/my-project")).toBeInTheDocument();
    });
  });

  it("should allow going back to source selection", async () => {
    render(
      <ImportRepositoryModal open={true} onClose={mockOnClose} onImported={mockOnImported} />
    );

    await waitFor(() => {
      expect(screen.getByText("My GitHub")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("My GitHub"));

    await waitFor(() => {
      expect(screen.getByText("org/my-project")).toBeInTheDocument();
    });

    // Click back button - find it by looking for the SVG with back arrow path
    const backButtons = document.querySelectorAll("button");
    const backButton = Array.from(backButtons).find(
      (btn) => btn.querySelector('svg path[d*="M15 19l-7-7 7-7"]')
    );
    expect(backButton).toBeTruthy();
    fireEvent.click(backButton!);

    await waitFor(() => {
      expect(screen.getByText("My GitHub")).toBeInTheDocument();
      expect(screen.getByText("My GitLab")).toBeInTheDocument();
    });
  });

  it("should show search input in browse step", async () => {
    render(
      <ImportRepositoryModal open={true} onClose={mockOnClose} onImported={mockOnImported} />
    );

    await waitFor(() => {
      expect(screen.getByText("My GitHub")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("My GitHub"));

    await waitFor(() => {
      expect(screen.getByPlaceholderText("Search repositories...")).toBeInTheDocument();
    });
  });

  it("should handle search form submission", async () => {
    render(
      <ImportRepositoryModal open={true} onClose={mockOnClose} onImported={mockOnImported} />
    );

    await waitFor(() => {
      expect(screen.getByText("My GitHub")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("My GitHub"));

    await waitFor(() => {
      expect(screen.getByPlaceholderText("Search repositories...")).toBeInTheDocument();
    });

    const searchInput = screen.getByPlaceholderText("Search repositories...");
    fireEvent.change(searchInput, { target: { value: "test-search" } });

    const searchButton = screen.getByText("Search");
    fireEvent.click(searchButton);

    await waitFor(() => {
      expect(userRepositoryProviderApi.listRepositories).toHaveBeenCalledWith(
        1,
        expect.objectContaining({ search: "test-search" })
      );
    });
  });
});
