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

describe("ImportRepositoryModal - Back Navigation", () => {
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

  it("should go back from confirm step to browse step", async () => {
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

    fireEvent.click(screen.getByText("org/my-project"));

    await waitFor(() => {
      expect(screen.getByText("Confirm Import")).toBeInTheDocument();
    });

    // Click back button
    const backButtons = document.querySelectorAll("button");
    const backButton = Array.from(backButtons).find(
      (btn) => btn.querySelector('svg path[d*="M15 19l-7-7 7-7"]')
    );
    expect(backButton).toBeTruthy();
    fireEvent.click(backButton!);

    await waitFor(() => {
      expect(screen.getByText("org/my-project")).toBeInTheDocument();
      expect(screen.getByPlaceholderText("Search repositories...")).toBeInTheDocument();
    });
  });

  it("should go back from confirm step to manual step", async () => {
    render(
      <ImportRepositoryModal open={true} onClose={mockOnClose} onImported={mockOnImported} />
    );

    await waitFor(() => {
      expect(screen.getByText("Enter Manually")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Enter Manually"));

    await waitFor(() => {
      expect(screen.getByPlaceholderText("https://github.com/org/repo.git")).toBeInTheDocument();
    });

    // Fill in required fields
    fireEvent.change(screen.getByPlaceholderText("https://github.com/org/repo.git"), {
      target: { value: "https://github.com/test/repo.git" },
    });
    fireEvent.change(screen.getByPlaceholderText("my-project"), {
      target: { value: "test-repo" },
    });
    fireEvent.change(screen.getByPlaceholderText("org/my-project"), {
      target: { value: "test/repo" },
    });

    fireEvent.click(screen.getByText("Continue"));

    await waitFor(() => {
      expect(screen.getByText("Confirm Import")).toBeInTheDocument();
    });

    // Click back button
    const backButtons = document.querySelectorAll("button");
    const backButton = Array.from(backButtons).find(
      (btn) => btn.querySelector('svg path[d*="M15 19l-7-7 7-7"]')
    );
    expect(backButton).toBeTruthy();
    fireEvent.click(backButton!);

    await waitFor(() => {
      expect(screen.getByText("Manual Entry")).toBeInTheDocument();
      expect(screen.getByPlaceholderText("https://github.com/org/repo.git")).toBeInTheDocument();
    });
  });

  it("should go back from manual step to source step", async () => {
    render(
      <ImportRepositoryModal open={true} onClose={mockOnClose} onImported={mockOnImported} />
    );

    await waitFor(() => {
      expect(screen.getByText("Enter Manually")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Enter Manually"));

    await waitFor(() => {
      expect(screen.getByText("Manual Entry")).toBeInTheDocument();
    });

    // Click back button
    const backButtons = document.querySelectorAll("button");
    const backButton = Array.from(backButtons).find(
      (btn) => btn.querySelector('svg path[d*="M15 19l-7-7 7-7"]')
    );
    expect(backButton).toBeTruthy();
    fireEvent.click(backButton!);

    await waitFor(() => {
      expect(screen.getByText("Enter Manually")).toBeInTheDocument();
      expect(screen.getByText("My GitHub")).toBeInTheDocument();
    });
  });
});
