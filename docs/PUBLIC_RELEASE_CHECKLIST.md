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
## Gitea To GitHub Mirror

Use Gitea push mirror from the active Gitea repository to the matching public GitHub repository.

1. Create the empty GitHub repository first.
2. Create a GitHub token with repository write permission for the target public repository.
3. In Gitea, open repository settings.
4. Add a push mirror using `https://github.com/<owner>/<repo>.git`.
5. Use the GitHub username and token in the authorization fields.
6. Enable sync on push if available.
7. Treat GitHub as read-only for human contributors.

Public GitHub direct commits must not be used because a Gitea push mirror can overwrite GitHub-side changes.

## GitHub Environment Policy

- `dev`: CI and non-production deploy workflows may run automatically after merge.
- `test`: workflow dispatch or release-candidate branches only.
- `prod`: workflow dispatch only, protected branch or tag, required reviewer, and separate production secrets.

Repository-level secrets are allowed for CI-only values.
Deployment secrets must use environment secrets.
