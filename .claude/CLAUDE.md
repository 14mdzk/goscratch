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
- Branch name = the slug from `docs/audit/punch-list.md` when doing audit work.

### Dispatched Agents
- Sub-agents launched via `Agent` tool MUST run in `isolation: "worktree"`.
- Sub-agent prompt MUST include: branch name, scope file (e.g. `docs/audit/pr-XX-*.md`), and "do not commit / push / open PR — main thread reviews then ships".
- Main thread reviews the worktree diff before any merge.

### PR Auto-Open + Task Marking
- After committing a completed task, immediately push the branch and open the PR via `gh pr create`. No waiting for confirmation.
- PR title = the task subject. Body links audit doc + finding IDs.
- Same commit (or follow-up commit on the same branch) MUST tick the task's checkboxes in its task file (`docs/audit/pr-NN-*.md`) and update the matching row in `docs/audit/punch-list.md` (e.g., add a status column or strikethrough).
- Also update `TaskList` (TaskUpdate status=completed) so in-conversation tracking matches the audit file.
- Never claim a task done in chat without ticking its file.

### PR Acceptance (per audit punch-list)
1. Tests added or updated for changed behavior.
2. `make lint test` clean.
3. `docs/features/<feature>.md` updated if behavior changed.
4. `CHANGELOG.md` `[Unreleased]` entry.
5. Security PRs (#2, #3, #5, #9 in current punch-list): include "operator must change after upgrade" note in PR body.

### Commit / PR Mechanics
- Conventional commits: `type(scope): subject` — types: feat, fix, refactor, docs, chore, test.
- One PR = one coherent finding set. No mixed scope.
- PR body links the audit doc + finding IDs it closes.
- No force-push to shared branches. No `--no-verify`.

### Out of Scope Guard
- Each PR has an explicit "Out of scope" section in its task file. Do not exceed it. New findings go to `docs/audit/punch-list.md` as a new row, not into the current PR.

### Agent Team Mode
Use Claude Code agent teams (`CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1`, `teammateMode=tmux`) when ≥2 punch-list rows have non-overlapping file scope and can ship in parallel. Otherwise use single-session or `Agent` tool with `isolation: "worktree"`.

**Team shape (conservative + adversarial):**
- **Lead** (this session): never edits source. Reads punch-list, plans slice, spawns teammates, reviews diffs, opens PRs, ticks audit files.
- **implementer-A**: owns one PR scope (one `docs/audit/pr-NN-*.md`), one file-set.
- **implementer-B**: owns a different PR scope, file-set must not overlap implementer-A.
- **tester**: owns `*_test.go` for both PRs in flight. Writes/updates tests only, never impl files.
- **reviewer**: read-only. Audits each implementer's diff after `make lint test` green, before lead opens PR.
- **devils-advocate**: read-only. Challenges plans, surfaces failure modes the lead is anchoring past, cross-checks reviewer findings on security/lifecycle/schema PRs.

Total: 5 teammates + lead. Do not scale up unless punch-list has 4+ truly independent rows.

**Spawn rules:**
- Implementer/tester teammates: spawn with `isolation: "worktree"` semantics — pass branch name + scope file path in spawn prompt. Worktree under `../goscratch-<branch>`.
- Spawn prompt MUST include: branch name, scope file, file-set boundary ("only edit files X/Y/Z"), "do not push or open PR — lead handles ship".
- Reviewer + devils-advocate: read-only, no worktree needed.
- Before spawning two implementers, lead diffs their scope file lists. If overlap, serialize (one at a time) instead of parallelizing.

**Plan-mode (lead judgment per spawn):**
Trigger plan-mode when implementer's scope touches any of:
- Auth/authz/crypto code (`internal/adapter/casbin`, `internal/platform/auth`, JWT, token/session).
- Schema/migration files (`migrations/`, DDL).
- Process lifecycle (`internal/platform/app`, shutdown, signal handling, worker drain).
- Block-ship class findings in current punch-list.

Otherwise no plan-mode — trust implementer + reviewer + tester loop. Devils-advocate runs always for security/lifecycle/schema.

**Task list discipline:**
- Lead creates one shared task per PR scope, one for tests, one for review pass. Implementers self-claim. Tester claims after impl marks ready-for-test. Reviewer claims after `make lint test` green.
- Hooks (when wired): `TeammateIdle` exit-2 nudge if PR not pushed; `TaskCompleted` exit-2 if `make lint test` not green.

**Cleanup:**
- Lead opens all PRs, ticks `docs/audit/pr-NN-*.md` checkboxes + `docs/audit/punch-list.md` rows, updates `TaskList`, then runs team cleanup. Never let a teammate self-cleanup.

**When NOT to use teams:**
- Single PR in flight, single file-set → single session.
- Sequential dependencies (PR-B blocks on PR-A) → single session or strict serialize.
- Quick edits (typo, single-line, doc tweak) → single session.
- Investigation-only (no edits) → consider 5x devils-advocate debate pattern instead of impl team.