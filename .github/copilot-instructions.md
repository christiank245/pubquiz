Here is the fully updated instructions file, including a structured **Session Lifecycle & Workflow Protocol** to handle branch creation, task execution, pull requests, self-optimization, and session transitions.

---

# AI Agent Development Guidelines & System Prompt

## 1. Project Overview

You are building a pub quiz registration web application. The platform allows users to view upcoming pub quiz events and register their teams, while providing administrators with tools to manage quiz locations and event schedules.

---

## 2. Tech Stack & Architecture

| Component        | Technology             | Description                                                                                            |
| ---------------- | ---------------------- | ------------------------------------------------------------------------------------------------------ |
| **Backend & DB** | **PocketBase (Go)**    | Handles authentication, database storage, API endpoints, and serves the frontend.                      |
| **Templating**   | **Go `html/template**` | Native PocketBase Go templating to dynamically render HTML and reusable components.                    |
| **Frontend**     | **HTML5 + HTMX**       | Handles dynamic DOM updates, inline form submissions, and modal transitions without full page reloads. |
| **Styling**      | **Tailwind CSS**       | Utility-first CSS framework for layout, components, and responsive design.                             |
| **Build System** | **`Makefile` (Make)**  | Bundles assets, templates, and PocketBase into a single executable binary (`go build` with `embed`).   |
| **Testing**      | **Ginkgo + Gomega**    | BDD-style testing framework for writing Go unit and integration tests.                                 |

---

## 3. Core Features & User Flows

### A. Public / User Flow

- **Landing Page:** Display upcoming pub quiz event tiles showing date, time, pub name, max capacity, and a Google Maps link.
- **Team Registration:** Clicking an event tile opens a registration view/modal driven by HTMX.
- Required registration fields: **Email address**, **Team name**, and **Team size**.
- Enforce maximum capacity checks against the venue size.

### B. Admin Flow

- **Authentication:** Admin login using PocketBase's built-in auth system.
- **Location Management:** Create, read, update, and delete locations (Pub/Bar Name, Google Maps Link, Max Capacity).
- **Quiz Event Management:** Schedule pub quizzes by picking a date, time, and linking an existing location.

---

## 4. Work Sessions & Git Branching Protocol

### A. Strict Branching Rule

> [!CAUTION]
> **NEVER commit or push directly to `main` (or `master`).**
> Every task must be performed inside an active **Work Session** on a dedicated feature branch.

---

### B. Work Session Lifecycle

#### Step 1: Session Initialization

- At the beginning of a conversation or turn (if no active session exists), ask the user:

> _"Would you like to start a new Work Session?"_

- If **Yes**:

1. Fetch and checkout the latest `main` branch (`git checkout main && git pull`).
2. Prompt the user for a feature name or generate a descriptive one based on the task.
3. Create and switch to a new feature branch (`git checkout -b feature/<feature-name>`).

#### Step 2: Task Execution

- Verify you are on the correct feature branch using `git branch --show-current` before making any edits.
- Execute the requested task, ensuring high quality, Google Go style compliance, and passing Ginkgo tests.

#### Step 3: Task Completion Prompt

- Immediately after finishing a requested task, ask the user:

> _"The task is complete. Would you like to wrap up this Work Session, or continue working on this branch?"_

#### Step 4: Finishing a Session (PR & Continuous Self-Optimization)

If the user chooses to wrap up the session:

1. **Submit Changes & Open PR:**

- Stage, commit, and push changes to the remote feature branch.
- Create a Pull Request (PR) on GitHub against the default `main` branch using GitHub CLI (`gh pr create`) or by providing a formatted PR link/body.

2. **Reflect & Optimize Instructions:**

- Review all actions, friction points, errors, and workflows experienced during the session.
- Evaluate if this instructions file (`INSTRUCTIONS.md` / `AGENTS.md`) can be enhanced with new rules, edge cases, context, or technical guidelines.
- If improvements are identified:
- Draft the proposed enhancements clearly for the user.
- Ask: _"I noticed [insight/friction point] during this session. Would you like me to update the instructions file with these additions before we close out?"_
- **ONLY modify the instructions file after receiving explicit user confirmation.**

3. **Close Session:**

- Mark the session as complete.
- Prompt the user for the next session on the next interaction.

---

## 5. Design & Component Architecture

- **Reusable Components:** Organize Go templates logically into modular components (e.g., `header.html`, `quiz_card.html`, `registration_modal.html`).
- **HTMX Interactivity:** Use HTMX attributes (`hx-get`, `hx-post`, `hx-target`, `hx-swap`) for interactive updates (e.g., submitting registrations, opening modals, dynamic UI state changes).
- **Single Binary Packaging:** Utilize Go's `embed` package to embed templates and static Tailwind assets directly into the Go binary so the application runs standalone.
- **Quiz Admin UI Defaults:**
  - Hide system and auth collections from Quiz Admin navigation.
  - Use full-width layout for Quiz Admin pages.
  - Prefer human-readable relation labels (never raw relation IDs in overview tables when resolvable).
  - Use dropdowns for single relation fields.
  - Make table rows clickable to open edit modal; keep destructive actions explicit via separate controls (checkbox + delete button).
  - For collection listing order, prefer query-level sorting. If sorting depends on related collection fields (for example registrations sorted by `quiz_dates.scheduled_at`), prefer join-based ordering in SQL instead of in-memory post-sorting when feasible.

---

## 6. Coding & Style Guidelines

### Go Code Style

- Strict adherence to the **[Google Go Style Guide](https://google.github.io/styleguide/go/)**.
- **Naming Conventions:** Use standard Go conventions (`camelCase`, `MixedCaps`). Avoid vague variable names.
- **Error Handling:** Always handle errors explicitly. Wrap context when returning errors (`fmt.Errorf("failed to register team: %w", err)`).
- **Concurrency:** Keep goroutine lifetimes clean and manageable if used.

### Testing Standard

- Use the **Ginkgo BDD framework** (paired with **Gomega** matchers) for all unit and integration tests.
- Organize specs cleanly into standard Ginkgo blocks (`Describe`, `Context`, `It`).
- Ensure test coverage includes:
- PocketBase custom routes and API endpoints.
- Capacity limit validations during team registration.
- Admin authorization and CRUD operations.

---

## 7. Development & Build Requirements

### Toolchain Reliability Rule

- When managing Tailwind via `mise`, always pin an explicit CLI version in `.mise.toml` and avoid unpinned upgrades.
- If migrating Tailwind major versions (e.g., v3 -> v4), explicitly update Tailwind config/input syntax and verify generated CSS includes expected base/utilities before closing the task.
- When using `gh` CLI commands with `--body` or other string arguments via shell, avoid Markdown backticks in inline command strings (bash executes them). Use plain text or escape backticks.
- Pull request descriptions must not include a standalone "Verification" section, because CI already runs the canonical checks.

Provide a `Makefile` containing at least the following standard targets:

```makefile
# Build Tailwind CSS, embed assets, and compile the single PocketBase binary
build:
	@echo "Building application..."

# Run local development server with dynamic reloading
dev:
	@echo "Starting development server..."

# Run Ginkgo test suite
test:
	ginkgo -r ./...

```
