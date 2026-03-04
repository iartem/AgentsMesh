---
title: 'Building AgentsMesh with AgentsMesh: 52 Days of Harness Engineering by One Person'
excerpt: "One person, 52 days, 600 commits, 965,687 lines of code throughput. The most interesting part isn't the numbers — it's that the tool was built using the very practice it enables."
date: "2026-03-04"
author: "AgentsMesh Team"
category: "Insight"
readTime: 12
---

OpenAI recently published a piece describing how they used AI agents to produce over a million lines of code in five months. They call the practice **Harness Engineering**.

I started building **AgentsMesh** a little over 50 days ago. 52 days, 600 commits, 965,687 lines of code throughput, 356,220 lines of production code still standing. One person.

But the numbers aren't the point. The structure is: I used Harness Engineering to build a Harness Engineering tool. AgentsMesh is self-bootstrapped.

The repository is fully open source and the git history is public. Every number here is verifiable via git log.

## Your Engineering Environment Sets the Ceiling for Agent Output

52 days of real work taught me one thing clearly: agent output quality depends less on the agent itself and more on the engineering soil it works in.

**Layered architecture tells the agent where to make changes.** Strict **DDD** layering with 22 domain modules and clear boundaries. The agent knows data structures go in domain, business rules go in service, routing goes in handler. No guessing.

**Directory structure is the documentation.** Naming is aligned across the entire stack. Example: backend/internal/domain/loop/ maps directly to web/src/components/loops/. 16 domain modules mirror 1:1 with the service layer. The codebase is self-describing.

**Tech debt gets amplified exponentially by agents.** This was the most counterintuitive lesson of the entire project. A quick hack gets picked up by the agent and systematically replicated everywhere. A human knows "this is a landmine, step around it." An agent doesn't. Good engineering practices get amplified into consistently good code. Tech debt gets amplified into consistently bad code. I stopped multiple times specifically to pay down debt before continuing.

**Strong typing acts as a compile-time quality gate.** Go + TypeScript + Proto. Signature mismatch? Build fails. API contract out of date? TypeScript screams. The shorter the feedback loop, the higher the throughput.

**Four layers of feedback.** Compilation (1 second) to unit tests (5 minutes, 700+ tests) to e2e tests to CI pipeline. Latency increases at each layer, but so does coverage.

**Worktrees enable native parallelism.** dev.sh automatically calculates port offsets based on git worktree, with fully isolated environments. Multiple agents, multiple worktrees, zero conflicts.

**The codebase IS the agent's context.** No separate Context Engineering setup, no RAG pipeline, no memory files. **The repository is the context.** **Harness Engineering and traditional software engineering are the same discipline.**

## Cognitive Bandwidth Is a Real Engineering Constraint

I hit the wall on day 5. Three worktrees with three agents running simultaneously. When I added a fourth, decision quality dropped. The daily ceiling of roughly 50,000 lines wasn't a tooling limitation — it was a cognitive bandwidth limitation.

The breakthrough: trade direct control for scale. Let agents coordinate agents. That's how **Autopilot** mode was born.

## When Experimentation Is Cheap, Design-First Loses to Build-First

The Relay architecture wasn't designed on a whiteboard. It was forged in production. Three Pods hammered the Backend simultaneously, so I added Relay. Then I added intelligent aggregation. The entire arc from failure to fix took less than two days. In a traditional team, that's two weeks of architecture discussion.

**AI doesn't just speed up coding. It changes the cost structure of the entire engineering process.** When experiments are cheap, building and learning beats planning and predicting.

## Self-Bootstrapping as Validation

AgentsMesh was built with AgentsMesh. 52 days, 965,687 lines of code throughput, 356,220 lines of production code, 600 commits, one author. The commit history is the proof.

## Three Engineering Primitives

**Isolation** — Implemented as **Pods**. Each agent gets its own git worktree and sandbox. Isolation also means readiness: repo, skills, and MCP servers are all pre-configured.

**Decomposition** — **Tickets** map to one-shot work items. **Loops** map to recurring automated tasks. Work is broken down into units the agent can execute independently.

**Coordination** — No role-based abstractions. **Channels** handle group communication. **Bindings** handle point-to-point permission grants (**terminal:read**, **terminal:write**). Simple primitives, composable outcomes.

OpenAI calls their version Context Engineering, architectural constraints, and entropy management. Different names, same problems.

## Why Open Source

Harness Engineering is an engineering discipline, not a product feature. Open sourcing lets the community verify, evolve, and push beyond what one person can build. The code is on [GitHub](https://github.com/AgentsMesh/AgentsMesh).
