import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { AutopilotPanelContent } from "../AutopilotPanelContent";
import type { AutopilotController, AutopilotThinking } from "@/stores/autopilot";

// Mock data
let mockAutopilotController: AutopilotController | undefined = undefined;
let mockThinking: AutopilotThinking | null = null;
const mockPauseAutopilotController = vi.fn();
const mockResumeAutopilotController = vi.fn();
const mockStopAutopilotController = vi.fn();
const mockTakeoverAutopilotController = vi.fn();
const mockHandbackAutopilotController = vi.fn();
const mockFetchIterations = vi.fn();
const mockIterations: Record<string, unknown[]> = {};

vi.mock("@/stores/autopilot", () => ({
  useAutopilotStore: () => ({
    getAutopilotControllerByPodKey: () => mockAutopilotController,
    getThinking: () => mockThinking,
    pauseAutopilotController: mockPauseAutopilotController,
    resumeAutopilotController: mockResumeAutopilotController,
    stopAutopilotController: mockStopAutopilotController,
    takeoverAutopilotController: mockTakeoverAutopilotController,
    handbackAutopilotController: mockHandbackAutopilotController,
    fetchIterations: mockFetchIterations,
    iterations: mockIterations,
  }),
}));

// Helper to create mock controller
const createMockController = (
  phase: AutopilotController["phase"] = "running"
): AutopilotController => ({
  autopilot_controller_key: "test-key-123",
  pod_key: "pod-123",
  phase,
  current_iteration: 3,
  max_iterations: 10,
  user_takeover: phase === "user_takeover",
  circuit_breaker: {
    state: "closed",
    reason: undefined,
  },
  task_description: "Test task",
  created_at: new Date().toISOString(),
  updated_at: new Date().toISOString(),
});

// Helper to create mock thinking
const createMockThinking = (overrides: Partial<AutopilotThinking> = {}): AutopilotThinking => ({
  autopilot_controller_key: "test-key-123",
  iteration: 3,
  decision_type: "CONTINUE",
  reasoning: "Processing the task...",
  confidence: 0.85,
  action: null,
  progress: null,
  help_request: null,
  ...overrides,
});

describe("AutopilotPanelContent", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAutopilotController = undefined;
    mockThinking = null;
    Object.keys(mockIterations).forEach((key) => delete mockIterations[key]);
  });

  describe("no pod selected", () => {
    it("should show select pod message when podKey is null", () => {
      render(<AutopilotPanelContent podKey={null} />);
      expect(screen.getByText("Select a Pod first")).toBeInTheDocument();
    });
  });

  describe("no autopilot for pod", () => {
    it("should show no autopilot message when controller not found", () => {
      mockAutopilotController = undefined;
      render(<AutopilotPanelContent podKey="pod-123" />);
      expect(screen.getByText("No active Autopilot for this Pod")).toBeInTheDocument();
      expect(screen.getByText("Click the Bot icon in terminal header to start")).toBeInTheDocument();
    });
  });

  describe("with active autopilot", () => {
    beforeEach(() => {
      mockAutopilotController = createMockController("running");
    });

    it("should display phase status", () => {
      render(<AutopilotPanelContent podKey="pod-123" />);
      expect(screen.getByText("Running")).toBeInTheDocument();
    });

    it("should display progress", () => {
      render(<AutopilotPanelContent podKey="pod-123" />);
      expect(screen.getByText("3/10")).toBeInTheDocument();
    });

    it("should show control buttons", () => {
      render(<AutopilotPanelContent podKey="pod-123" />);
      expect(screen.getByText("Pause")).toBeInTheDocument();
      expect(screen.getByText("Takeover")).toBeInTheDocument();
      expect(screen.getByText("Stop")).toBeInTheDocument();
    });
  });

  describe("tabs", () => {
    beforeEach(() => {
      mockAutopilotController = createMockController("running");
    });

    it("should show three tabs", () => {
      render(<AutopilotPanelContent podKey="pod-123" />);
      expect(screen.getByText("Thinking")).toBeInTheDocument();
      expect(screen.getByText("Progress")).toBeInTheDocument();
      expect(screen.getByText("History")).toBeInTheDocument();
    });

    it("should default to Thinking tab", () => {
      render(<AutopilotPanelContent podKey="pod-123" />);
      // Thinking tab content should be visible
      expect(screen.getByText("Waiting for Control Agent...")).toBeInTheDocument();
    });

    it("should switch to Progress tab when clicked", () => {
      render(<AutopilotPanelContent podKey="pod-123" />);
      fireEvent.click(screen.getByText("Progress"));
      expect(screen.getByText("No progress data available")).toBeInTheDocument();
    });

    it("should switch to History tab when clicked", () => {
      render(<AutopilotPanelContent podKey="pod-123" />);
      fireEvent.click(screen.getByText("History"));
      expect(screen.getByText("No iterations yet")).toBeInTheDocument();
    });
  });

  describe("thinking tab content", () => {
    beforeEach(() => {
      mockAutopilotController = createMockController("running");
    });

    it("should display thinking data when available", () => {
      mockThinking = createMockThinking({
        reasoning: "Analyzing the code structure",
        decision_type: "CONTINUE",
      });
      render(<AutopilotPanelContent podKey="pod-123" />);
      expect(screen.getByText("Analyzing the code structure")).toBeInTheDocument();
      expect(screen.getByText("Continue")).toBeInTheDocument();
    });
  });

  describe("auto switch to thinking on need_help", () => {
    beforeEach(() => {
      mockAutopilotController = createMockController("running");
    });

    it("should switch to thinking tab when decision is need_help", () => {
      const { rerender } = render(<AutopilotPanelContent podKey="pod-123" />);

      // Switch to Progress tab
      fireEvent.click(screen.getByText("Progress"));
      expect(screen.getByText("No progress data available")).toBeInTheDocument();

      // Update thinking to need_help
      mockThinking = createMockThinking({ decision_type: "NEED_HUMAN_HELP" });
      rerender(<AutopilotPanelContent podKey="pod-123" />);

      // Should auto-switch to Thinking tab and show help needed content
      expect(screen.getByText("Need Help")).toBeInTheDocument();
    });
  });

  describe("phase configurations", () => {
    it.each([
      ["initializing", "Initializing"],
      ["running", "Running"],
      ["paused", "Paused"],
      ["user_takeover", "User Control"],
      ["waiting_approval", "Waiting Approval"],
      ["completed", "Completed"],
      ["failed", "Failed"],
      ["stopped", "Stopped"],
      ["max_iterations", "Max Iterations"],
    ] as const)("should display correct label for %s phase", (phase, expectedLabel) => {
      mockAutopilotController = createMockController(phase);
      render(<AutopilotPanelContent podKey="pod-123" />);
      expect(screen.getByText(expectedLabel)).toBeInTheDocument();
    });
  });

  describe("className prop", () => {
    it("should apply custom className", () => {
      render(<AutopilotPanelContent podKey={null} className="custom-class" />);
      const container = screen.getByText("Select a Pod first").closest("div");
      expect(container).toHaveClass("custom-class");
    });
  });
});
