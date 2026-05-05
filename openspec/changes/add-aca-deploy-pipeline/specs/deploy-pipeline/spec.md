## ADDED Requirements

### Requirement: Pipeline lives in this repository

The CI/CD pipeline for `ldap-service` SHALL be defined entirely within this repository's `.github/workflows/` directory. It MUST NOT depend on a `workflow_call` to a workflow file in any other repository. Each workflow file MUST be self-contained so that `mfa-service` (and other Go services) can copy and adapt it without external coupling.

#### Scenario: Workflow files are self-contained

- **WHEN** an operator inspects `.github/workflows/deploy.yml` and `.github/workflows/ci.yml`
- **THEN** neither file declares `uses: <other-org-or-repo>/.github/workflows/...@<ref>`
- **AND** all reusable logic (build, push, deploy steps) is inlined or invoked via published, third-party, version-pinned Actions only

### Requirement: Azure authentication uses OIDC federated credentials

The pipeline SHALL authenticate to Azure exclusively via OpenID Connect with a federated credential bound to a user-assigned managed identity. It MUST NOT store, read, or accept any long-lived Azure client secret, certificate, or service-principal password from GitHub Secrets.

#### Scenario: No Azure client secret is referenced

- **WHEN** the workflow files are scanned for the strings `AZURE_CLIENT_SECRET`, `AZURE_CREDENTIALS`, or `client-secret`
- **THEN** zero matches are found
- **AND** the only Azure auth step uses `azure/login@v2` with `client-id`, `tenant-id`, `subscription-id` sourced from `vars.*` (repo variables), not `secrets.*`

#### Scenario: Workflow declares the OIDC token permission

- **WHEN** any job that talks to Azure runs
- **THEN** that job (or the workflow root) declares `permissions: id-token: write` and `contents: read`

### Requirement: Deploy is triggered by merge to main and by manual dispatch

The deploy workflow SHALL run on `push` to branch `main` and on `workflow_dispatch`. The dispatch trigger SHALL accept an `environment` input (`staging` or `prod`) and use the dispatched ref's commit SHA for the build.

#### Scenario: Merge to main triggers full chain

- **WHEN** a pull request is merged into `main`
- **THEN** the deploy workflow runs with `github.ref == 'refs/heads/main'`
- **AND** the workflow executes `build` → `deploy-staging` → `deploy-prod` jobs in that order

#### Scenario: workflow_dispatch deploys an arbitrary ref to a chosen env

- **WHEN** an operator triggers `workflow_dispatch` with `environment: staging` and `ref: feature/x`
- **THEN** the workflow builds the image from `feature/x`'s commit and deploys to `ldap-service-staging`
- **AND** the workflow does NOT touch `ldap-service-prod`

### Requirement: Build once, deploy the same image to every environment

The build job SHALL produce exactly one container image per workflow run, tagged with `git-<short-sha>` derived from the triggering commit. Both `deploy-staging` and `deploy-prod` jobs SHALL deploy that exact tag. The pipeline MUST NOT rebuild between staging and prod.

#### Scenario: Same digest reaches staging and prod

- **WHEN** a `main` push runs the full chain to completion
- **THEN** the image digest deployed to `ldap-service-staging` equals the image digest deployed to `ldap-service-prod`

#### Scenario: Tag is derived from the triggering commit

- **WHEN** the build job runs for commit `abc1234deadbeef...`
- **THEN** the pushed image tag is `git-abc1234` (first 7 chars of SHA)

### Requirement: Image registry is the shared Azure Container Registry

The build job SHALL push images to the shared Azure Container Registry whose name is supplied via the `ACR_NAME` repo variable. It MUST NOT push to GitHub Container Registry, Docker Hub, or any other registry.

#### Scenario: Image is pushed only to the configured ACR

- **WHEN** the build job completes
- **THEN** the image is pushed to `${{ vars.ACR_NAME }}.azurecr.io/ldap-service:git-<sha>`
- **AND** no other registry endpoints appear in the workflow logs

### Requirement: Mutable env-latest tags are written only by the main chain

In addition to `git-<sha>`, the workflow SHALL push `staging-latest` and `prod-latest` tags pointing to the same image — but ONLY when the run is triggered by push to `main` and the corresponding deploy job succeeds. `workflow_dispatch` runs MUST NOT update either `<env>-latest` tag.

#### Scenario: Dispatch deploy does not move env-latest

- **WHEN** an operator runs `workflow_dispatch` with environment=`staging` and an older SHA
- **THEN** `staging-latest` in the registry continues to point to whatever image the most recent `main` chain deployed
- **AND** the older `git-<sha>` is what runs in `ldap-service-staging`

### Requirement: Prod deploy is gated by reviewer approval

The `deploy-prod` job SHALL declare `environment: prod`. The GitHub `prod` Environment SHALL be configured with at least one required reviewer. The job MUST NOT execute its deploy steps until a designated reviewer approves the run in the GitHub UI.

#### Scenario: Prod waits for approval

- **WHEN** `deploy-staging` completes successfully on a `main` push
- **THEN** the `deploy-prod` job enters a "Waiting" state
- **AND** no `az containerapp update` call against `ldap-service-prod` happens until a reviewer approves

#### Scenario: Reviewer rejection blocks prod deploy

- **WHEN** the assigned reviewer rejects the deployment
- **THEN** the `deploy-prod` job is marked failed
- **AND** `ldap-service-prod` retains its previous revision unchanged

### Requirement: Health gate blocks prod when staging is unhealthy

After updating a Container App revision, the deploy job SHALL poll the app's `/healthz` and `/readyz` endpoints over HTTPS until both return HTTP 200, with a default timeout of 3 minutes and 5-second interval. If either endpoint does not return 200 within the timeout, the job SHALL fail. A failed `deploy-staging` job SHALL prevent `deploy-prod` from running.

#### Scenario: Staging health failure blocks prod

- **WHEN** `ldap-service-staging`'s new revision returns non-200 from `/readyz` for the entire 3-minute window
- **THEN** `deploy-staging` exits non-zero
- **AND** `deploy-prod` is skipped (not just gated on approval — skipped entirely)

#### Scenario: Health passes on first poll

- **WHEN** `/healthz` and `/readyz` both return 200 on the first poll
- **THEN** the deploy job moves to the tag-update step without further waiting

### Requirement: CI workflow runs on PRs without Azure access

A separate workflow at `.github/workflows/ci.yml` SHALL run on `pull_request` and on `push` to non-`main` branches. It SHALL run `go vet ./...`, `go test ./...`, and a `docker build` smoke check. It MUST NOT declare `permissions: id-token: write` and MUST NOT reference any Azure variable or secret.

#### Scenario: CI cannot federate to Azure

- **WHEN** the CI workflow file is inspected
- **THEN** no job declares `id-token: write`
- **AND** no step references `vars.AZURE_*` or `secrets.AZURE_*`

#### Scenario: PR triggers tests but not deploy

- **WHEN** a pull request targeting `main` is opened or updated
- **THEN** `ci.yml` runs
- **AND** `deploy.yml` does NOT run

### Requirement: Concurrency protects in-flight prod deploys

The workflow SHALL declare two concurrency groups: a per-ref group `deploy-${{ github.ref }}` with `cancel-in-progress: true` covering `build` and `deploy-staging`, and a global group `deploy-prod` with `cancel-in-progress: false` covering `deploy-prod`. A second `main` push SHALL cancel an in-progress build/staging deploy of the first push but MUST NOT cancel a `deploy-prod` job that has already started.

#### Scenario: Rapid second merge cancels staging only

- **WHEN** two merges to `main` happen within seconds, and the first run is mid-`deploy-staging` while the second run begins `build`
- **THEN** the first run's `build` and `deploy-staging` are cancelled
- **AND** if the first run had already entered `deploy-prod`, that job continues to completion

### Requirement: Pipeline does not provision or modify Azure resources beyond the two Container Apps

The deploy workflow SHALL only invoke `az containerapp update` (or equivalent) against the Container Apps named in `ACA_APP_NAME_STAGING` and `ACA_APP_NAME_PROD`. It MUST NOT create, delete, or reconfigure: Container App Environments, the ACR itself, managed identities, federated credentials, secret bindings, networking, or RBAC assignments.

#### Scenario: No az resource creation commands

- **WHEN** the workflow files are scanned for `az containerapp env create`, `az identity create`, `az role assignment create`, `az acr create`, `az containerapp secret set`
- **THEN** zero matches are found

### Requirement: Repository documents the deploy contract

The repository's `README.md` SHALL contain a "Deploying" section that lists all required GitHub repo variables, the GitHub Environments and their protection rules, the trigger model, and the rollback procedure (workflow_dispatch with a prior `git-<sha>`).

#### Scenario: README lists all required variables

- **WHEN** a new operator reads the README "Deploying" section
- **THEN** they find each of `AZURE_TENANT_ID`, `AZURE_SUBSCRIPTION_ID`, `AZURE_CLIENT_ID`, `ACR_NAME`, `ACA_RESOURCE_GROUP`, `ACA_APP_NAME_STAGING`, `ACA_APP_NAME_PROD` documented with its purpose
- **AND** the document explains that `prod` requires reviewer approval
- **AND** the document explains how to roll back via `workflow_dispatch`
