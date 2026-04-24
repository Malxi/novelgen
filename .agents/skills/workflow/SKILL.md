# Workflow Skill

## Description
Executes the optimization workflow for one or more novelgen modules. This skill coordinates analysis, documentation, implementation, and completion phases for module improvements.

## Input
One or more optimization targets (space-separated):
- `setup` - Optimize story setup module
- `compose` - Optimize outline compose module  
- `craft` - Optimize worldbuilding craft module
- `draft-write` - Optimize draft and write modules
- `all` - Optimize all modules

## Overview
This skill defines the standard workflow for optimizing the novelgen project modules.

## Prerequisites
- All improvement tracking files exist:
  - `setup_improvements.md`
  - `compose_improvements.md`
  - `craft_improvements.md`
  - `draft_write_improvements.md`

## Workflow Steps

### Phase 0: Preparation
1. **Checkout main branch**
   - Ensure you are on main branch: `git checkout main`
   - Pull latest changes: `git pull origin main`

2. **Create worktree for changes**
   - Create worktree from main: `git worktree add .Codex/worktrees/<branch-name> -b <branch-name>`
   - Switch to the new worktree: `cd .Codex/worktrees/<branch-name>`
   - All subsequent work happens in this isolated workspace

### Phase 1: Analysis
3. **Use project skills to analyze optimization opportunities**
   - Run `optimize-setup` skill to analyze setup module
   - Run `optimize-compose` skill to analyze compose module
   - Run `optimize-craft` skill to analyze craft module
   - Run `optimize-draft-write` skill to analyze draft/write module

4. **Review code logic**
   - Examine related source code for each module
   - Identify specific optimization opportunities
   - Focus on: performance, clarity, maintainability, correctness

### Phase 2: Documentation
5. **Document improvements**
   - Write identified improvements to corresponding files
   - Use format: `- improve: [status] description`
   - Status options: `[inprogress]`, `[completed]`
   - Files to update:
     - `setup_improvements.md` for setup module
     - `compose_improvements.md` for compose module
     - `craft_improvements.md` for craft module
     - `draft_write_improvements.md` for draft/write module

### Phase 3: Implementation
6. **Implement improvements**
   - Make code changes according to documented improvements
   - Update improvement status to `[inprogress]` while working

7. **Verify build**
   - Run `go build` to ensure code compiles
   - Fix any compilation errors

### Phase 4: Completion
8. **Update documentation**
   - Change improvement status to `[completed]`
   - Add any notes about implementation details

9. **Commit changes**
   - Stage modified files: `git add .`
   - Commit with descriptive message: `git commit -m "description"`
   - Push to remote: `git push origin <branch-name>`

10. **Merge to main**
    - Switch to main branch: `git checkout main`
    - Merge the worktree branch: `git merge <branch-name>`
    - Push main: `git push origin main`

11. **Cleanup**
    - Remove worktree: `git worktree remove .Codex/worktrees/<branch-name>`
    - Delete remote branch if pushed: `git push origin --delete <branch-name>`

12. **Work complete**

## Example Improvement Entry
```markdown
- improve: [completed] Optimize database queries in character loading by adding cache layer.
```

## Notes
- Always verify build before marking as completed
- Keep improvements focused and atomic
- Update documentation before committing code changes
