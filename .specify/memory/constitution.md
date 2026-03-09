# SynapseX Constitution

## Project Mission
- SynapseX is a remote AI CLI control gateway for professional developers.
- The current product goal is to map chat messages into controlled remote execution against the project-supported CLI backend.
- The current delivery focus is to make remote CLI control stable, session-safe, observable, and operable on Linux.

## Product Scope

### Current In-Scope
- Remote control of the supported CLI backend through chat-driven interaction.
- Session creation, resume, switching, cancellation, and isolation.
- Streaming or chunked output return with ordering guarantees.
- Discord Bot and Telegram Bot as the primary channels.
- Linux deployment, configuration, logging, and basic health checks.
- Scoped Skill Registry + Intent Router for Claude Code compatible skills, with explicit governance and bounded routing behavior.

### Current Out-of-Scope
- Multi-model orchestration.
- Additional model families beyond the currently supported primary backend.
- Generic plugin marketplaces, arbitrary tool-runtime frameworks, and RAG platforms.
- Multi-tenant access control or full audit systems.
- Web control panel.
- Unplanned external platform integrations.

## Core Principles

### I. Session-First Architecture
- `Session` is the primary execution and context unit.
- Conversation isolation must be implemented through `Session`, not through `Agent`.
- A new window, thread, or equivalent interaction container should default to creating or binding a `Session`.
- Concurrent writes to the same `Session` are forbidden.
- The same `Session` must execute serially.

### II. Agent as Runtime Template
- `Agent` exists to define runtime defaults, not to replace `Session`.
- `Agent` may define default workspace, backend, profile, tool policy, and system prompt.
- Multiple `Session` instances may share one `Agent`.
- No feature may require `Agent` as a prerequisite when `Session` alone satisfies the use case.
- If both `Session` and `Agent` provide values, `Session` overrides runtime defaults for that `Session`.

### III. Direct Execution Before Heavy Orchestration
- The default execution model is the shortest viable chain:
  - `Router -> Session Manager -> Backend Adapter`
- New features must integrate with the direct execution chain before introducing additional orchestration layers.
- Heavy gateway, plugin, node, or distributed coordination layers must not be introduced unless current requirements cannot be satisfied by the direct execution model.
- Complexity must be justified by an explicit need in `docs/plans/`.

### IV. Stable Boundaries and Replaceable Adapters
- Routing, session management, backend execution, and channel delivery must remain separate concerns.
- `Router`, `Session Manager`, `Backend Adapter`, and channel adapters must be independently understandable and replaceable.
- Backend-specific behavior must stay behind adapter boundaries.
- Channel-specific behavior must stay behind channel adapter boundaries.
- New integrations must extend the existing boundaries rather than bypass them.

### V. Docs Drive Scope
- `docs/plans/` defines delivery scope by phase.
- `docs/features/` defines feature-level architecture and implementation.
- `docs/reference/` is research-only and must not be used as the source of product truth.
- Any substantial feature change must update the relevant feature and plan documents before or alongside implementation.
- When a feature has moved into `docs/features/`, old locations are compatibility entry points only.
- Project-owned plans, feature docs, and generated specification artifacts must use Chinese as the default documentation language unless a phase plan explicitly states otherwise.

### VI. Reference Is Input, Never Authority
- `docs/reference/` exists only for local research, comparison, and temporary architectural inspiration.
- Reference materials may inform design thinking, but they must be translated into SynapseX-native decisions before they affect product code or official docs.
- No implementation may copy product rules, naming, architecture, or behavior directly from reference materials without first defining the equivalent rule in `docs/features/` or `docs/plans/`.
- Reference materials must never override this constitution, feature docs, phase plans, or stable project docs.

### VII. Go and DDD by Default
- Go is the default implementation language for project-owned backend and service code unless a phase plan explicitly approves an exception.
- Core business behavior must be organized using domain-driven design boundaries.
- Domain rules must be expressed in domain models and domain services, not scattered across handlers, adapters, or infrastructure code.
- Application flow may orchestrate use cases, but it must not absorb domain decision logic that belongs inside the domain layer.
- Infrastructure concerns must stay outside domain logic and be accessed through stable interfaces.

## System Architecture Rules

### Implementation Stack Rules
- Go is the default language for implementation of core runtime modules, adapters, services, and supporting backend logic.
- Any non-Go implementation for project-owned runtime code must be explicitly justified in `docs/plans/` or the relevant feature document.
- Tooling, templates, or local scripts may use other languages when appropriate, but they must not redefine core product architecture.

### Required Core Modules
- `Channel Adapter`
- `Router`
- `Session Manager`
- `Backend Adapter`
- `Output Streamer` or equivalent output delivery layer
- `Config & Logging`

### Required Responsibilities
- `Channel Adapter` receives channel events, normalizes message payloads, and sends output back to the channel.
- `Router` resolves whether a request creates a new session, resumes an existing session, or executes within the bound session.
- `Session Manager` owns session creation, lookup, locking, state, and backend session mapping.
- `Backend Adapter` owns backend-specific session creation, execution, resume, timeout, and cancellation semantics.
- `Output Streamer` owns chunking, order guarantees, markdown/code block formatting, and retry-safe delivery behavior.

### DDD Layering Rules
- The codebase should be structured around these logical layers:
  - Domain
  - Application
  - Infrastructure
  - Interface / Delivery
- Domain layer responsibilities:
  - session rules
  - execution invariants
  - conversation identity rules
  - core business constraints
- Application layer responsibilities:
  - use case orchestration
  - transaction-like coordination
  - command handling flow
- Infrastructure layer responsibilities:
  - backend execution
  - persistence
  - logging integrations
  - external service access
- Interface / Delivery layer responsibilities:
  - channel adapters
  - transport normalization
  - user-facing request/response mapping
- No layer may depend inward on a more external layer for domain decisions.
- Domain logic must not depend on channel-specific structures or infrastructure-specific implementations.

### Session Record Requirements
- `Session` records must support:
  - `id`
  - `window_id`
  - `agent_id`
  - `backend`
  - `backend_session_id`
  - `status`
  - `cwd`
  - `last_used_at`
  - `lock_token`
- `agent_id` may be nullable during transition, but new feature work must preserve compatibility with future multi-agent support.
- `backend_session_id` must be persisted whenever the backend provides a native session identifier.

### Execution Record Requirements
- An execution or job record should capture:
  - `session_id`
  - command or prompt summary
  - `start_at`
  - `end_at`
  - duration
  - exit status or failure kind
  - output chunk summary or equivalent traceability fields

## Backend Rules

### Supported Backend Policy
- The currently supported primary execution backend is the project’s CLI-first backend.
- Product behavior, architecture, and tests must optimize for the current primary backend first.
- Future backend expansion must happen through `Backend Adapter`, not through ad hoc branching across the codebase.

### Execution Safety
- The backend may only execute within allowed work directories.
- Work directory policy must default to configured root or cwd allowlists.
- Unsupported system command passthrough is forbidden.
- Timeout and cancellation must be explicit behaviors, not best-effort afterthoughts.

### Output Behavior
- Output must preserve order.
- Output may be true streaming or simulated chunked streaming, but user-facing delivery must remain understandable and stable.
- Long output must be chunked according to channel limits.
- Code blocks and basic markdown formatting must be preserved when possible.

## Channel Rules

### Supported Channels
- Discord Bot
- Telegram Bot

### Shared Channel Semantics
- Each channel must normalize inbound messages into a unified internal shape:
  - `conversation_id`
  - `user_id`
  - `text`
  - optional `reply_to`
  - optional `attachments`
- Each channel adapter must expose:
  - `send_text`
  - `send_error`
- Send failures must be retried up to a bounded limit and logged.
- Output ordering must be preserved by the output delivery layer.

### Conversation Mapping
- Private messages map to an isolated conversation.
- Group threads or topic/thread equivalents map to isolated conversations.
- Conversation identity must preserve channel isolation.
- The canonical conversation identity rule is:
  - `{channel}:{guild?}:{thread?}:{user}`

### Discord Rules
- Direct messages may trigger directly.
- Group messages require an explicit bot mention.
- Threads map to independent sessions.
- Maximum outbound message size is 2000 characters.

### Telegram Rules
- Direct messages may trigger directly.
- Group messages require a bot mention or a command.
- Topic or thread-like structures map to independent sessions when available.
- Maximum outbound message size is 4096 characters.

### Control Commands
- The core command set is:
  - `/new`
  - `/resume`
  - `/list`
  - `/cancel`
- Natural language requests route to backend execution.
- Command semantics must remain consistent across supported channels unless a documented channel limitation requires deviation.

## Performance and Reliability Rules

### Execution Limits
- A single execution must default to a hard upper bound of 10 minutes unless a phase plan explicitly changes it.
- The target concurrency baseline is 5 to 20 concurrent sessions.
- Different sessions may execute concurrently.
- The same session must remain serialized.

### Reliability Baseline
- Timeout must release locks.
- Cancellation must release locks.
- Partial output delivery failures must be logged and surfaced as partial-failure states when necessary.
- The system must fail closed on backend unavailability, with explicit user-visible diagnostics.

## Security and Permissions Rules

### Minimum Security Baseline
- Only configured channel tokens and approved channel contexts may trigger execution.
- Sensitive configuration values such as tokens and secrets must never be echoed back in user-visible errors.
- Logs must be sanitized for tokens, secrets, and sensitive paths where appropriate.
- Lightweight execution summaries and timestamps must be retained for troubleshooting.

### Command and Scope Restrictions
- The product must only execute supported backend operations through approved adapters.
- No feature may introduce arbitrary shell passthrough under the guise of backend control.
- Scope restrictions for workspace and session ownership must remain enforceable through configuration.

## Operations and Deployment Rules

### Deployment Baseline
- Linux is the primary supported deployment target.
- Runtime configuration must be supported through environment files and/or YAML.
- Process supervision should assume a single active instance unless clustering is explicitly designed and documented.

### Observability Baseline
- Structured JSON logging is required.
- Core log fields should include:
  - timestamp
  - level
  - session_id
  - channel
  - command or request summary
  - duration
  - exit code or failure kind
- Health checks must cover:
  - process liveness
  - backend probe viability

### Failure Handling Baseline
- Backend unavailable:
  - fail fast
  - return an actionable error
- Stuck locks:
  - force release after timeout
  - log an abnormal event
- Channel send failure:
  - bounded retry
  - log failure
  - indicate possible partial output loss when necessary

## Delivery Phases

### Phase 1
- Stable direct execution.
- Single-window baseline behavior.
- Session lifecycle and safe backend invocation.

### Phase 2
- Multi-window, multi-session behavior becomes the default interaction model.
- Window-to-session binding is treated as first-class product behavior.

### Phase 3
- Lightweight multi-agent support is introduced.
- `Agent` remains a runtime template, not a replacement for session semantics.

### Phase 4
- Stability, operations hardening, persistence upgrades, and additional extensibility.

### Phase Discipline
- Work that belongs to a later phase must not be pulled forward without updating the relevant phase plan.
- Feature scope expansion must be visible in `docs/plans/` before it is treated as an accepted project direction.

## Documentation Workflow

### Spec Inputs
- When generating or updating specs, use inputs in this order:
  1. This constitution
  2. Relevant files in `docs/plans/`
  3. Relevant files in `docs/features/`
  4. Supporting files in:
     - `docs/01_concepts/`
     - `docs/02_architecture/`
     - `docs/08_permissions/`
     - `docs/09_channels/`
     - `docs/11_operations/`
- Do not use `docs/reference/` as the primary source for official specs.
- `docs/reference/` may be consulted only after scope and feature rules are already established by project-owned documents.

### Specification Language Rules
- All generated specification documents under `specs/` must be written in Chinese by default.
- Specification checklists generated for project-owned specs must also be written in Chinese.
- If a tool or template starts in another language, the final committed project artifact must be translated into Chinese before it is considered valid.
- Feature, phase, and implementation specs are only considered aligned when their user-facing document content is readable without relying on English placeholders.

### Reference Usage Restrictions
- `docs/reference/` is allowed for:
  - implementation comparison
  - architectural pattern study
  - local behavior verification during research
- `docs/reference/` is not allowed as:
  - the canonical source for product requirements
  - the canonical source for architecture decisions
  - the direct source for generated specs
  - the only source used to justify implementation behavior
- If reference materials influence a design, the resulting rule must be restated in SynapseX-owned documentation before implementation is considered aligned.
- Official docs must not rely on external project names, external file paths, or reference-specific terminology unless the document is explicitly marked as research.
- `docs/reference/` should remain excluded from normal product governance and should not be treated as part of the versioned product specification surface.

### Change Workflow
- For a new feature:
  1. Confirm the active phase in `docs/plans/`
  2. Add or update the feature entry in `docs/features/`
  3. Implement within existing architectural boundaries
  4. Update task and milestone summaries only after feature docs are aligned
- For an architectural change:
  1. Update the relevant feature architecture doc
  2. Confirm compatibility with `Session-First Architecture`
  3. Confirm that direct execution remains viable unless explicitly superseded
  4. Update the phase plan if the change affects scope or sequencing
  5. Confirm that Go-first and DDD layer boundaries remain intact

### Quality Gates
- No change may weaken session isolation semantics.
- No change may bypass the `Session Manager` for stateful execution.
- No change may bypass the `Backend Adapter` for backend-specific execution behavior.
- No change may couple feature logic directly to research materials in `docs/reference/`.
- Feature work must remain understandable from docs alone without reading local research snapshots.
- If removing `docs/reference/` would make a feature spec unclear, the feature docs are incomplete and must be expanded before implementation proceeds.
- No change may move domain rules into channel handlers, transport adapters, or infrastructure code for convenience.
- No change may introduce a new runtime module in a non-Go language without explicit documented justification.

## Governance
- This constitution supersedes ad hoc local conventions for planning, feature architecture, and implementation scope.
- If a document conflicts with this constitution, the constitution wins until the document is updated.
- Amendments require:
  1. Updating this file
  2. Updating affected files in `docs/plans/` or `docs/features/`
  3. A clear reason for the change in architecture, product scope, or process
- All future specs and implementation plans should be checked against:
  - Session-first behavior
  - Agent-as-template behavior
  - Direct execution default
  - Go-first implementation policy
  - DDD layer boundaries
  - Channel and output guarantees
  - Security and observability baselines
  - Docs as the source of scope

**Version**: 1.2.0 | **Ratified**: 2026-03-03 | **Last Amended**: 2026-03-03
