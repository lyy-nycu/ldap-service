# LDAP Service — Technical Specification (MVP)

> **Status**: Draft v3.1
> **Target**: Azure Container Apps
> **Language**: Go
> **Date**: 2026-04

---

## 1. Project overview

### Purpose

為 portal.nycu.edu.tw 建立一個獨立的 LDAP 存取層 microservice，作為所有服務查詢與操作 on-prem OpenLDAP 的唯一入口。採用 Strangler Fig pattern，從 PHP 8.3 monolith 逐步抽離 LDAP 相關邏輯。

### Scope (MVP)

MVP 僅包含 read 與 authenticate 功能。Write（修改 LDAP 屬性）留到 phase 2。

| In scope | Out of scope (phase 2) |
|----------|----------------------|
| 單一帳號屬性查詢 | 修改 LDAP 屬性 |
| 批次帳號屬性查詢 | Scope-based API Key 權限控制 |
| LDAP bind 密碼驗證 | Audit log dashboard |
| API Key 驗證 middleware | Redis-based 分散式 rate limiting |
| Health check endpoints | |
| 結構化 logging (zap) | |
| Authenticate rate limiting (per-username, in-memory) | |

### Callers

| Caller | 用途 | 通訊方式 |
|--------|------|---------|
| PHP 8.3 monolith (portal) | 登入流程的帳號查詢與密碼驗證 | API Key + HTTPS via S2S VPN |
| MFA Service (Go, 未來) | 查詢使用者 email / mobile 以發送 OTP | API Key + HTTPS (service-to-service) |

Browser **永遠不會**直接呼叫此 service。

---

## 2. Tech stack — dependency 選型

| Dependency | Version | 用途 | 選用理由 |
|-----------|---------|------|---------|
| Go | 1.22+ | Language runtime | `net/http.ServeMux` 已支援 method routing 與 path param，不需第三方 router |
| `net/http` (stdlib) | — | HTTP server + router | 官方標準庫，零外部依賴 |
| `github.com/go-ldap/ldap/v3` | v3 | LDAP client | Go 生態圈唯一成熟的 LDAP library |
| `go.uber.org/zap` | latest | Structured logging | 高效能 JSON structured logging，適合 production 環境 |
| `crypto/subtle` (stdlib) | — | API Key 比對 | Constant-time comparison 防止 timing attack |
| `crypto/tls` (stdlib) | — | LDAPS 連線 | 標準庫 TLS 實作 |
| `encoding/json` (stdlib) | — | JSON 序列化 | 標準庫，效能對此 service 足夠 |
| `github.com/google/uuid` | latest | Request ID 產生 | 輕量、廣泛使用、Google 維護 |
| `github.com/joho/godotenv` | latest | 開發環境 .env 載入 | 僅在 `.env` 存在時載入，cloud 環境直接讀系統 env var |
| `golang.org/x/time/rate` | latest | Per-username rate limiting | Go 官方 extended library，搭配 `sync.Map` (stdlib) 做 in-memory rate limit |

### godotenv 在 cloud 環境的使用策略

```go
// main.go — 條件載入，cloud 環境不需要 .env 檔案
if _, err := os.Stat(".env"); err == nil {
    _ = godotenv.Load()
}
```

Local 開發時用 `.env` 檔案方便快速設定，Azure Container Apps 部署時 secrets 和 config 設定在平台的 environment variables，不帶 `.env` 進 container image。兩者互不干擾。

---

## 3. API contract

Base URL: `https://<container-app-url>`

所有 API endpoints（除 health check 外）都需要 `X-Api-Key` header。

### 3.1 Error response — RFC 7807 Problem Details

所有 error response 共用 RFC 7807 格式，Content-Type 為 `application/problem+json`。

**Schema**

| Field | Type | Required | 說明 |
|-------|------|----------|------|
| `type` | string (URI) | Yes | 錯誤類型的 URI 識別碼 |
| `title` | string | Yes | 人類可讀的錯誤摘要 |
| `status` | integer | Yes | HTTP status code |
| `detail` | string | No | 此次錯誤的具體描述 |
| `instance` | string | No | 此次 request 的識別碼（request ID） |

**Error type URI 定義**

| Type URI | Title | Status | 觸發條件 |
|----------|-------|--------|---------|
| `/problems/invalid-request` | Invalid request | 400 | JSON parse 失敗、缺少必要欄位 |
| `/problems/invalid-username` | Invalid username | 400 | Username 不符合格式規則 |
| `/problems/attribute-not-allowed` | Attribute not allowed | 400 | 請求了不在 whitelist 的 attribute |
| `/problems/unauthorized` | Unauthorized | 401 | API Key 缺少或無效 |
| `/problems/authentication-failed` | Authentication failed | 401 | LDAP bind 驗證失敗 |
| `/problems/not-found` | Account not found | 404 | 查無此帳號 |
| `/problems/service-unavailable` | Service unavailable | 503 | LDAP 連線異常 |
| `/problems/internal-error` | Internal error | 500 | 非預期錯誤 |
| `/problems/rate-limit-exceeded` | Rate limit exceeded | 429 | 同一 username 的 authenticate 嘗試超過頻率限制 |

**Error response 範例**

```json
{
  "type": "/problems/invalid-username",
  "title": "Invalid username",
  "status": 400,
  "detail": "username must match [a-zA-Z0-9._-], max 64 chars",
  "instance": "req-550e8400-e29b-41d4-a716-446655440000"
}
```

```json
{
  "type": "/problems/attribute-not-allowed",
  "title": "Attribute not allowed",
  "status": 400,
  "detail": "attribute 'userPassword' is not in the allowed list",
  "instance": "req-550e8400-e29b-41d4-a716-446655440001"
}
```

```json
{
  "type": "/problems/authentication-failed",
  "title": "Authentication failed",
  "status": 401,
  "detail": "authentication failed",
  "instance": "req-550e8400-e29b-41d4-a716-446655440002"
}
```

### 3.2 Health check

#### `GET /healthz`

Liveness probe，不檢查 LDAP 連線。

**Response** `200 OK`
```json
{
  "status": "ok"
}
```

#### `GET /readyz`

Readiness probe，檢查 LDAP 連線是否正常。

**Response** `200 OK`
```json
{
  "status": "ready"
}
```

**Response** `503 Service Unavailable`
```json
{
  "type": "/problems/service-unavailable",
  "title": "Service unavailable",
  "status": 503,
  "detail": "ldap connection check failed"
}
```

### 3.3 Lookup — 單一帳號查詢

#### `POST /api/v1/ldap/lookup`

**Request**
```json
{
  "username": "110550001",
  "attributes": ["mail", "mobile", "givenName"]
}
```

**Response** `200 OK`
```json
{
  "dn": "uid=110550001,ou=student,o=nycu",
  "uid": "110550001",
  "attributes": {
    "mail": "110550001@nycu.edu.tw",
    "mobile": "0912345678",
    "givenName": "王小明"
  }
}
```

**Error responses**: 參照 3.1 的共用格式（`/problems/not-found`、`/problems/invalid-username`、`/problems/attribute-not-allowed`）。

### 3.4 Lookup batch — 批次帳號查詢

#### `POST /api/v1/ldap/lookup/batch`

**Request**
```json
{
  "usernames": ["110550001", "T1234"],
  "attributes": ["mail", "dept"]
}
```

**Response** `200 OK`
```json
{
  "accounts": [
    {
      "dn": "uid=110550001,ou=student,o=nycu",
      "uid": "110550001",
      "attributes": {
        "mail": "110550001@nycu.edu.tw",
        "dept": "資訊工程學系"
      }
    },
    {
      "dn": "uid=T1234,ou=employee,o=nycu",
      "uid": "T1234",
      "attributes": {
        "mail": "t1234@nycu.edu.tw",
        "dept": "資訊技術服務中心"
      }
    }
  ],
  "not_found": []
}
```

**Constraints**: `usernames` 上限 50 筆。不存在的帳號回在 `not_found` 陣列中，不影響其他帳號的查詢結果。

### 3.5 Authenticate — 密碼驗證

#### `POST /api/v1/ldap/authenticate`

使用 search-then-bind pattern：先用 read-only account 搜尋使用者 DN，再用使用者的密碼嘗試 bind。

**Request**
```json
{
  "username": "110550001",
  "password": "user-password-here"
}
```

**Response** `200 OK` — 驗證成功
```json
{
  "authenticated": true
}
```

**Response** `401 Unauthorized` — 驗證失敗（不論帳號不存在或密碼錯誤，統一回傳）
```json
{
  "type": "/problems/authentication-failed",
  "title": "Authentication failed",
  "status": 401,
  "detail": "authentication failed",
  "instance": "req-..."
}
```

**安全要求**：
- 不論是「帳號不存在」或「密碼錯誤」，一律回傳相同 response，防止 username enumeration
- `password` 欄位絕對不能出現在任何 log 中
- Per-username rate limiting：同一 username 每分鐘最多 5 次嘗試，超過回傳 `429`

**Response** `429 Too Many Requests` — 超過 rate limit
```json
{
  "type": "/problems/rate-limit-exceeded",
  "title": "Rate limit exceeded",
  "status": 429,
  "detail": "too many authentication attempts for this username, try again later",
  "instance": "req-..."
}
```

---

## 4. Attribute whitelist

只有以下 LDAP attributes 允許透過 API 查詢。任何不在此清單的 attribute 請求會回傳 `400`。

| Attribute | 說明 |
|-----------|------|
| `cn` | 學校教職員工號碼、學生號碼，校外人士 email |
| `uid` | 與 cn 相同值 |
| `sn` | 使用者 first name |
| `givenName` | 使用者姓名 |
| `fullname` | 使用者全名（on-prem 自訂 attribute） |
| `initials` | 使用者縮寫（on-prem 自訂 attribute） |
| `dept` | 部門、系所中文名稱 |
| `deptCode` | 部門、系所代碼 |
| `employeeStatus` | 使用者狀態 |
| `title` | 使用者職稱 |
| `ou` | 使用者在 OpenLDAP 的 ou |
| `mobile` | 個人手機 |
| `mail` | 學校 Google mail |
| `alternative-mail` | 個人備援 mail（自訂 attribute，含 hyphen） |

備註：目前 whitelist 為全域規則，所有通過 API key 驗證的 service 共用同一套可查詢 attributes，尚未實作 per-service attribute ACL。

---

## 5. Input validation rules

| Field | Rule |
|-------|------|
| `username` | Regex `^[a-zA-Z0-9._-]{1,64}$` |
| `attributes` | 每個值必須在 section 4 whitelist 內 |
| `usernames` (batch) | 最多 50 筆，每筆套用 username regex |
| `password` | 不為空即可，不做格式限制（LDAP 自己會驗） |
| LDAP filter | 所有 user input 必須經過 `ldap.EscapeFilter()` |

---

## 6. LDAP directory structure

### Base DN

`o=nycu`

### OU 清單

| OU | DN | 說明 | 狀態 |
|----|-----|------|------|
| `student` | `ou=student,o=nycu` | 學生帳號 | In use |
| `employee` | `ou=employee,o=nycu` | 教職員帳號 | In use |
| `alumni` | `ou=alumni,o=nycu` | 校友帳號 | In use |
| `cooperator` | `ou=cooperator,o=nycu` | 合作單位帳號 | In use |
| `retire` | `ou=retire,o=nycu` | 退休人員帳號 | In use |

### Search strategy

Search base 設定為 `o=nycu`，scope 使用 `WholeSubtree`。帳號可能分佈在任一 OU，由 LDAP server 自動搜尋所有子 OU，application 層不需要判斷帳號屬於哪個 OU。

### Bind accounts

| Account | 用途 | 權限 |
|---------|------|------|
| Read-only bind | lookup, search, readyz health check | `search` on `o=nycu` |
| User bind (臨時) | authenticate 時用使用者密碼做 bind | 驗完後 connection 回 pool 前 re-bind 回 read-only |
| Write bind (phase 2) | 修改 attribute | `write` on 限定 attributes |

### Connection pool

- Pool size: 可設定，預設 10
- Idle timeout: 可設定，預設 30 秒
- 從 pool 取出 connection 時檢查是否 alive，dead 就重建
- Pool 滿了就建新的 connection，用完不放回 pool（直接關閉）
- VPN 可能造成 connection 靜默斷線 → 需要 health check 機制

### Rate limiting — authenticate endpoint

Connection pool 管理的是「資源」（LDAP 連線數），rate limit 管理的是「安全」（防止 brute force）。兩者互補，缺一不可。

**實作方式**：
- `sync.Map` 儲存每個 username 對應的 `rate.Limiter` instance
- `golang.org/x/time/rate` 控制頻率：每個 username 每分鐘最多 5 次（Token Bucket: rate=5/60s, burst=5）
- 僅套用在 `POST /api/v1/ldap/authenticate`，lookup endpoints 暫不限制（caller 為 PHP 和 MFA service，流量可控）
- In-memory 實作，不需要 Redis。MVP 的 replica 數為 1-3，各 replica 獨立計數，worst case 是允許 5 × replica 數的嘗試，對 MVP 可接受
- 定期清理：背景 goroutine 每 10 分鐘清除超過 10 分鐘未使用的 limiter entry，防止 memory leak

**Phase 2 升級路徑**：如果未來 replica 數增加或需要精確的全域計數，可替換為 Redis-based rate limiter，介面不變

---

## 7. Authentication middleware

### API Key 驗證

- Header: `X-Api-Key`
- 比對方式: `crypto/subtle.ConstantTimeCompare()`
- Key 格式: 至少 32 bytes random，hex 或 base64 encoded
- 設定方式: 環境變數 `API_KEYS`，格式 `key1:service_name1,key2:service_name2`
- 無效 key: 回 RFC 7807 格式 `401`，log warning（包含 remote IP，不包含 key 值）

### Request tracing

- 每個 request 注入 `X-Request-ID`（caller 可自帶，否則 server 用 `google/uuid` 產生）
- 所有 log 都帶 request ID
- Error response 的 `instance` 欄位帶 request ID

---

## 8. Security checklist (OWASP)

| OWASP | 風險 | 對策 |
|-------|------|------|
| A01 Broken Access Control | 未授權存取 LDAP 資料 | API Key middleware，attribute whitelist |
| A02 Cryptographic Failures | 密碼明文洩漏 | LDAPS (TLS 1.2+)，password 不進 log |
| A03 Injection | LDAP filter injection | `ldap.EscapeFilter()` 處理所有 user input |
| A04 Insecure Design | 過度暴露 LDAP 操作 | Read-only bind，MVP 不開 write |
| A07 Auth Failures | Username enumeration | Authenticate endpoint 回傳統一的 error response |
| A07 Auth Failures | Brute force 密碼猜測 | Per-username rate limiting（5 次/分鐘），`sync.Map` + `x/time/rate` |
| A09 Logging | 缺乏稽核紀錄 | zap structured logging，每個 request 記錄 who/what/when |

---

## 9. Directory structure

```
ldap-service/
├── CLAUDE.md                          # Claude Code harness
├── .claude/commands/new-endpoint.md   # Custom slash command
├── .github/copilot-instructions.md    # Copilot context
├── .env.example
├── .env                               # Local 開發用（gitignored）
├── .gitignore
├── Dockerfile
├── go.mod
├── cmd/
│   └── server/
│       └── main.go                    # Entrypoint, godotenv 條件載入, graceful shutdown
└── internal/
    ├── domain/
    │   ├── domain.go                  # Entities, interfaces, whitelist, errors, validation
    │   └── problem.go                 # RFC 7807 Problem Details struct + builder
    ├── usecase/
    │   ├── lookup.go                  # 查詢邏輯
    │   └── authenticate.go            # 認證邏輯
    ├── handler/
    │   ├── router.go                  # Route registration (net/http.ServeMux)
    │   ├── lookup.go                  # Lookup handlers
    │   ├── authenticate.go            # Authenticate handler
    │   ├── health.go                  # Health check handlers
    │   └── response.go               # 共用 JSON response + RFC 7807 error helpers
    ├── middleware/
    │   ├── apikey.go                  # API Key validation
    │   ├── ratelimit.go              # Per-username rate limiting (sync.Map + x/time/rate)
    │   ├── requestid.go              # Request ID injection
    │   └── logger.go                  # zap request logging
    └── infra/
        ├── config/
        │   └── config.go             # Env var loading
        └── ldap/
            └── repository.go          # LDAP connection pool + operations
```

---

## 10. Environment variables

| Variable | Required | Default | 說明 |
|----------|----------|---------|------|
| `PORT` | No | `8080` | HTTP server port |
| `LDAP_HOST` | Yes | — | OpenLDAP server hostname |
| `LDAP_PORT` | No | `636` | LDAP port |
| `LDAP_USE_TLS` | No | `true` | 是否使用 LDAPS |
| `LDAP_BASE_DN` | Yes | — | Base DN（`o=nycu`） |
| `LDAP_READONLY_BIND_DN` | Yes | — | Read-only bind account DN |
| `LDAP_READONLY_BIND_PW` | Yes | — | Read-only bind account password |
| `LDAP_CONN_POOL_SIZE` | No | `10` | Connection pool size |
| `LDAP_CONN_MAX_IDLE_SEC` | No | `30` | Connection idle timeout (seconds) |
| `API_KEYS` | Yes | — | Format: `key1:name1,key2:name2` |
| `AUTH_RATE_LIMIT` | No | `5` | Authenticate endpoint 每個 username 每分鐘最多嘗試次數 |
| `AUTH_RATE_CLEANUP_MIN` | No | `10` | Rate limiter 清理間隔（分鐘） |

---

## 11. Deployment

### Dockerfile strategy

- Multi-stage build: `golang:1.22-alpine` build stage → `scratch` runtime
- `CGO_ENABLED=0` for static binary
- Copy CA certificates for LDAPS
- 不包含 `.env` 檔案

### Azure network layout

| Subnet | CIDR | 用途 |
|--------|------|------|
| Production | `10.0.3.0/24` | Production Container Apps |
| Staging | `10.0.4.0/24` | Staging Container Apps |
| spoke-paas | `10.0.1.0/24` | 共用 PaaS 資源（ACR） |

所有 subnet 透過既有的 S2S VPN 連回 on-prem OpenLDAP。

### Azure Container Registry (ACR)

- **Resource group**: spoke-paas
- **用途**: Production 和 staging 的 Container Apps 共用同一個 ACR
- **認證**: Container Apps 使用 Managed Identity pull image，不需額外 credentials
- **Image tagging**: staging 用 commit SHA，production 用 semver tag (e.g. `v1.0.0`)

### Azure Container Apps

- **Ingress**: internal only（不暴露 public endpoint）
- **Health probes**: liveness → `GET /healthz`，readiness → `GET /readyz`
- **Network**: VNet integration，透過 S2S VPN 連回 on-prem OpenLDAP
- **Scaling**: min 1, max 3（MVP）
- **Secrets**: API Keys 和 LDAP bind password 存在 Container Apps secrets
- **環境區分**: production (10.0.3.0/24) 和 staging (10.0.4.0/24) 各部署一組，環境變數（LDAP host、API Keys）各自獨立設定

### CI/CD — GitHub Actions + ACR

```
GitHub push → GitHub Actions → Build image → Push to ACR → Deploy to Container Apps
```

- **Trigger**: push to `main` → deploy staging；手動 promote 或 tag push → deploy production
- **Steps**: lint → test → docker build → push ACR → az containerapp update

---

## 12. Local development — OpenLDAP test container

開發與測試時使用本地 OpenLDAP container，不依賴 on-prem 環境。

- **Image**: `bitnami/openldap` 或 `osixia/openldap`
- **Base DN**: `o=nycu`
- **預建 OU**: `student`, `employee`, `alumni`, `cooperator`, `retire`
- **測試帳號**: 每個 OU 各建 2-3 個帳號，包含完整的 custom attributes（`dept`, `deptCode`, `alternative-mail` 等）
- **用途**: 跑 integration test、本地開發除錯

---

## 13. Pending items

| # | Item | Status |
|---|------|--------|
| 1 | 在 spoke-paas resource group 建立 ACR | Pending |
| 2 | 產生 API Keys（portal-php, mfa-service 各一組） | Pending |
| 3 | 確認 LDAP read-only bind account 的 DN 和密碼（待同事協助或自建測試 OpenLDAP） | Pending |
| 4 | 建立 local OpenLDAP test container + seed data | Pending |
| 5 | GitHub Actions workflow 設定（ACR credentials、Container Apps deploy） | Pending |