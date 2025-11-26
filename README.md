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

**Live URLs:**
- App: https://stickermule-app-386055911814.us-central1.run.app
- Prometheus: http://35.239.84.255:9090
- Grafana: http://35.239.84.255:3000