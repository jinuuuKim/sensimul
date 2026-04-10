# SenSimul Web Client Development Plan

## TL;DR
> **Summary**: Add a separately operable Go web service for SenSimul that provides end-user CRUD workflows for sites, sensors, and controllers; live sensor views and charts; one-shot sensor tests; and in-app manual pages. The web service will be server-rendered with HTMX, consume metadata from SQLite and live telemetry from MQTT, and be deployable as an optional service in the same Docker stack.
> **Deliverables**:
> - `cmd/web` Go web service
> - server-rendered UI with HTMX + embedded static assets
> - full site/sensor/controller CRUD backend + pages
> - live sensor value and chart pages via MQTT → web cache → SSE
> - one-shot sensor test flow isolated from normal telemetry
> - MQTT manual page and user manual page in-app
> - Docker/compose wiring with optional web enablement
> - tests for repository, handlers, live bridge, and end-to-end flows
> **Effort**: Large
> **Parallel**: YES - 3 waves
> **Critical Path**: 1 → 2 → 3 → 8 → 9 → 10 → 12 → 13 → 14

## Context
### Original Request
Build a web client for general users based on the current CLI commands and user manual. Authentication pages are not needed because authentication will later be handled by nginx. Required features are site CRUD/settings, sensor CRUD/settings, sensor testing and data display pages with charts, controller CRUD/settings, MQTT integration manual page, and user manual page. The web client will be deployed in the same Docker environment as the Go simulator, but must be operable separately and optionally disabled.

### Interview Summary
- Web must be a **separate service/container** from the simulator process.
- It must be possible to **disable the web service at deploy time**.
- Technical direction is **Go server-rendered UI + HTMX**, not a SPA.
- Graphs are **live-only for testing use**, with **no history persistence in MVP**.
- Backend path is a **separate Go web backend** in the same repository, reusing domain/repository packages.
- Sensor test behavior is **one-shot test**.
- Authentication UI is **out of scope**; nginx auth will be added later.
- Execution planning must allow **OpenCode Go models and OpenAI models together**; no OpenAI-only restriction.
- Test strategy: **tests-after**.

### Metis Review (gaps addressed)
- Added explicit guardrail that the web service must **not reuse `internal/app.App`** because it is simulator-oriented and has startup side effects.
- Fixed live-data ambiguity by choosing **SQLite for metadata**, **MQTT for live telemetry**, and **web process-local cache** for latest values and short chart windows.
- Fixed one-shot test ambiguity by choosing a **dedicated test topic namespace** so test events do not pollute normal live charts.
- Added deploy-time enable/disable as a **compose-profile/service-level decision**, not a build flag.
- Added stale/disconnected UI states, empty DB states, simulator-down behavior, and sensor-test timeout behavior to acceptance criteria.

## Work Objectives
### Core Objective
Deliver a production-suitable MVP web client for SenSimul as a separate Go service that lets authenticated users later manage simulator metadata, observe live sensor data, run one-shot tests, and read operational documentation.

### Deliverables
- Separate Go web binary at `cmd/web`
- Server-rendered UI using Go templates/templ + HTMX
- Embedded static assets served by the web service
- CRUD flows for sites, sensors, and controllers
- Live sensor overview/detail pages with charts
- MQTT-backed live bridge and SSE browser streaming
- One-shot sensor test flow with isolated test topic
- In-app MQTT manual page
- In-app user manual page
- Dockerfile/compose wiring for optional web service enablement
- Test suite for handlers, repositories, live bridge, and web flows

### Definition of Done (verifiable conditions with commands)
- [ ] `go build ./cmd/web` succeeds
- [ ] `go test ./...` succeeds
- [ ] `go test ./cmd/web/... ./internal/web/... -race` succeeds where toolchain allows CGO
- [ ] `docker compose --profile web up -d --build` starts simulator, broker, and web service successfully
- [ ] `curl -f http://localhost:8080/healthz` returns success when web profile is enabled
- [ ] Site/sensor/controller create, edit, delete flows work end-to-end from the browser
- [ ] Live sensor detail page shows new chart points when MQTT telemetry arrives
- [ ] One-shot sensor test emits to test topic only and is visible only in test result UI
- [ ] Web service remains usable for metadata/manual pages when simulator is down, and live views show disconnected state
- [ ] In-app manual pages render correctly from embedded content

### Must Have
- Separate `cmd/web` service and container
- Optional deployment via compose profile or equivalent service toggle
- Go server-rendered pages with HTMX interactions
- SQLite metadata read/write through shared repository/service layer
- MQTT subscription in web service for live data
- SSE browser updates for live cards/charts
- Live-only chart cache in memory with bounded window
- Dedicated test topic namespace for sensor tests
- Manual pages for MQTT integration and user usage
- No dependency on simulator process memory

### Must NOT Have
- No login page or user management
- No nginx auth implementation
- No history persistence or analytics warehouse in MVP
- No browser-direct MQTT as primary architecture
- No mandatory REST SPA backend split
- No reuse of simulator CLI command code as web handler logic
- No hidden metadata bootstrap writes during web startup
- No WebSocket-first design when SSE is sufficient
- No CMS/editor for docs pages
- No multi-tenant or role-based permissions scope

## Verification Strategy
> ZERO HUMAN INTERVENTION - all verification is agent-executed.
- Test decision: **tests-after** with handler/repository/live-bridge/E2E coverage
- QA policy: Every task includes executable verification and explicit happy/failure scenarios
- Evidence: `.sisyphus/evidence/task-{N}-{slug}.{ext}`

## Execution Strategy
### Parallel Execution Waves
Wave 1: architecture boundaries, persistence/API foundation, web bootstrap, compose toggle  
Wave 2: entity CRUD UI + live bridge + charts  
Wave 3: sensor test flow, docs/manual pages, verification, deployment polish

### Dependency Matrix (full, all tasks)
```text
1 → 2 → 5 → 8 → 9 → 10 → 13 → 14
1 → 3 → 6 ───────────────→ 13
1 → 4 ─┬→ 5/6/7/11/12/13/14
       └→ 8/9/10
2 → 7 ───────────────────→ 13
3 → 11 ──────────────────→ 14
8 → 9 → 10 ──────────────→ 13/14
12 ──────────────────────→ 14
```

### Agent Dispatch Summary
| Wave | Task Count | Categories |
|------|------------|------------|
| Wave 1 | 5 | deep / unspecified-high / visual-engineering |
| Wave 2 | 5 | visual-engineering / deep / unspecified-high |
| Wave 3 | 4 | writing / unspecified-high / deep |

## TODOs
> Implementation + Test = ONE task. Never separate.
> Every task includes agent-executed QA scenarios.

- [ ] 1. Establish web-service architecture boundary and separate composition root

  **What to do**: Add a new `cmd/web` entrypoint and a web-specific composition root/service wiring package. Reuse domain, repository, payload, and shared MQTT topic helpers only. Do not import simulator CLI command logic or `internal/app.App`. Define web config keys for listen address, MQTT subscription settings, SSE cache window, and docs paths.
  **Must NOT do**: Reuse simulator startup orchestration, auto-create default site/sensors on web startup, or bind the web process to simulator in-memory state.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: architecture boundary and composition-root separation are the highest-risk design decisions
  - Skills: `[]` - no special skill required
  - Omitted: `[]` - no omission beyond default

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: 2-14 | Blocked By: none

  **References**:
  - Pattern: `cmd/sensimul/main.go` - current binary entrypoint shape
  - Pattern: `internal/app/app.go` - simulator-specific composition root that must not be reused directly
  - Pattern: `internal/config/config.go` - current config schema and validation style
  - Pattern: `docker-compose.yml` - current service-level deployment wiring

  **Acceptance Criteria**:
  - [ ] `cmd/web/main.go` exists and builds
  - [ ] web wiring does not import `internal/cli` or call `internal/app.App.Run()`
  - [ ] web config is independently loadable from simulator runtime config
  - [ ] startup performs no metadata bootstrap writes

  **QA Scenarios**:
  ```
  Scenario: web service boots independently
    Tool: Bash
    Steps: `go build ./cmd/web`
    Expected: build succeeds without importing simulator CLI runtime path
    Evidence: .sisyphus/evidence/task-1-web-bootstrap.txt

  Scenario: no hidden bootstrap writes
    Tool: Bash
    Steps: `rm -f data/test-web.db && SENSIMUL_SQLITE_PATH=data/test-web.db go run ./cmd/web --check-config-only`
    Expected: config check succeeds and no default site/sensor records are inserted
    Evidence: .sisyphus/evidence/task-1-no-bootstrap.txt
  ```

  **Commit**: YES | Message: `feat(web): add separate web service bootstrap` | Files: `cmd/web`, `internal/webapp`, `internal/config`

- [ ] 2. Extend repository/service layer for full metadata CRUD and explicit no-bootstrap behavior

  **What to do**: Add update/delete/get-by-id/list methods for sites, sensors, and controllers in a shared service/repository layer. Add explicit service methods for create/update/delete validation. Preserve current SQLite schema unless additional metadata fields are required. Make entity initialization explicit and remove any assumption that missing metadata should be auto-created during normal runtime.
  **Must NOT do**: Add history persistence, hide validation failures, or embed HTTP concerns into repository methods.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: concentrated backend data-model expansion
  - Skills: `[]`
  - Omitted: `[]`

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: 6-7, 13-14 | Blocked By: 1

  **References**:
  - Pattern: `internal/persistence/sqlite/repository.go` - current schema and repository style
  - Pattern: `internal/domain/site.go` - site validation rules
  - Pattern: `internal/domain/sensor.go` - sensor validation and allowed types
  - Pattern: `internal/domain/controller.go` - controller validation and indoor-only rule

  **Acceptance Criteria**:
  - [ ] site/sensor/controller update and delete methods exist
  - [ ] delete operations are hard deletes in MVP
  - [ ] validation errors are preserved as domain/config/runtime errors
  - [ ] repository tests cover create/update/delete/list/get flows

  **QA Scenarios**:
  ```
  Scenario: CRUD repository flow
    Tool: Bash
    Steps: `go test ./internal/persistence/sqlite -run TestRepositoryCRUD -v`
    Expected: create/update/delete/list/get tests pass for all three entity types
    Evidence: .sisyphus/evidence/task-2-repo-crud.txt

  Scenario: invalid controller for outdoor site
    Tool: Bash
    Steps: `go test ./internal/persistence/sqlite ./internal/domain -run TestControllerValidationForOutdoorSite -v`
    Expected: validation failure is returned and persisted data is unchanged
    Evidence: .sisyphus/evidence/task-2-controller-validation.txt
  ```

  **Commit**: YES | Message: `feat(data): add full metadata CRUD services` | Files: `internal/persistence/sqlite`, `internal/domain`, `internal/services`

- [ ] 3. Add shared MQTT topic contract and isolated test-topic namespace

  **What to do**: Centralize all MQTT topic formatting/parsing into shared helpers. Preserve current live topic contract for simulator telemetry and add a dedicated one-shot test contract with separate request/result namespaces: `sensimul/tests/requests/sites/{site_id}/sensors/{sensor_id}` and `sensimul/tests/results/sites/{site_id}/sensors/{sensor_id}`. Define payload contracts for request and result explicitly so the simulator can sample once and reply once without touching normal live chart streams.
  **Must NOT do**: Duplicate topic strings across simulator and web code, publish test events onto the normal live topic, or use a single shared topic for both request and result.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: cross-service contract and future drift prevention
  - Skills: `[]`
  - Omitted: `[]`

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 8-10, 13-14 | Blocked By: 1

  **References**:
  - Pattern: `internal/mqtt/publisher.go` - current live publish path
  - Pattern: `internal/payload/payload.go` - current payload schema
  - Pattern: `internal/sim/loop.go` - current publish call sites

  **Acceptance Criteria**:
  - [ ] live topic plus test request/result helpers are defined in one shared package
  - [ ] simulator publish path uses shared helpers
  - [ ] test request/result topics cannot collide with normal live topic
  - [ ] helper tests cover parse/format round-trips

  **QA Scenarios**:
  ```
  Scenario: topic round-trip contract
    Tool: Bash
    Steps: `go test ./internal/mqtt -run TestTopicContract -v`
    Expected: live and test request/result topic format/parse tests pass
    Evidence: .sisyphus/evidence/task-3-topic-contract.txt

  Scenario: test topic isolation
    Tool: Bash
    Steps: `go test ./internal/mqtt ./internal/web -run TestTestTopicIsolation -v`
    Expected: test request/result telemetry is rejected from normal live stream paths
    Evidence: .sisyphus/evidence/task-3-test-isolation.txt
  ```

  **Commit**: YES | Message: `feat(mqtt): centralize topics and add test namespace` | Files: `internal/mqtt`, `internal/payload`, `internal/sim`

- [ ] 4. Implement web HTTP bootstrap, embedded assets, health route, and compose-profile deployment wiring

  **What to do**: Add router/middleware, HTML layout shell, embedded static asset serving, `GET /healthz`, and Docker/compose wiring for a separately operable web service. Use compose profile `web` or equivalent explicit service toggle. Mount the SQLite data directory, not just the DB file. Configure service-to-service hostnames through env/config, never `localhost` for inter-container calls.
  **Must NOT do**: Require the simulator container to start before the web service can boot, or depend on external CDNs for mandatory assets.

  **Recommended Agent Profile**:
  - Category: `visual-engineering` - Reason: layout shell plus deployment wiring and embedded assets
  - Skills: `[]`
  - Omitted: `[]`

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 5-14 | Blocked By: 1

  **References**:
  - Pattern: `Dockerfile` - current builder/runtime approach
  - Pattern: `docker-compose.yml` - current service wiring
  - Pattern: `docker-compose.mqtt.yml` - broker-only compose pattern
  - Pattern: `config/sensimul.yaml` - config conventions and container broker naming

  **Acceptance Criteria**:
  - [ ] web service can be enabled via compose profile without modifying simulator service definition
  - [ ] `GET /healthz` succeeds when web service starts
  - [ ] assets are served from embedded/static files in the binary
  - [ ] web startup succeeds even if simulator is down

  **QA Scenarios**:
  ```
  Scenario: profile-based startup
    Tool: Bash
    Steps: `docker compose --profile web config`
    Expected: web service appears only when the web profile is selected
    Evidence: .sisyphus/evidence/task-4-compose-profile.txt

  Scenario: independent health route
    Tool: Bash
    Steps: `docker compose --profile web up -d web && curl -f http://localhost:8080/healthz`
    Expected: health check returns success without simulator dependency
    Evidence: .sisyphus/evidence/task-4-healthz.txt
  ```

  **Commit**: YES | Message: `feat(web): add routing shell and optional deployment wiring` | Files: `cmd/web`, `Dockerfile`, `docker-compose.yml`, `config/`

- [ ] 5. Build shared page shell, navigation, error handling, and empty/disconnected states

  **What to do**: Implement the common page layout, navigation, flash/error rendering, and standard empty states. Include explicit UI states for empty database, no sensors configured, no live data yet, stale/disconnected sensor feed, and orphan MQTT event cases. Design manual pages and CRUD pages to share the same shell.
  **Must NOT do**: Couple UI state rendering to raw repository errors or hide disconnected/stale conditions.

  **Recommended Agent Profile**:
  - Category: `visual-engineering` - Reason: cross-cutting UX shell and state system
  - Skills: `[]`
  - Omitted: `[]`

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 6-12, 14 | Blocked By: 1, 4

  **References**:
  - Pattern: `docs/USAGE.md` - user-facing terminology and workflows to mirror
  - Pattern: `README.md` - current product framing
  - Pattern: `internal/cli/root.go` - current command names that should shape nav labels

  **Acceptance Criteria**:
  - [ ] shared shell is used across all web pages
  - [ ] empty/disconnected states are explicitly rendered and testable
  - [ ] page shell remains usable when simulator is down

  **QA Scenarios**:
  ```
  Scenario: empty database UX
    Tool: Bash
    Steps: `go test ./internal/web -run TestEmptyStateRendering -v`
    Expected: empty-state views render expected messages and actions
    Evidence: .sisyphus/evidence/task-5-empty-states.txt

  Scenario: simulator-down live state
    Tool: Bash
    Steps: `go test ./internal/web -run TestDisconnectedLiveState -v`
    Expected: stale/disconnected indicators render without crashing the page shell
    Evidence: .sisyphus/evidence/task-5-disconnected-state.txt
  ```

  **Commit**: YES | Message: `feat(web): add shared shell and state views` | Files: `internal/web`, `web/templates`, `web/static`

- [ ] 6. Implement site CRUD pages and handlers

  **What to do**: Add list/create/edit/delete pages and handlers for sites. On create, users can set `id`, `name`, `type`, `latitude`, `longitude`, `timezone`, and `elevation`. On edit, `id` is immutable; editable fields are `name`, `type`, `latitude`, `longitude`, `timezone`, and `elevation`. `type` may be changed only when the site has no controllers; otherwise the UI must block the change and instruct delete/recreate or controller cleanup first. Environment defaults are not editable in MVP because they are not persisted today. Use server-rendered forms and HTMX partial updates where appropriate.
  **Must NOT do**: Add unsupported site types, silently mutate related sensors/controllers, expose non-persisted environment defaults as editable settings, or hide validation errors.

  **Recommended Agent Profile**:
  - Category: `visual-engineering` - Reason: form-heavy server-rendered UX
  - Skills: `[]`
  - Omitted: `[]`

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 13-14 | Blocked By: 2, 4, 5

  **References**:
  - Pattern: `internal/domain/site.go` - site fields and validation
  - Pattern: `internal/cli/site.go` - current site creation/list semantics
  - Pattern: `internal/persistence/sqlite/repository.go` - site persistence model

  **Acceptance Criteria**:
  - [ ] site list/create/edit/delete routes exist
  - [ ] invalid lat/lon/type inputs show validation errors in UI
  - [ ] delete action removes site and cascaded children per DB rules
  - [ ] site type edit is blocked when controllers exist for that site
  - [ ] site pages render correctly with zero, one, or many sites

  **QA Scenarios**:
  ```
  Scenario: site CRUD web flow
    Tool: Bash
    Steps: `go test ./internal/web -run TestSiteCRUDHandlers -v`
    Expected: create/edit/delete/list flows pass against test DB
    Evidence: .sisyphus/evidence/task-6-site-crud.txt

  Scenario: invalid site form submission
    Tool: Bash
    Steps: `go test ./internal/web -run TestSiteValidationErrors -v`
    Expected: invalid type or coordinates return form errors with no DB write
    Evidence: .sisyphus/evidence/task-6-site-validation.txt
  ```

  **Commit**: YES | Message: `feat(web): add site management pages` | Files: `internal/web`, `web/templates/sites`

- [ ] 7. Implement sensor CRUD pages, settings, and detail navigation

  **What to do**: Add list/create/edit/delete pages and handlers for sensors scoped to a site. On create, users can set `id` and built-in `sensor_type`. On edit, `id`, `site_id`, and `sensor_type` are immutable in MVP; editable fields are `calibration`, `noise_sigma`, and `status`. `value_kind`, `source_channel`, and `unit` remain derived from the selected built-in profile and are read-only display fields. Include direct navigation from sensor list to live detail/test page.
  **Must NOT do**: Allow arbitrary unsupported sensor types unless the registry explicitly supports them, allow inconsistent value_kind/source_channel combinations, or allow in-place sensor type mutation in MVP.

  **Recommended Agent Profile**:
  - Category: `visual-engineering` - Reason: entity settings UX plus navigation into live pages
  - Skills: `[]`
  - Omitted: `[]`

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 8-10, 13-14 | Blocked By: 2, 4, 5

  **References**:
  - Pattern: `internal/domain/sensor.go` - built-in profiles and fields
  - Pattern: `internal/cli/sensor.go` - current add/list flow
  - Pattern: `internal/persistence/sqlite/repository.go` - sensor persistence fields

  **Acceptance Criteria**:
  - [ ] sensor list/create/edit/delete routes exist under a site
  - [ ] built-in sensor types are selectable from the current registry on create only
  - [ ] invalid sensor settings show validation feedback
  - [ ] sensor detail page route exists from the list page

  **QA Scenarios**:
  ```
  Scenario: sensor CRUD web flow
    Tool: Bash
    Steps: `go test ./internal/web -run TestSensorCRUDHandlers -v`
    Expected: create/edit/delete/list flows pass for supported sensor profiles
    Evidence: .sisyphus/evidence/task-7-sensor-crud.txt

  Scenario: unsupported sensor type rejection
    Tool: Bash
    Steps: `go test ./internal/web ./internal/domain -run TestUnsupportedSensorTypeRejected -v`
    Expected: unsupported sensor type is rejected with no persisted row
    Evidence: .sisyphus/evidence/task-7-sensor-validation.txt
  ```

  **Commit**: YES | Message: `feat(web): add sensor management pages` | Files: `internal/web`, `web/templates/sensors`

- [ ] 8. Implement controller CRUD pages and settings

  **What to do**: Add list/create/edit/delete pages and handlers for controllers scoped to a site. On create, users can set `id` and `type`. On edit, `id`, `site_id`, and `type` are immutable in MVP; editable fields are `status` and `output_level`. `target_axis` is derived from controller type and displayed as read-only. Respect indoor-only support and conflict rules in validation and UI hints.
  **Must NOT do**: Permit controller creation for outdoor sites, allow in-place controller type mutation in MVP, or mask conflict/validation rules.

  **Recommended Agent Profile**:
  - Category: `visual-engineering` - Reason: form and validation-heavy workflow similar to sensor/site pages
  - Skills: `[]`
  - Omitted: `[]`

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 13-14 | Blocked By: 2, 4, 5

  **References**:
  - Pattern: `internal/domain/controller.go` - allowed controller types, output range, indoor-only rule
  - Pattern: `internal/cli/controller.go` - current add/list flow
  - Pattern: `internal/persistence/sqlite/repository.go` - controller persistence fields

  **Acceptance Criteria**:
  - [ ] controller list/create/edit/delete routes exist
  - [ ] outdoor sites cannot create controllers from the UI
  - [ ] output level validation enforces `0..100`
  - [ ] conflict rules are surfaced to the user before invalid submission

  **QA Scenarios**:
  ```
  Scenario: controller CRUD web flow
    Tool: Bash
    Steps: `go test ./internal/web -run TestControllerCRUDHandlers -v`
    Expected: create/edit/delete/list flows pass for indoor sites
    Evidence: .sisyphus/evidence/task-8-controller-crud.txt

  Scenario: outdoor controller prevention
    Tool: Bash
    Steps: `go test ./internal/web ./internal/domain -run TestOutdoorControllerBlockedInUIFlow -v`
    Expected: outdoor controller creation is rejected with validation messaging
    Evidence: .sisyphus/evidence/task-8-controller-validation.txt
  ```

  **Commit**: YES | Message: `feat(web): add controller management pages` | Files: `internal/web`, `web/templates/controllers`

- [ ] 9. Build live telemetry ingest bridge and bounded in-memory read model in the web service

  **What to do**: Subscribe the web service to live sensor MQTT topics and test topics using shared topic helpers. Maintain process-local latest-value cache and a bounded recent point buffer per sensor for charting. Define stale/disconnected policy explicitly: mark a sensor stale after a fixed timeout derived from configured tick interval or a configured `stale_after` value. Preserve metadata vs live-telemetry separation.
  **Must NOT do**: Persist live history to SQLite in MVP, poll SQLite for live chart data, or require one MQTT connection per browser.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: concurrency, cache consistency, SSE fanout, and stale-policy correctness
  - Skills: `[]`
  - Omitted: `[]`

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 10, 13-14 | Blocked By: 3, 4, 7

  **References**:
  - Pattern: `internal/mqtt/publisher.go` - broker settings and current MQTT usage
  - Pattern: `internal/payload/payload.go` - incoming live payload format
  - Pattern: `internal/config/config.go` - config validation style

  **Acceptance Criteria**:
  - [ ] web service can subscribe to live topics and parse payloads
  - [ ] latest-value cache and bounded chart buffer exist per sensor
  - [ ] stale/disconnected state is computed without simulator process memory
  - [ ] orphan events for deleted/unknown sensors are handled gracefully

  **QA Scenarios**:
  ```
  Scenario: live payload ingestion
    Tool: Bash
    Steps: `go test ./internal/web ./internal/mqtt -run TestLiveBridgeIngestsPayloads -v`
    Expected: live payloads update latest-value cache and chart buffers
    Evidence: .sisyphus/evidence/task-9-live-bridge.txt

  Scenario: stale sensor state
    Tool: Bash
    Steps: `go test ./internal/web -run TestSensorMarkedStaleAfterTimeout -v`
    Expected: sensors with no new telemetry transition to stale/disconnected state
    Evidence: .sisyphus/evidence/task-9-stale-state.txt
  ```

  **Commit**: YES | Message: `feat(web): add mqtt live bridge and sensor cache` | Files: `internal/web`, `internal/mqtt`, `internal/config`

- [ ] 10. Implement live sensor overview/detail pages, SSE endpoints, and chart rendering

  **What to do**: Create live overview and per-sensor detail pages showing current value, status, last update time, and recent live chart points. Use SSE from the web service to feed the browser. Keep HTMX for page/partial interactions and use a small embedded JS chart island for live graph rendering. Provide pages for each sensor type and a generic detail template so all built-in sensor kinds are supported.
  **Must NOT do**: Depend on history queries, require a SPA framework, or hide the “blank until next tick” startup state.

  **Recommended Agent Profile**:
  - Category: `visual-engineering` - Reason: live UX and chart rendering within server-rendered pages
  - Skills: `[]`
  - Omitted: `[]`

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 11, 13-14 | Blocked By: 5, 7, 9

  **References**:
  - Pattern: `internal/payload/payload.go` - value/status/timestamp fields to display
  - Pattern: `internal/domain/sensor.go` - sensor kinds and units
  - Pattern: `docs/USAGE.md` - user-facing terminology for sensor flows

  **Acceptance Criteria**:
  - [ ] live overview page exists with cards/table for configured sensors
  - [ ] sensor detail page exists with current value and recent live chart
  - [ ] SSE endpoint streams updates to browser consumers
  - [ ] page shows explicit blank/no-data state until first message arrives

  **QA Scenarios**:
  ```
  Scenario: SSE live updates
    Tool: Bash
    Steps: `go test ./internal/web -run TestSSEStreamsSensorUpdates -v`
    Expected: SSE endpoint emits new events as cached values change
    Evidence: .sisyphus/evidence/task-10-sse.txt

  Scenario: no data yet state
    Tool: Bash
    Steps: `go test ./internal/web -run TestLivePageShowsNoDataBeforeFirstTelemetry -v`
    Expected: live page renders without chart points and shows waiting state
    Evidence: .sisyphus/evidence/task-10-no-data-state.txt
  ```

  **Commit**: YES | Message: `feat(web): add live sensor views and charts` | Files: `internal/web`, `web/templates/live`, `web/static`

- [ ] 11. Implement one-shot sensor test flow isolated from normal telemetry

  **What to do**: Add a one-shot sensor test action from the sensor detail page. The web service publishes a request to the dedicated test-request topic, the simulator handles the request by sampling the addressed sensor once, and the simulator publishes a single response on the dedicated test-result topic. Render the result in a separate test result panel or stream. Define timeout/error handling explicitly: a test returns success on first result event within timeout, otherwise shows timeout/failure without retry. Keep test results separate from normal live chart buffers.
  **Must NOT do**: Mix test events into normal live charts, silently retry, require simulator restart, or fake a successful test entirely inside the web service.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: test isolation, timeout semantics, and cross-service event flow
  - Skills: `[]`
  - Omitted: `[]`

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: 13-14 | Blocked By: 3, 9, 10

  **References**:
  - Pattern: `internal/mqtt/publisher.go` - current MQTT publish mechanism
  - Pattern: `internal/payload/payload.go` - payload structure to adapt for test results if needed
  - Pattern: `internal/domain/sensor.go` - sensor identity and status model

  **Acceptance Criteria**:
  - [ ] sensor detail page exposes one-shot test action
  - [ ] test request and result use dedicated topic namespaces
  - [ ] test result appears in isolated UI component only
  - [ ] timeout/error path shows deterministic failure message with no DB/history write

  **QA Scenarios**:
  ```
  Scenario: successful one-shot test
    Tool: Bash
    Steps: `go test ./internal/web ./internal/mqtt -run TestOneShotSensorTestSuccess -v`
    Expected: one request event and one result event are emitted and isolated result UI updates once
    Evidence: .sisyphus/evidence/task-11-one-shot-success.txt

  Scenario: test timeout
    Tool: Bash
    Steps: `go test ./internal/web -run TestOneShotSensorTestTimeout -v`
    Expected: timeout state is rendered and no normal live chart point is created
    Evidence: .sisyphus/evidence/task-11-one-shot-timeout.txt
  ```

  **Commit**: YES | Message: `feat(web): add isolated one-shot sensor tests` | Files: `internal/web`, `internal/mqtt`, `web/templates/live`

- [ ] 12. Add in-app MQTT manual page and user manual page rendering

  **What to do**: Render manual pages in the web service from embedded markdown or maintained HTML templates. Include at least two in-app docs pages: user manual and MQTT integration manual. Reuse the same navigation shell. The user manual should align with current CLI/manual terminology while being phrased for web users where needed.
  **Must NOT do**: Build a CMS, require external doc hosting, or diverge from the actual runtime contracts.

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: documentation structure and user-facing clarity
  - Skills: `[]`
  - Omitted: `[]`

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: 14 | Blocked By: 4, 5

  **References**:
  - Pattern: `docs/USAGE.md` - current user manual source
  - Pattern: `README.md` - high-level framing and usage terminology
  - Pattern: `config/sensimul.yaml` - MQTT connection examples and config values

  **Acceptance Criteria**:
  - [ ] `/docs/manual` and `/docs/mqtt` (or equivalent) routes exist
  - [ ] rendered docs are embedded into the web binary
  - [ ] docs nav is reachable from the main shell
  - [ ] docs content matches current runtime contract and config names

  **QA Scenarios**:
  ```
  Scenario: docs render in-app
    Tool: Bash
    Steps: `go test ./internal/web -run TestManualPagesRender -v`
    Expected: docs routes return rendered content using the shared shell
    Evidence: .sisyphus/evidence/task-12-docs-render.txt

  Scenario: mqtt manual contract accuracy
    Tool: Bash
    Steps: `go test ./internal/web -run TestMQTTManualIncludesCurrentTopicAndBrokerSettings -v`
    Expected: rendered MQTT docs include current topic/broker/config references
    Evidence: .sisyphus/evidence/task-12-mqtt-docs.txt
  ```

  **Commit**: YES | Message: `docs(web): add in-app manuals` | Files: `docs/`, `internal/web`, `web/templates/docs`

- [ ] 13. Add handler, repository, SSE, and end-to-end verification coverage

  **What to do**: Add tests-after coverage for repository CRUD, form handlers, validation errors, live MQTT ingest, SSE streaming, stale-state behavior, and one-shot test behavior. Include browser-level verification for at least one end-to-end CRUD path and one live chart path. Keep evidence paths explicit.
  **Must NOT do**: Rely solely on manual clicking or skip failure-path coverage.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: broad verification across backend and UI flows
  - Skills: `[]`
  - Omitted: `[]`

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: 14 | Blocked By: 2, 5, 6, 7, 9, 10, 11, 12

  **References**:
  - Pattern: `Makefile` - existing build/test commands
  - Pattern: `docker-compose.yml` - integration environment wiring
  - Pattern: `docs/USAGE.md` - expected user flows to reproduce in browser tests

  **Acceptance Criteria**:
  - [ ] `go test ./...` passes with new web coverage
  - [ ] race test target exists for web packages where supported
  - [ ] one browser-level CRUD test passes
  - [ ] one browser-level live-data/chart test passes

  **QA Scenarios**:
  ```
  Scenario: package test suite
    Tool: Bash
    Steps: `go test ./...`
    Expected: all repository, web, and integration tests pass
    Evidence: .sisyphus/evidence/task-13-go-test.txt

  Scenario: browser CRUD + live verification
    Tool: Playwright
    Steps:
      1. Start web profile compose stack
      2. Navigate to web UI
      3. Create site, sensor, controller
      4. Open sensor detail page
      5. Verify live update/chart behavior
    Expected: CRUD succeeds and live page renders updates
    Evidence: .sisyphus/evidence/task-13-playwright.txt
  ```

  **Commit**: YES | Message: `test(web): add coverage for handlers and live flows` | Files: `internal/**/_test.go`, `tests/integration`

- [ ] 14. Finalize Docker deployment, docs alignment, and operational toggles

  **What to do**: Update Docker build targets, compose files, and written docs so the web service can be enabled or omitted cleanly. Confirm data volume sharing rules for SQLite/WAL files, confirm broker hostname usage, and document simulator-down/web-up behavior. Update user manual references so CLI and web docs stay consistent.
  **Must NOT do**: Leave deploy toggles implicit, rely on `localhost` for inter-container calls, or require web service for simulator-only deployments.

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: deployment/operator documentation plus final contract alignment
  - Skills: `[]`
  - Omitted: `[]`

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: Final Verification Wave | Blocked By: 4, 10, 12, 13

  **References**:
  - Pattern: `Dockerfile` - current binary build path
  - Pattern: `docker-compose.yml` - current service topology
  - Pattern: `docker-compose.mqtt.yml` - profile/optional-service precedent
  - Pattern: `docs/USAGE.md` - existing usage doc to align with web docs
  - Pattern: `config/sensimul.yaml` - current service-to-service configuration naming

  **Acceptance Criteria**:
  - [ ] compose profile or equivalent toggle cleanly enables/disables the web service
  - [ ] SQLite volume-sharing rules are documented and implemented
  - [ ] docs explain simulator-only vs simulator+web deployment
  - [ ] docs explain live-only charts and no-history limitation

  **QA Scenarios**:
  ```
  Scenario: simulator-only deployment still works
    Tool: Bash
    Steps: `docker compose up -d --build`
    Expected: simulator and broker run without web service requirement
    Evidence: .sisyphus/evidence/task-14-simulator-only.txt

  Scenario: web-enabled deployment works
    Tool: Bash
    Steps: `docker compose --profile web up -d --build && docker compose ps`
    Expected: web, simulator, and broker services all report healthy/running
    Evidence: .sisyphus/evidence/task-14-web-profile.txt
  ```

  **Commit**: YES | Message: `chore(web): finalize deployment wiring and docs` | Files: `Dockerfile`, `docker-compose.yml`, `docs/USAGE.md`, `README.md`

## Final Verification Wave (MANDATORY — after ALL implementation tasks)
> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.
- [ ] F1. Plan Compliance Audit — oracle
- [ ] F2. Code Quality Review — unspecified-high
- [ ] F3. Real Manual QA — unspecified-high (+ playwright if UI)
- [ ] F4. Scope Fidelity Check — deep

## Commit Strategy
- Use one commit per completed task where the task materially changes behavior or deployment.
- Keep commits scoped to the task boundaries above.
- Execution may use **OpenCode Go-focused agents/models and OpenAI models together**; there is no OpenAI-only restriction in this plan.
- Do not push partial web architecture changes that break simulator-only deployment.

## Success Criteria
- A general user can manage sites, sensors, and controllers from the browser without using CLI commands.
- Live sensor pages update in real time from MQTT without persistent history requirements.
- One-shot sensor tests are clearly separated from normal telemetry.
- The web service can be deployed alongside or independently from the simulator through Docker compose configuration.
- Documentation pages are available in-app and consistent with real runtime behavior.
- Simulator-only deployments still function unchanged when the web service is disabled.
