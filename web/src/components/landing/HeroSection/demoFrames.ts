import type { DemoFrame } from "./types";

const TICKETS = {
  auth: { id: "auth", title: "Auth API", agent: "Claude Code", color: "primary" },
  payment: { id: "payment", title: "Payment Flow", agent: "Codex CLI", color: "blue" },
  tests: { id: "tests", title: "E2E Tests", agent: "Aider", color: "purple" },
  docs: { id: "docs", title: "API Docs", agent: "Gemini CLI", color: "green" },
};

export function getDemoFrames(): DemoFrame[] {
  return [
    // Phase 0: Board with backlog tickets
    {
      time: 0,
      tickets: [
        { ...TICKETS.auth, status: "backlog" },
        { ...TICKETS.payment, status: "backlog" },
        { ...TICKETS.tests, status: "backlog" },
        { ...TICKETS.docs, status: "done" },
      ],
      terminals: [],
      showTerminals: false,
    },
    // Phase 1: Auth "Execute" button clicked
    {
      time: 1500,
      tickets: [
        { ...TICKETS.auth, status: "backlog", executing: true },
        { ...TICKETS.payment, status: "backlog" },
        { ...TICKETS.tests, status: "backlog" },
        { ...TICKETS.docs, status: "done" },
      ],
      terminals: [],
      showTerminals: false,
    },
    // Phase 2: Auth moves to in_progress, terminal spawns
    {
      time: 2500,
      tickets: [
        { ...TICKETS.auth, status: "in_progress" },
        { ...TICKETS.payment, status: "backlog" },
        { ...TICKETS.tests, status: "backlog" },
        { ...TICKETS.docs, status: "done" },
      ],
      terminals: [
        {
          id: "t1", agent: "Claude Code", ticketId: "auth", active: true,
          lines: [
            { text: "$ claude --pod auth-dev", type: "command" },
            { text: "Analyzing ticket: Auth API...", type: "info" },
          ],
        },
      ],
      showTerminals: true,
    },
    // Phase 3: Payment "Execute" button clicked
    {
      time: 4000,
      tickets: [
        { ...TICKETS.auth, status: "in_progress" },
        { ...TICKETS.payment, status: "backlog", executing: true },
        { ...TICKETS.tests, status: "backlog" },
        { ...TICKETS.docs, status: "done" },
      ],
      terminals: [
        {
          id: "t1", agent: "Claude Code", ticketId: "auth", active: true,
          lines: [
            { text: "$ claude --pod auth-dev", type: "command" },
            { text: "Writing src/auth/handler.ts", type: "output" },
            { text: "Writing src/auth/oauth.ts", type: "output" },
          ],
        },
      ],
      showTerminals: true,
    },
    // Phase 4: Payment moves to in_progress, second terminal
    {
      time: 5000,
      tickets: [
        { ...TICKETS.auth, status: "in_progress" },
        { ...TICKETS.payment, status: "in_progress" },
        { ...TICKETS.tests, status: "backlog" },
        { ...TICKETS.docs, status: "done" },
      ],
      terminals: [
        {
          id: "t1", agent: "Claude Code", ticketId: "auth", active: true,
          lines: [
            { text: "Writing src/auth/handler.ts", type: "output" },
            { text: "Writing src/auth/oauth.ts", type: "output" },
            { text: "Running tests...", type: "info" },
            { text: "✓ 12 tests passed", type: "success" },
          ],
        },
        {
          id: "t2", agent: "Codex CLI", ticketId: "payment", active: true,
          lines: [
            { text: "$ codex --pod payment-dev", type: "command" },
            { text: "Analyzing ticket: Payment Flow...", type: "info" },
          ],
        },
      ],
      showTerminals: true,
    },
    // Phase 5: Both agents working
    {
      time: 7500,
      tickets: [
        { ...TICKETS.auth, status: "in_progress" },
        { ...TICKETS.payment, status: "in_progress" },
        { ...TICKETS.tests, status: "backlog" },
        { ...TICKETS.docs, status: "done" },
      ],
      terminals: [
        {
          id: "t1", agent: "Claude Code", ticketId: "auth", active: true,
          lines: [
            { text: "Running tests...", type: "info" },
            { text: "✓ 12 tests passed", type: "success" },
            { text: "Creating merge request...", type: "info" },
            { text: "✓ MR !41 created", type: "success" },
          ],
        },
        {
          id: "t2", agent: "Codex CLI", ticketId: "payment", active: true,
          lines: [
            { text: "Writing src/payment/stripe.ts", type: "output" },
            { text: "Writing src/payment/webhook.ts", type: "output" },
            { text: "Running tests...", type: "info" },
          ],
        },
      ],
      showTerminals: true,
    },
    // Phase 6: Auth done, Tests "Execute" clicked
    {
      time: 9500,
      tickets: [
        { ...TICKETS.auth, status: "done" },
        { ...TICKETS.payment, status: "in_progress" },
        { ...TICKETS.tests, status: "backlog", executing: true },
        { ...TICKETS.docs, status: "done" },
      ],
      terminals: [
        {
          id: "t2", agent: "Codex CLI", ticketId: "payment", active: true,
          lines: [
            { text: "✓ 8 tests passed", type: "success" },
            { text: "Creating merge request...", type: "info" },
            { text: "✓ MR !42 created", type: "success" },
          ],
        },
      ],
      showTerminals: true,
    },
    // Phase 7: Tests in_progress, new terminal
    {
      time: 10500,
      tickets: [
        { ...TICKETS.auth, status: "done" },
        { ...TICKETS.payment, status: "done" },
        { ...TICKETS.tests, status: "in_progress" },
        { ...TICKETS.docs, status: "done" },
      ],
      terminals: [
        {
          id: "t3", agent: "Aider", ticketId: "tests", active: true,
          lines: [
            { text: "$ aider --pod tests-dev", type: "command" },
            { text: "Analyzing ticket: E2E Tests...", type: "info" },
            { text: "Writing tests/e2e/auth.spec.ts", type: "output" },
          ],
        },
      ],
      showTerminals: true,
    },
    // Phase 8: All done
    {
      time: 13000,
      tickets: [
        { ...TICKETS.auth, status: "done" },
        { ...TICKETS.payment, status: "done" },
        { ...TICKETS.tests, status: "done" },
        { ...TICKETS.docs, status: "done" },
      ],
      terminals: [],
      showTerminals: false,
    },
  ];
}
