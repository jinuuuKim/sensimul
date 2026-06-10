# Public Release Checklist — sensimul

## Required Before Public GitHub Mirror

- [ ] No `.env`, `.env.*`, private keys, Terraform state, or generated build output is tracked.
- [ ] AI-agent local files are ignored and absent from the staged diff.
- [ ] `.gitlab-ci.yml` is absent if the repo has migrated to GitHub Actions.
- [ ] README documents required environment variables without real values.
- [ ] CI uses GitHub secrets or environment variables, not hardcoded credentials.
- [ ] Local absolute paths are removed or replaced with generic examples.
- [ ] Test/build commands pass on a clean checkout.
- [ ] GitHub mirror is configured from Gitea, and GitHub direct commits are disabled by team policy.

## Manual Secret Rotation Check

- [ ] Any credential previously committed in local files has been rotated.
- [ ] Any Terraform state that contained generated passwords has been removed from Git history before public push.
- [ ] GitHub environment secrets exist for `dev`, `test`, and `prod` where applicable.
