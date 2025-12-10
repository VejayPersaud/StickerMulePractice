## CURRENT PROJECT STRUCTURE
```
StickerMulePractice/
├── main.go                    (Handler struct, CRUD resolvers, Prometheus middleware)
├── main_test.go               (13 test cases, all passing)
├── go.mod
├── go.sum
├── docker-compose.yml         (Prometheus + Grafana stack)
├── traffic-generator.sh       (Load testing script)
├── observability/
│   └── prometheus.yml         (Prometheus scrape config)
├── .env                       (DATABASE_URL - not in Git)
├── .gitignore
├── README.md
└── PROGRESS.md                
```


### Testing


### Running Tests
```bash
#Run all tests
go test -v

#Run with coverage
go test -cover

#Generate coverage report
go test -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Test Strategy
- Unit tests with mocked database dependencies using `sqlmock`
- HTTP handler tests using `httptest`
- Comprehensive error path coverage (not found, database errors, invalid input)
- 13 comprehensive test cases
- 95%+ coverage on business logic handlers
- Test driven development approach
- Mocked database dependencies for fast, isolated tests



## Features

### GraphQL API
- **Queries:**
  - `store(id: Int!)` - Fetch store by ID
- **Mutations:**
  - `createStore(name: String!, revenue: Float!, active: Boolean)` - Create new store
  - `updateStore(id: Int!, name: String, revenue: Float, total_orders: Int, active: Boolean)` - Update existing store
  - `deleteStore(id: Int!)` - Delete store



### Example Queries

**Create a store:**
```graphql
mutation {
  createStore(name: "My Store", revenue: 50000, active: true) {
    id
    name
    revenue
  }
}
```

**Update a store:**
```graphql
mutation {
  updateStore(id: 1, revenue: 75000) {
    id
    name
    revenue
  }
}
```

**Delete a store:**
```graphql
mutation {
  deleteStore(id: 1) {
    success
    id
  }
}
```


### Prometheus Metrics 
- Implemented HTTP instrumentation with Counter and Histogram metrics
- Automated middleware 
- Metrics exposed at `/metrics` endpoint for Prometheus scraping
- **RED Method Coverage:**
  - Rate: `http_requests_total` (by method, path, status)
  - Errors: Status code tracking (200, 404, 500, etc.)
  - Duration: `http_request_duration_seconds` histogram with percentiles
- Created `responseWriter` wrapper to capture response status codes
- Performance insights: /health ~0.07ms, /store ~143ms (with database latency visible)

**Key Learning:** Middleware pattern enables instrumentation without touching business logic. Histograms reveal distribution patterns that averages hide.

**Production Pattern:** Single middleware automatically instruments all endpoints - scalable and maintainable.



### Observability Stack 
- Set up Prometheus and Grafana in Docker using docker-compose
- Configured Prometheus to scrape `/metrics` endpoint every 15s
- Built comprehensive Grafana dashboard with RED method:
  - **Rate:** `rate(http_requests_total[1m])` - requests per second by endpoint
  - **Errors:** 4xx/5xx percentage tracking with regex filtering
  - **Duration:** p50/p95/p99 latency percentiles using `histogram_quantile()`
- Created traffic generator script for realistic load testing
- Real time visualization of all metrics with historical data retention
- Architecture: Pull based monitoring (Prometheus scrapes app, zero app dependencies)

**Key Learning:** Histograms + `histogram_quantile()` enable percentile calculations. Pull model keeps app simple, monitoring infrastructure has zero impact on app reliability.

**Production Insight:** Latency percentiles reveal distribution, /health at ~0.1ms vs /store at ~150ms (database overhead visible). p99 tracking catches worst case user experiences that averages hide.


### Observability Stack Extended + CI/CD 
- Deployed app to GCP Cloud Run (serverless, auto-scaling)
- Set up CI/CD with GitHub Actions (auto-deploy on push to main)
- Deployed Prometheus + Grafana on GCP Compute Engine (e2-micro VM)
- Configured Prometheus to scrape Cloud Run metrics via HTTPS
- Rebuilt Grafana dashboard with RED method, made json congif file:
  - **Rate:** `rate(http_requests_total[1m])` - requests per second by endpoint
  - **Errors:** 4xx/5xx percentage tracking with regex filtering
  - **Duration:** p50/p95/p99 latency percentiles using `histogram_quantile()`
- Fixed traffic generator for live load testing

**Key Learning:** Cloud Run (serverless) vs Compute Engine (VMs) - understanding when to use each. CI/CD eliminates machine-specific deployment issues. Pull based monitoring keeps app independent of observability infrastructure.

**Production Insight:** Latency distribution visible, /health ~0.1ms vs /store ~150ms (database overhead). p99 tracking reveals worst-case user experience. Error rate fluctuates 0-30% with traffic patterns.


### Distributed Tracing 
- Integrated OpenTelemetry SDK for trace instrumentation
- HTTP handlers with otelhttp middleware
- Added custom spans for database queries with metadata
- Deployed Jaeger on observability VM
- Configured OTLP HTTP exporter to send traces from Cloud Run to Jaeger
- Opened firewall port 4318 for trace ingestion

**Key Learning:** Distributed tracing reveals WHERE time is spent, not just HOW MUCH. Nested spans show parent-child relationships. Tags provide context. Foundation for performance optimization.

**Production Insight:** Traces show individual request journeys through the system. Database network latency is the primary bottleneck, candidate for caching layer.


**Live URLs:**
- Production app: https://stickermule-app-386055911814.us-central1.run.app
- Prometheus: http://35.225.111.249:9090
- Grafana: http://35.225.111.249:3000
- Jaeger: http://35.225.111.249:16686
- Redis: http://35.225.111.249:6379
- All tests passing (13/13)
- CI/CD operational (GitHub Actions)
- Full observability: Metrics + Logs + Traces



### Redis Caching
- Deployed Redis 7 on observability VM (256MB, LRU eviction)
- Using cache aside pattern for read operations
- Cache invalidation on mutations (create/update/delete)
- Added cache hit/miss Prometheus metrics
- X-Cache headers show HIT/MISS status for debugging
- Distributed tracing includes cache.get and cache.set spans
- **Performance Results:**
  - Cache hit rate: 55% initial
  - Network latency to Neon reduced significantly

**Key Learning:** Cache-aside pattern with TTL and invalidation prevents stale data. Graceful degradation ensures app works even if Redis fails. Metrics  show cache effectiveness.

**Production Insight:** Hit rate climbs as cache warms up. Cache metrics enable optimization.

**Architecture:** Redis co-located with observability stack. Cloud Run connects via public IP (will move to internal networking with Kubernetes later).

## CI/CD Pipeline Refinement

### Backend CI/CD Pipeline
- **Two-stage pipeline:** `test` job must pass before `deploy` job runs
- **Code quality gates:** `go vet` and `staticcheck` catch bugs before deployment
- **Test coverage threshold:** Pipeline fails if coverage drops below 20%
- **Race condition detection:** `go test -race` flag identifies concurrency bugs
- **Dependency caching:** Go modules cached between runs for faster builds
- **Version tagging:** Automatic `v1.0.TIMESTAMP` tags on each successful deploy
- **PR support:** Tests run on pull requests without deploying

### Frontend CI/CD Pipeline
- **TypeScript type checking:** `tsc --noEmit` catches type errors before deploy
- **ESLint integration:** Code quality enforcement on every push
- **Build verification:** `npm run build` must succeed before deployment
- **npm caching:** Dependencies cached for faster CI runs
- **Version tagging:** Matches backend tagging pattern


### Key Files Modified
- `backend/.github/workflows/deploy.yml` - Full CI/CD with test gates
- `frontend/.github/workflows/deploy.yml` - TypeScript/ESLint checks

**Key Learning:** `needs: test` creates job dependencies - deploy only runs after tests pass. `permissions: contents: write` required for git tagging. Go version in CI must match `go.mod` exactly.

**Bug Fixes:**
- Go 1.25 required for module compatibility (dependencies needed newer Go)
- `staticcheck` enforces Go style conventions (lowercase error messages)
- GitHub Actions needs explicit write permission for pushing tags