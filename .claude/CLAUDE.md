## Wiki Knowledge Base
Path: ~/claude-obsidian/wiki

When you need context not already in this project:
1. Read wiki/hot.md first (recent context cache)
2. If not enough, read wiki/index.md
3. If you need domain details, read the relevant domain sub-index
4. Only then drill into specific wiki pages

Do NOT read the wiki for general coding questions or tasks unrelated to this project.

## Workflow Rules

### Branch / Worktree Discipline
- Never commit directly to `main`. Every change ships via PR.
- Long task (>1 file edit, >30min, or audit-driven PR): use `git worktree add` under `../goscratch-<branch>` and a feature branch (`feat/...`, `fix/...`, `refactor/...`, `docs/...`).
- Quick edit (single typo, doc tweak, single-line fix): branch in main checkout is OK, still PR.
- Branch name = the slug from the active scope file or issue when doing tracked audit work (historical v1.x `docs/audit/` files are archived on branch `archive/audit-v1.x`).

### Dispatched Agents
- Sub-agents launched via `Agent` tool MUST run in `isolation: "worktree"`.
- Sub-agent prompt MUST include: branch name, scope file or issue (if any), and "do not commit / push / open PR — main thread reviews then ships".
- Main thread reviews the worktree diff before any merge.

### PR Auto-Open + Task Marking
- After committing a completed task, immediately push the branch and open the PR via `gh pr create`. No waiting for confirmation.
- PR title = the task subject. Body links audit doc / issue + finding IDs (when applicable).
- Same commit (or follow-up commit on the same branch) MUST tick the task's checkboxes in its task/scope file when present (never fail if `docs/audit/` is absent; v1.x files live on `archive/audit-v1.x`).
- Also update `TaskList` (TaskUpdate status=completed) so in-conversation tracking matches the scope file/issue.
- Never claim a task done in chat without ticking its file (when one exists).

### PR Acceptance (when using an audit punch-list)
1. Tests added or updated for changed behavior.
2. `make lint test` clean.
3. `docs/features/<feature>.md` updated if behavior changed.
4. `CHANGELOG.md` `[Unreleased]` entry.
5. Security PRs: include "operator must change after upgrade" note in PR body.

### Commit / PR Mechanics
- Conventional commits: `type(scope): subject` — types: feat, fix, refactor, docs, chore, test.
- One PR = one coherent finding set. No mixed scope.
- PR body links the scope doc / issue + finding IDs it closes.
- No force-push to shared branches. No `--no-verify`.

### Out of Scope Guard
- When a PR has an explicit "Out of scope" section in its scope file or issue, do not exceed it. Record new findings in the tracking issue or punch-list, not into the current PR.

### Agent Team Mode
Use Claude Code agent teams (`CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1`, `teammateMode=tmux`) when ≥2 independent tasks/rows have non-overlapping file scope and can ship in parallel. Otherwise use single-session or `Agent` tool with `isolation: "worktree"`.

**Team shape (conservative + adversarial):**
- **Lead** (this session): never edits source. Reads punch-list/issues, plans slice, spawns teammates, reviews diffs, opens PRs, ticks scope files.
- **implementer-A**: owns one PR scope (scope file or issue), one file-set.
- **implementer-B**: owns a different PR scope, file-set must not overlap implementer-A.
- **tester**: owns `*_test.go` for both PRs in flight. Writes/updates tests only, never impl files.
- **reviewer**: read-only. Audits each implementer's diff after `make lint test` green, before lead opens PR.
- **devils-advocate**: read-only. Challenges plans, surfaces failure modes the lead is anchoring past, cross-checks reviewer findings on security/lifecycle/schema PRs.

Total: 5 teammates + lead. Do not scale up unless punch-list has 4+ truly independent rows.

**Spawn rules:**
- Implementer/tester teammates: spawn with `isolation: "worktree"` semantics — pass branch name + scope file/issue in spawn prompt. Worktree under `../goscratch-<branch>`.
- Spawn prompt MUST include: branch name, scope file/issue, file-set boundary ("only edit files X/Y/Z"), "do not push or open PR — lead handles ship".
- Reviewer + devils-advocate: read-only, no worktree needed.
- Before spawning two implementers, lead diffs their scope file lists. If overlap, serialize (one at a time) instead of parallelizing.

**Plan-mode (lead judgment per spawn):**
Trigger plan-mode when implementer's scope touches any of:
- Auth/authz/crypto code (`internal/adapter/casbin`, `internal/platform/auth`, JWT, token/session).
- Schema/migration files (`migrations/`, DDL).
- Process lifecycle (`internal/platform/app`, shutdown, signal handling, worker drain).
- Block-ship class findings.

Otherwise no plan-mode — trust implementer + reviewer + tester loop. Devils-advocate runs always for security/lifecycle/schema.

**Task list discipline:**
- Lead creates one shared task per PR scope, one for tests, one for review pass. Implementers self-claim. Tester claims after impl marks ready-for-test. Reviewer claims after `make lint test` green.
- Hooks (when wired): `TeammateIdle` exit-2 nudge if PR not pushed; `TaskCompleted` exit-2 if `make lint test` not green.

**Cleanup:**
- Lead opens all PRs, ticks active scope file/issue checkboxes when present (v1.x `docs/audit/` lives on `archive/audit-v1.x`), updates `TaskList`, then runs team cleanup. Never let a teammate self-cleanup.

**When NOT to use teams:**
- Single PR in flight, single file-set → single session.
- Sequential dependencies (PR-B blocks on PR-A) → single session or strict serialize.
- Quick edits (typo, single-line, doc tweak) → single session.
- Investigation-only (no edits) → consider 5x devils-advocate debate pattern instead of impl team.