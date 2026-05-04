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