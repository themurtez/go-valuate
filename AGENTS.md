- Do not use git commands unless instructed.
- At the end of each task that involves changes, update README.md with the summary (the same summary you give the user at the end of task completion).
- When re-running tests, make sure previous runs are not still going / stuck for some reason, unless you are trying to test multple runs / concurrency of the same test.

GIT IS STRICTLY OFF-LIMITS

Do not run or invoke any Git operation, including:
- git status
- git diff
- git log
- git branch
- git checkout
- git switch
- git reset
- git stash
- git add
- git commit
- git worktree
- EnterWorktree or equivalent worktree-switching tools

If the current workspace appears stale, detached, incomplete, or different
from the expected repository state, STOP and ask the user to relaunch the
agent in the correct working directory. Never repair or switch repository
state yourself.
