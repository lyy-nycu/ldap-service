## 1. Azure resource provisioning (Copilot via Azure MCP)

These tasks are executed by Copilot using Azure MCP tools (search → create → update). The pipeline cannot do them and must not try.

- [ ] 1.1 Confirm shared ACR exists; capture its name → set as `ACR_NAME` repo variable target
- [ ] 1.2 Confirm both Container App Environments exist (one staging, one prod); capture their resource group(s) → set as `ACA_RESOURCE_GROUP` (assumes both Envs in the same RG; if not, split into `_STAGING` / `_PROD` and update spec accordingly)
- [ ] 1.3 Create user-assigned managed identity `id-ldap-service-cicd` in the same resource group; capture `clientId`, `tenantId`, `subscriptionId`
- [ ] 1.4 Assign role `AcrPush` to the identity, scoped to the shared ACR
- [ ] 1.5 Create Container App `ldap-service-staging` in the staging Container App Environment using a placeholder image (e.g. `mcr.microsoft.com/k8se/quickstart:latest`); enable ingress on port 8080 and the existing `/healthz`+`/readyz` probes
- [ ] 1.6 Create Container App `ldap-service-prod` in the prod Container App Environment with the same configuration as 1.5
- [ ] 1.7 Assign role `Container App Contributor` to the identity, scoped individually to each of the two Container Apps (NOT the resource group)
- [ ] 1.8 Create three federated credentials on the identity:
  - subject `repo:<org>/ldap-service:ref:refs/heads/main`, audience `api://AzureADTokenExchange`
  - subject `repo:<org>/ldap-service:environment:staging`
  - subject `repo:<org>/ldap-service:environment:prod`
- [ ] 1.9 Configure runtime secrets on each Container App (`API_KEY`, `LDAP_INTERNAL_BIND_DN`, `LDAP_INTERNAL_BIND_PASSWORD`, `LDAP_EXTERNAL_BIND_DN`, `LDAP_EXTERNAL_BIND_PASSWORD`, plus any others required by `internal/infra/config/config.go`); values supplied out-of-band per environment
- [ ] 1.10 Document an ACR retention policy (retain last 30 `git-*` tags) — apply via Azure MCP

## 2. GitHub repo configuration

- [ ] 2.1 Create GitHub Environment `staging` (no protection rules)
- [ ] 2.2 Create GitHub Environment `prod` with **required reviewers** (at least one); populate reviewer list per the user's call (Q1 in design)
- [ ] 2.3 Add repo variables: `AZURE_TENANT_ID`, `AZURE_SUBSCRIPTION_ID`, `AZURE_CLIENT_ID`, `ACR_NAME`, `ACA_RESOURCE_GROUP`, `ACA_APP_NAME_STAGING` (`ldap-service-staging`), `ACA_APP_NAME_PROD` (`ldap-service-prod`)
- [ ] 2.4 Verify branch protection on `main` requires the `ci.yml` checks before merge

## 3. CI workflow (`.github/workflows/ci.yml`)

- [ ] 3.1 Trigger: `pull_request` (any base) and `push` (branches != `main`)
- [ ] 3.2 Job `test`: `actions/checkout@v4` → `actions/setup-go@v5` (Go 1.22+) → `go vet ./...` → `go test -race -count=1 ./...`
- [ ] 3.3 Job `build-smoke`: `docker buildx build .` (no push, just verify the Dockerfile builds)
- [ ] 3.4 Workflow has NO `permissions: id-token: write` and references no `vars.AZURE_*` / `secrets.AZURE_*` (verified by grep in 6.1)

## 4. Deploy workflow (`.github/workflows/deploy.yml`)

- [ ] 4.1 Triggers: `push` on branch `main`; `workflow_dispatch` with inputs `environment` (choice: `staging`|`prod`) and implicit `ref`
- [ ] 4.2 Workflow-level `permissions: { id-token: write, contents: read }`
- [ ] 4.3 Workflow-level concurrency: `group: deploy-${{ github.ref }}`, `cancel-in-progress: true` (covers `build` and `deploy-staging` only — `deploy-prod` overrides per 4.7)
- [ ] 4.4 Job `build`:
  - Compute `IMAGE_TAG=git-${GITHUB_SHA::7}` and `IMAGE=${{ vars.ACR_NAME }}.azurecr.io/ldap-service:${IMAGE_TAG}`
  - `azure/login@v2` with `client-id`, `tenant-id`, `subscription-id` from `vars.*`
  - `az acr login --name ${{ vars.ACR_NAME }}`
  - `docker buildx build --push -t $IMAGE .`
  - Output `image_tag` and `image` for downstream jobs
- [ ] 4.5 Job `deploy-staging`:
  - `needs: build`
  - `environment: staging`
  - On `push` to `main` only: also tag and push `${{ vars.ACR_NAME }}.azurecr.io/ldap-service:staging-latest` after the deploy succeeds; on `workflow_dispatch` skip the latest-tag step
  - Steps: `azure/login@v2` → `az containerapp update --name ${{ vars.ACA_APP_NAME_STAGING }} --resource-group ${{ vars.ACA_RESOURCE_GROUP }} --image $IMAGE` → poll FQDN `/healthz` and `/readyz` until both 200 (timeout 180s, interval 5s)
  - Conditional: only runs if `github.event_name == 'push'` OR (`workflow_dispatch` AND `inputs.environment == 'staging'`)
- [ ] 4.6 Job `deploy-prod`:
  - `needs: [build, deploy-staging]` for the `push` path; `needs: build` for the `workflow_dispatch` path with env=prod
  - `environment: prod` (triggers reviewer gate)
  - Same `az containerapp update` + health-poll pattern targeting `ACA_APP_NAME_PROD`
  - On `push` to `main` only: tag and push `prod-latest` after deploy succeeds
  - Conditional: runs if `github.event_name == 'push'` OR (`workflow_dispatch` AND `inputs.environment == 'prod'`)
- [ ] 4.7 Job-level concurrency on `deploy-prod`: `group: deploy-prod`, `cancel-in-progress: false`
- [ ] 4.8 Health-poll script extracted to `.github/scripts/health-poll.sh` (shared between staging+prod jobs); curl with `-fsS`, exit 0 only after both endpoints return 200, exit 1 on timeout

## 5. README documentation

- [ ] 5.1 Add a "Deploying" section to `README.md` that includes:
  - Trigger model diagram (merge → staging → reviewer gate → prod)
  - Table of required GitHub repo variables and what each is
  - GitHub Environment configuration (which has reviewer rules)
  - How to roll back: `gh workflow run deploy.yml -f environment=prod --ref <prior-good-sha>`
  - Note that runtime config (API key, LDAP creds) is set on the Container App, not in the workflow
- [ ] 5.2 Add a one-line "Deploying" pointer near the top of the README that links to the new section

## 6. Verification

- [ ] 6.1 `grep -rE 'AZURE_CLIENT_SECRET|AZURE_CREDENTIALS|client-secret' .github/` returns no matches
- [ ] 6.2 `grep -E 'id-token: write' .github/workflows/ci.yml` returns no matches
- [ ] 6.3 `grep -E 'az (containerapp env create|identity create|role assignment create|acr create|containerapp secret set)' .github/workflows/` returns no matches
- [ ] 6.4 First merge to `main` after landing this change: confirm `build` runs once, `deploy-staging` succeeds and health-passes, `deploy-prod` waits for approval, approval triggers prod deploy and prod health-passes
- [ ] 6.5 After 6.4: `az acr repository show-tags --name <ACR_NAME> --repository ldap-service` shows `git-<sha>`, `staging-latest`, `prod-latest` all pointing at the same digest
- [ ] 6.6 `workflow_dispatch` smoke test: deploy a non-`main` branch to `staging`; confirm `staging-latest` did NOT move
- [ ] 6.7 Concurrency smoke test: trigger two `main` pushes within 10s; confirm first run's `build`/`deploy-staging` cancels but a hypothetical first-run `deploy-prod` would continue (verify by reading workflow logs)
