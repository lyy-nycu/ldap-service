## Context

`ldap-service` is a Go microservice extracted from the PHP monolith and deployed to Azure Container Apps. Two Container App Environments (staging, prod) and a shared Azure Container Registry already exist. Today there is no automated pipeline — images are built and pushed by hand, and `az containerapp update` is invoked locally. This is fragile, leaves no audit trail, and risks long-lived service principal secrets sitting in operator shells.

The pipeline must:

- Be entirely owned by this repo (no shared workflow repo); independent lifecycle from `mfa-service`.
- Hold no long-lived Azure credentials.
- Guarantee the bytes deployed to prod are the same bytes that passed staging.
- Be safe to re-run for rollback.

Resource provisioning (the Container Apps, managed identity, federated credentials, secret bindings) is delegated to **Copilot via Azure MCP** during implementation. The pipeline assumes those exist and references them by name through GitHub repo variables.

## Goals / Non-Goals

**Goals:**

- Automatic build → staging → prod deploy on every PR merge to `main`, with the prod step gated by required reviewer approval on the GitHub `prod` Environment.
- OIDC federated auth — zero secrets stored in GitHub for Azure access.
- Same image (`git-<sha>`) deployed to both environments.
- Health-gated promotion: prod only runs if staging's new revision passes `/healthz` and `/readyz`.
- Manual `workflow_dispatch` escape hatch for rollback or pre-merge staging validation.
- A separate CI workflow runs `go vet` and `go test ./...` on every PR with **no Azure access**.

**Non-Goals:**

- Provisioning Azure resources from the pipeline (delegated to Copilot + Azure MCP).
- Multi-region deploy, blue/green, canary traffic splitting.
- Promoting a staging-validated image to prod without rebuild from a later `main`.
- Sharing the workflow file with `mfa-service`. The pattern is reused; the file is not.

## Decisions

### D1. OIDC federated credential, not a service principal secret

**Choice**: Federate one user-assigned managed identity to GitHub via OIDC. The workflow uses `azure/login@v2` with `client-id`, `tenant-id`, `subscription-id` from repo variables.

**Why**: A federated credential issues a short-lived token scoped to a specific GitHub claim (branch ref or environment name). Nothing rotatable lives in GitHub Secrets. Compromise of the GitHub repo cannot replay against Azure outside the federation context.

**Alternatives considered**:

- *SP client secret in `AZURE_CREDENTIALS`*: rotation burden, leak surface (logs, exfil), no claim-binding.
- *Self-hosted runner with managed identity*: heavier ops; not justified for a single-repo deploy.

### D2. One workflow run, build once, deploy twice

**Choice**: Single `deploy.yml` with three jobs — `build` → `deploy-staging` → `deploy-prod`. The image is built and pushed once in `build`; both deploy jobs reference the same `git-<sha>` tag.

**Why**: Eliminates "the prod image isn't the staging image" failure mode. No source race between staging-time and prod-time builds. Cheaper.

**Alternatives considered**:

- *Two separate workflows triggered by different events*: would require image tagging coordination across runs, easy to drift.
- *Rebuild for prod from same SHA*: byte-identical not guaranteed (timestamps, dependency cache, base image float).

### D3. GitHub Environments for the prod approval gate

**Choice**: `prod` is a GitHub Environment with required-reviewer protection. The `deploy-prod` job declares `environment: prod`; the run blocks until a designated reviewer approves in the GitHub UI.

**Why**: Native, auditable, integrates with the same OIDC federation (the federated credential includes `environment:prod` as a subject claim). No third-party approval bot.

**Alternatives considered**:

- *Manual workflow_dispatch for prod*: requires the operator to remember to run it; loses the chained-from-staging signal.
- *External approval system (PagerDuty, etc.)*: overkill for this scale.

### D4. Image tags: `git-<sha>` (immutable) + `<env>-latest` (mutable)

**Choice**: `ghcr/acr/.../ldap-service:git-abc1234` is what `az containerapp update --image` references. `staging-latest` and `prod-latest` are pushed alongside as moving pointers for human inspection (`az acr repository show-tags`).

**Why**: Rollback = workflow_dispatch with a previous SHA. Audit = the deployed tag is unambiguous. Mutable `<env>-latest` is convenience only — never the deploy target.

**Alternatives considered**:

- *Semver tags*: requires release management this service doesn't have yet.
- *Only `<env>-latest`*: makes rollback hard, breaks immutability.

### D5. Health gate via existing `/healthz` + `/readyz`

**Choice**: After `az containerapp update`, poll the app's FQDN until both endpoints return 200, with timeout (default 3 min) and 5s interval. Failure = job fails = prod blocked.

**Why**: The endpoints already exist (per `internal/handler/health.go`). No new probes to maintain.

**Alternatives considered**:

- *Trust ACA's revision health alone*: ACA marks a revision healthy on container start, before LDAP pool warmup completes. False positives are likely.

### D6. Pipeline RBAC: minimum scope

**Choice**: The federated identity gets `AcrPush` on the shared ACR (build job) and `Container App Contributor` on each of the two Container Apps (deploy jobs). Nothing at resource group or subscription scope.

**Why**: A compromised pipeline cannot create new resources, modify networking, or read secrets outside the two apps it deploys.

**Alternatives considered**:

- *`Contributor` on the resource group*: convenient but oversized — pipeline could reconfigure the Container App Environment, log workspace, etc.

### D7. CI workflow is separate and has no Azure permissions

**Choice**: `.github/workflows/ci.yml` runs on `pull_request` and on `push` to non-`main` branches. It runs `go vet`, `go test ./...`, and `docker build` (smoke). It has no `permissions: id-token: write` — cannot federate to Azure.

**Why**: PRs from forks (or accidental compromises) cannot reach Azure even via a contrived workflow change, because the CI workflow simply lacks the token. Defense in depth alongside branch protection.

### D8. Concurrency: cancel in-progress staging, never cancel prod

**Choice**: Workflow concurrency group `deploy-${{ github.ref }}` with `cancel-in-progress: true` for the `build` and `deploy-staging` jobs. The `deploy-prod` job uses a separate `prod-deploy` concurrency group with `cancel-in-progress: false`.

**Why**: Two rapid merges to `main`: the second build cancels the first build mid-flight, but a prod deploy already in motion completes. Avoids a half-applied prod revision.

## Risks / Trade-offs

- **Risk**: `git-<sha>` tags accumulate forever in ACR → cost.
  → **Mitigation**: Out-of-band ACR retention policy (Copilot + Azure MCP, not pipeline concern). Document the recommended policy (retain last 30 tags or 90 days) in README.

- **Risk**: Prod reviewer not available; merge to `main` leaves a deployment hanging.
  → **Mitigation**: Configure multiple reviewers on the `prod` Environment. The build artifact stays in ACR indefinitely, so a delayed approval does not lose the image.

- **Risk**: Health gate gives false negative (transient probe blip), blocks prod.
  → **Mitigation**: Re-run via `workflow_dispatch` with the same SHA. Document this in README.

- **Risk**: `<env>-latest` tags drift confusingly if a workflow_dispatch-driven deploy uses an older SHA.
  → **Mitigation**: `workflow_dispatch` deploys do **not** update `<env>-latest`. Only the on-`main` chain updates it.

- **Risk**: Concurrent merges + the cancel-on-staging rule → operator confusion ("why did my deploy cancel?").
  → **Mitigation**: Workflow logs the cancel reason; README explains the concurrency model.

- **Trade-off**: Two Container Apps share the same ACR repository. A bug in ACR retention policy could prune an image still referenced by prod. Caller must coordinate the retention rule with the longest-lived deployed revision.

## Migration Plan

This is a greenfield pipeline; nothing is being deprecated.

**Phase 1 — Provisioning (Copilot + Azure MCP):**

1. Create user-assigned managed identity `id-ldap-service-cicd`.
2. Grant `AcrPush` on the shared ACR.
3. Create Container Apps `ldap-service-staging` (in staging Env) and `ldap-service-prod` (in prod Env), each with a placeholder image.
4. Grant `Container App Contributor` on each app to the identity.
5. Create federated credentials on the identity for: `repo:<org>/ldap-service:ref:refs/heads/main`, `:environment:staging`, `:environment:prod`.
6. Configure per-app secret bindings (`API_KEY`, LDAP bind credentials, etc.) — values set out-of-band, not in the pipeline.

**Phase 2 — Pipeline (this change):**

1. Land `.github/workflows/ci.yml` first (PR-only, low blast radius).
2. Configure GitHub repo variables and Environments (`staging`, `prod` with required reviewers).
3. Land `.github/workflows/deploy.yml`.
4. First merge to `main` is treated as a smoke test; reviewer should expect to inspect staging before approving prod.

**Rollback:**

- Bad image lands in prod → run `workflow_dispatch` with environment=`prod` and the prior known-good `git-<sha>` ref. The build job is skipped (image already exists in ACR with that tag); deploy-prod re-pins ACA to that image.
- Bad workflow lands → revert the workflow file PR; the next `main` push uses the reverted workflow.

## Open Questions

- **Q1**: Should the `prod`-Environment reviewer list be the same as the GitHub repo admins, or a narrower subset (e.g., on-call rotation only)? **Defer** to the user — populated at provisioning time, not encoded in the workflow.
- **Q2**: Health-gate timeout default of 3 minutes — is the LDAP pool warmup over the S2S VPN ever slower than that? If yes, raise to 5 min. **Defer** to first staging smoke test.
- **Q3**: Do we want the CI workflow to also run `golangci-lint`? It is not currently in the project's tech stack list. **Defer** — out of scope for this change.
