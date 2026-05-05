## Why

`ldap-service` has no automated deploy path. To finish the strangler-fig cutover from the PHP monolith, the service needs a repeatable, auditable pipeline that builds a container image and ships it to Azure Container Apps on every push to the release branch — without long-lived cloud credentials in CI and without manual `az` commands at deploy time.

A pipeline that lives in this repo (rather than a shared template repo) keeps the service's release lifecycle independent: bugs in one workflow can never block the other service's deploy, and each repo owns its versioning and rollback.

## What Changes

- Add a GitHub Actions workflow at `.github/workflows/deploy.yml` that:
  - **Push to `main`** (i.e. PR merge into main) → builds image once, then runs **staging deploy → prod deploy** as two sequential jobs in the same run. Staging is automatic; prod is gated by **required reviewer approval** on the `prod` GitHub Environment and additionally chained on staging health passing.
  - **Manual `workflow_dispatch`** with `environment` input (`staging` | `prod`) and arbitrary branch ref → deploys the chosen ref to the chosen environment (used for ad-hoc rollbacks or testing a PR branch in staging before merge).
  - Authenticates to Azure via **OIDC federated credential** (no client secrets in GitHub)
  - Builds a multi-stage container image from the existing `Dockerfile` exactly once per run; both staging and prod deploy the **same `git-<sha>` image** (no rebuild between envs)
  - Tags the image with both `git-<sha>` (immutable, the deployed tag) and `<env>-latest` (mutable, per-env, for human reference)
  - Pushes to the **shared** Azure Container Registry
  - Updates the target Azure Container App revision to the new image tag
  - Waits for the new revision to become healthy (`/healthz`, `/readyz`) before declaring success; if staging fails health, prod is **not** deployed
- Add a CI workflow at `.github/workflows/ci.yml` that runs `go vet`, `go test ./...`, and a container build smoke check on every PR (no Azure access).
- Document the pipeline contract and required GitHub repo variables in `README.md`.
- **Azure resources** (Container Apps for staging+prod, user-assigned managed identity, federated credential, secret bindings) are provisioned and updated by Copilot via Azure MCP during implementation — they are NOT created by the pipeline itself. The pipeline only consumes already-provisioned resources by name. The shared ACR already exists and is reused.

Out of scope for this change:

- Automatic promotion of a staging-validated image to prod. Prod always rebuilds from `main`; staging is a manual destination.
- Blue/green or canary traffic splitting.
- The mfa-service pipeline (separate change in that repo, following the same pattern).

## Capabilities

### New Capabilities

- `deploy-pipeline`: Defines the CI/CD contract for `ldap-service` — what triggers a deploy, what artifacts are produced, how Azure authentication works, what guarantees the pipeline gives (immutable tags, health-gated rollout, no-secret-leak), and what Azure resources it expects to exist.

### Modified Capabilities

_None — this change adds a new capability; it does not alter existing service behavior._

## Impact

- **New files**: `.github/workflows/deploy.yml`, `.github/workflows/ci.yml`, README section "Deploying".
- **Azure resources used** (provisioned/updated out-of-band by Copilot + Azure MCP; names recorded as GitHub repo variables):
  - **Existing, reused**:
    - Shared Azure Container Registry
    - Two Container App Environments — one staging, one prod
  - **New per-service**:
    - Two Azure Container Apps — `ldap-service-staging` deployed into the staging Container App Environment, `ldap-service-prod` deployed into the prod Container App Environment
    - User-assigned managed identity with `AcrPull` on the shared registry and Container App Contributor on both Container Apps
    - Federated credential bindings for the identity:
      - `repo:<org>/ldap-service:ref:refs/heads/main` (push trigger)
      - `repo:<org>/ldap-service:environment:staging` and `:environment:prod` (workflow_dispatch via GitHub Environments)
- **New GitHub repo configuration**:
  - Variables: `AZURE_TENANT_ID`, `AZURE_SUBSCRIPTION_ID`, `AZURE_CLIENT_ID`, `ACR_NAME`, `ACA_RESOURCE_GROUP`, `ACA_APP_NAME_STAGING`, `ACA_APP_NAME_PROD`
  - GitHub Environments: `staging` (no gate), `prod` (**required reviewer approval** — at least one designated reviewer must approve before prod job runs)
  - No long-lived secrets required (OIDC handles auth)
- **No code changes** to `internal/` or `cmd/` — the service already produces a working container.
- **Runtime config** (API key, LDAP bind credentials, etc.) is injected as Container App secrets, set out-of-band by Copilot/Azure MCP per environment, not committed or pushed by the pipeline.
