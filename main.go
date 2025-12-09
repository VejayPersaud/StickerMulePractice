package main

import (
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"context"
	"time"
	"encoding/json"
	"os/exec"
	

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	//OpenTelemetry imports
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"

	//Redis
	"github.com/redis/go-redis/v9"


	
)

var db *sql.DB

var (
	//httpRequestsTotal counts all HTTP requests
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	//httpRequestDuration tracks request latency distribution
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	//cacheHits tracks cache hit operations
	cacheHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"cache_key_prefix"},
	)

	//cacheMisses tracks cache miss operations
	cacheMisses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"cache_key_prefix"},
	)


)






//Handler is a struct that holds dependencies
type Handler struct {
	database *sql.DB
	logger *slog.Logger
	redis    *redis.Client
}



//initTracer initializes OpenTelemetry tracing
func initTracer() (*trace.TracerProvider, error) {
	//Get Jaeger endpoint from environment 
	jaegerEndpoint := os.Getenv("JAEGER_ENDPOINT")
	if jaegerEndpoint == "" {
		jaegerEndpoint = "localhost:4318" //Local development default
	}

	//Create OTLP HTTP exporter
	exporter, err := otlptracehttp.New(
		context.Background(),
		otlptracehttp.WithEndpoint(jaegerEndpoint),
		otlptracehttp.WithInsecure(), //Use HTTP (not HTTPS) for internal communication
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	//Create resource describing this service
	resource, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceName("stickermule-app"),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	//Create tracer provider
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(resource),
	)

	//Set as global tracer provider
	otel.SetTracerProvider(tp)

	return tp, nil
}

//initRedis initializes Redis client
func initRedis() *redis.Client {
	//Get Redis address from environment (with fallback to localhost for local dev)
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379" //Local development default
	}

	//Create Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "", //No password for now
		DB:       0,  //Use default DB
	})

	//Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("Warning: Redis connection failed: %v", err)
		log.Printf("Continuing without cache...")
		return nil //Return nil if Redis unavailable 
	}

	fmt.Println("Redis connected successfully")
	return rdb
}



//responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

//prometheusMiddleware wraps handlers to automatically record metrics
func prometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//Start timer
		start := time.Now()
		
		//Wrap the ResponseWriter to capture status code
		wrappedWriter := &responseWriter{
			ResponseWriter: w,
			statusCode:     200,
		}

		//Call the actual handler
		next.ServeHTTP(wrappedWriter, r)

		//Calculate duration
		duration := time.Since(start).Seconds()

		//Record counter metric
		httpRequestsTotal.WithLabelValues(
			r.Method,
			r.URL.Path,
			fmt.Sprintf("%d", wrappedWriter.statusCode),
		).Inc()

		//Record histogram metric
		httpRequestDuration.WithLabelValues(
			r.Method,
			r.URL.Path,
		).Observe(duration)
	})
}

//corsMiddleware adds CORS headers for frontend access
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//Allow requests from any origin (for development)
		//In production, you'd restrict this to your frontend domain
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		//Handle preflight OPTIONS request
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}



//__________________________________________________________________LOGGING__________________________________________________________________

//shouldSampleLog determines if we should log based on sampling rate
//Always logs errors/warnings, samples INFO logs
func shouldSampleLog(level slog.Level, samplingRate float64) bool {
	//Always log errors and warnings (100%)
	if level >= slog.LevelWarn {
		return true
	}
	
	//Sample INFO logs based on rate (5% = 0.05)
	//Simple hash-based sampling for consistent behavior
	return rand.Float64() < samplingRate
}


//SamplingHandler wraps slog.Handler to implement sampling
type SamplingHandler struct {
	handler      slog.Handler
	samplingRate float64
}

func (h *SamplingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return shouldSampleLog(level, h.samplingRate)
}

func (h *SamplingHandler) Handle(ctx context.Context, record slog.Record) error {
	if shouldSampleLog(record.Level, h.samplingRate) {
		return h.handler.Handle(ctx, record)
	}
	return nil
}

func (h *SamplingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &SamplingHandler{
		handler:      h.handler.WithAttrs(attrs),
		samplingRate: h.samplingRate,
	}
}

func (h *SamplingHandler) WithGroup(name string) slog.Handler {
	return &SamplingHandler{
		handler:      h.handler.WithGroup(name),
		samplingRate: h.samplingRate,
	}
}





//__________________________________________________________________END-LOGGING__________________________________________________________________

//Method on Handler, can create test Handler with mock db
func(h *Handler) getStoreInfo(w http.ResponseWriter, r *http.Request) {
	//Start a custom span for this handler
	ctx := r.Context()
	tracer := otel.Tracer("stickermule-app")
	ctx, span := tracer.Start(ctx, "getStoreInfo")
	defer span.End()

	storeID := r.URL.Query().Get("id")
	if storeID == "" {
		storeID = "1"
	}
	
	h.logger.Info("store endpoint called",
		"store_id", storeID,
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr,
	)

	//Try cache first (if Redis is available)
	if h.redis != nil {
		_, cacheSpan := tracer.Start(ctx, "cache.get")
		cacheSpan.SetAttributes(
			attribute.String("cache.key", "store:"+storeID),
		)

		cacheKey := "store:" + storeID
		cachedData, err := h.redis.Get(ctx, cacheKey).Result()
		cacheSpan.End()

		if err == nil {
			//CACHE HIT!
			h.logger.Info("cache hit",
				"store_id", storeID,
				"cache_key", cacheKey,
			)
			cacheHits.WithLabelValues("store").Inc()
			w.Header().Set("X-Cache", "HIT")
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(cachedData))
			return
		} else if err != redis.Nil {
			//Redis error (not just cache miss)
			h.logger.Warn("cache error",
				"store_id", storeID,
				"error", err.Error(),
			)
		} else {
			//Cache miss,continue to database
			h.logger.Info("cache miss",
				"store_id", storeID,
			)
			cacheMisses.WithLabelValues("store").Inc()
		}
	}

	//Create a child span for the database query
	_, dbSpan := tracer.Start(ctx, "database.query.stores")
	dbSpan.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.statement", "SELECT name, revenue, total_orders, active FROM stores WHERE id = $1"),
		attribute.String("store.id", storeID),
	)

	//Query Postgres
	var name string
	var revenue float64
	var totalOrders int
	var active bool

	query := "SELECT name, revenue, total_orders, active FROM stores WHERE id = $1"
	err := h.database.QueryRow(query, storeID).Scan(&name, &revenue, &totalOrders, &active)

	dbSpan.End()

	if err == sql.ErrNoRows {
		h.logger.Warn("store not found",
			"store_id", storeID,
		)
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "Store not found"}`))
		return
	} else if err != nil {
		h.logger.Error("database error",
			"store_id", storeID,
			"error", err.Error(),
		)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Database error"}`))
		return
	}

	response := fmt.Sprintf(
		`{"store_id": "%s", "name": "%s", "revenue": %.2f, "total_orders": %d, "active": %t}`,
		storeID, name, revenue, totalOrders, active,
	)

	//Store in cache for next time 
	if h.redis != nil {
		_, cacheSetSpan := tracer.Start(ctx, "cache.set")
		cacheSetSpan.SetAttributes(
			attribute.String("cache.key", "store:"+storeID),
			attribute.Int("cache.ttl_seconds", 300),
		)

		cacheKey := "store:" + storeID
		err := h.redis.Set(ctx, cacheKey, response, 5*time.Minute).Err()
		cacheSetSpan.End()

		if err != nil {
			h.logger.Warn("failed to cache response",
				"store_id", storeID,
				"error", err.Error(),
			)
		} else {
			h.logger.Info("response cached",
				"store_id", storeID,
				"ttl", "5m",
			)
		}
	}

	h.logger.Info("store query successful",
		"store_id", storeID,
		"name", name,
		"revenue", revenue,
	)

	w.Header().Set("X-Cache", "MISS")
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(response))
}

//storesResolver returns all stores (LIST)
func (h *Handler) storesResolver(p graphql.ResolveParams) (interface{}, error) {
	h.logger.Info("graphql stores query - fetching all stores")

	query := "SELECT id, name, revenue, total_orders, active FROM stores ORDER BY id"
	rows, err := h.database.Query(query)
	if err != nil {
		h.logger.Error("database error during stores query",
			"error", err.Error(),
		)
		return nil, err
	}
	defer rows.Close()

	var stores []map[string]interface{}

	for rows.Next() {
		var id int
		var name string
		var revenue float64
		var totalOrders int
		var active bool

		err := rows.Scan(&id, &name, &revenue, &totalOrders, &active)
		if err != nil {
			h.logger.Error("error scanning store row",
				"error", err.Error(),
			)
			return nil, err
		}

		stores = append(stores, map[string]interface{}{
			"id":           id,
			"name":         name,
			"revenue":      revenue,
			"total_orders": totalOrders,
			"active":       active,
		})
	}

	h.logger.Info("graphql stores query successful",
		"count", len(stores),
	)

	return stores, nil
}

//Method that returns a GraphQL resolver function (READ)
func (h *Handler) storeResolver(p graphql.ResolveParams) (interface{}, error){
	//Extract ID from query
	id, ok := p.Args["id"].(int)
	if !ok {
		h.logger.Error("invalid store id argument")
		return nil, fmt.Errorf("invalid id")
	}

	h.logger.Info("graphql store query",
		"store_id", id,
	)


	//Query database - using h.database instead of global db
	var storeID int
	var name string
	var revenue float64
	var totalOrders int
	var active bool

	query := "SELECT id, name, revenue, total_orders, active FROM stores WHERE id = $1"
	err := h.database.QueryRow(query, id).Scan(&storeID, &name, &revenue, &totalOrders, &active)

	if err == sql.ErrNoRows {
		h.logger.Warn("store not found",
			"store_id", id,
		)
		return nil, fmt.Errorf("Store not found")
	}

	if err != nil {
		h.logger.Error("database error during query",
			"store_id", id,
			"error", err.Error(),
		)
		return nil, err
	}

	h.logger.Info("graphql store query successful",
		"store_id", id,
		"name", name,
	)
	

	//Return as a map
	return map[string]interface{}{
		"id":           storeID,
		"name":         name,
		"revenue":      revenue,
		"total_orders": totalOrders,
		"active":       active,
	}, nil



}


//createStoreResolver handles creating a new store (CREATE)
func (h *Handler) createStoreResolver(p graphql.ResolveParams) (interface{}, error) {
	//Extract arguments
	name, nameOk := p.Args["name"].(string)
	revenue, revenueOk := p.Args["revenue"].(float64)
	active, activeOk := p.Args["active"].(bool)

	//Validate required fields
	if !nameOk || !revenueOk {
		h.logger.Error("invalid arguments for createStore")
		return nil, fmt.Errorf("invalid arguments: name and revenue are required")
	}

	//Default active to true if not provided
	if !activeOk {
		active = true
	}

	h.logger.Info("creating new store",
		"name", name,
		"revenue", revenue,
		"active", active,
	)

	//Insert into database and return the new ID
	var newID int
	query := "INSERT INTO stores (name, revenue, total_orders, active) VALUES ($1, $2, $3, $4) RETURNING id"
	err := h.database.QueryRow(query, name, revenue, 0, active).Scan(&newID)

	if err != nil {
		h.logger.Error("database error during insert",
			"name", name,
			"error", err.Error(),
		)
		return nil, err
	}

	h.logger.Info("store created successfully",
		"store_id", newID,
		"name", name,
	)

	//Invalidate cache for the new store
	if h.redis != nil {
		cacheKey := fmt.Sprintf("store:%d", newID)
		err := h.redis.Del(context.Background(), cacheKey).Err()
		if err != nil {
			h.logger.Warn("failed to invalidate cache after create",
				"store_id", newID,
				"error", err.Error(),
			)
		} else {
			h.logger.Info("cache invalidated after create",
				"store_id", newID,
			)
		}
	}


	//Return the created store
	return map[string]interface{}{
		"id":           newID,
		"name":         name,
		"revenue":      revenue,
		"total_orders": 0,
		"active":       active,
	}, nil
}

//edits a existing store (UPDATE)
func (h *Handler) updateStoreResolver(p graphql.ResolveParams) (interface{}, error){
	//Extract and Validate id
	id, idOk := p.Args["id"].(int)
	if !idOk {
		h.logger.Error("invalid id for updateStore")
		return nil, fmt.Errorf("invalid id")
	}

	//Extract optional fields
	name, _ := p.Args["name"].(string)
	revenue, _ := p.Args["revenue"].(float64)
	totalOrders, _ := p.Args["total_orders"].(int)
	active, _ := p.Args["active"].(bool)

	h.logger.Info("updating store",
		"store_id", id,
	)


	//Update the database
	query := "UPDATE stores SET name = $1, revenue = $2, total_orders = $3, active = $4 WHERE id = $5"
	result, err := h.database.Exec(query, name, revenue, totalOrders, active, id)

	if err != nil {
		h.logger.Error("database error during update",
			"store_id", id,
			"error", err.Error(),
		)
		return nil, err
	}


	//Check if any rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		h.logger.Warn("store not found for update",
			"store_id", id,
		)
		return nil, fmt.Errorf("store with id %d not found", id)
	}

	h.logger.Info("store updated successfully",
		"store_id", id,
	)

	//Invalidate cache for the updated store
	if h.redis != nil {
		cacheKey := fmt.Sprintf("store:%d", id)
		err := h.redis.Del(context.Background(), cacheKey).Err()
		if err != nil {
			h.logger.Warn("failed to invalidate cache after update",
				"store_id", id,
				"error", err.Error(),
			)
		} else {
			h.logger.Info("cache invalidated after update",
				"store_id", id,
			)
		}
	}


	//Fetch and return updated store
	var storeID int
	var storeName string
	var storeRevenue float64
	var storeTotalOrders int
	var storeActive bool


	selectQuery := "SELECT id, name, revenue, total_orders, active FROM stores WHERE id = $1"
	err = h.database.QueryRow(selectQuery, id).Scan(&storeID, &storeName, &storeRevenue, &storeTotalOrders, &storeActive)


	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"id":           storeID,
		"name":         storeName,
		"revenue":      storeRevenue,
		"total_orders": storeTotalOrders,
		"active":       storeActive,
	}, nil





}

//deleteStoreResolver handles deleting a store (DELETE)
func (h *Handler) deleteStoreResolver(p graphql.ResolveParams) (interface{}, error) {
	//Extract and validate ID
	id, idOk := p.Args["id"].(int)
	if !idOk {
		h.logger.Error("invalid id for deleteStore")
		return nil, fmt.Errorf("invalid id")
	}

	h.logger.Info("deleting store",
		"store_id", id,
	)

	//Delete from database
	query := "DELETE FROM stores WHERE id = $1"
	result, err := h.database.Exec(query, id)

	if err != nil {
		h.logger.Error("database error during delete",
			"store_id", id,
			"error", err.Error(),
		)
		return nil, err
	}

	//Check if any rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		h.logger.Warn("store not found for delete",
			"store_id", id,
		)
		return nil, fmt.Errorf("store with id %d not found", id)
	}

	h.logger.Info("store deleted successfully",
		"store_id", id,
	)

	//Invalidate cache for the deleted store
	if h.redis != nil {
		cacheKey := fmt.Sprintf("store:%d", id)
		err := h.redis.Del(context.Background(), cacheKey).Err()
		if err != nil {
			h.logger.Warn("failed to invalidate cache after delete",
				"store_id", id,
				"error", err.Error(),
			)
		} else {
			h.logger.Info("cache invalidated after delete",
				"store_id", id,
			)
		}
	}



	//Return success response
	return map[string]interface{}{
		"success": true,
		"id":      id,
	}, nil
}


func (h *Handler) healthCheck(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("health check endpoint called",
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr,
	)

	w.Write([]byte("OK"))

	h.logger.Info("health check response sent")
}

//stressTest triggers a k6 load test and returns results
func (h *Handler) stressTest(w http.ResponseWriter, r *http.Request) {
	//Only allow POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	//Simple API key check
	apiKey := r.Header.Get("X-API-Key")
	expectedKey := os.Getenv("STRESS_TEST_API_KEY")
	if expectedKey == "" {
		expectedKey = "dev-only-key" //Local development default
	}
	
	if apiKey != expectedKey {
		h.logger.Warn("unauthorized stress test attempt",
			"remote_addr", r.RemoteAddr,
		)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	h.logger.Info("stress test triggered",
		"remote_addr", r.RemoteAddr,
	)

	//Return immediate response (test runs async)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	
	response := map[string]interface{}{
		"status":  "accepted",
		"message": "Load test started. Check metrics in Grafana.",
		"grafana": "http://35.225.111.249:3000",
	}
	
	json.NewEncoder(w).Encode(response)

	//Run k6 test asynchronously
	go h.runK6Test()
}

// runK6Test executes a k6 load test script
func (h *Handler) runK6Test() {
	h.logger.Info("starting k6 load test")
	
	// Create a simple k6 script inline
	script := `
		import http from 'k6/http';
		import { check, sleep } from 'k6';

		export const options = {
		stages: [
			{ duration: '30s', target: 20 },
			{ duration: '1m', target: 20 },
			{ duration: '30s', target: 0 },
		],
		};

		export default function () {
		const res = http.get('` + os.Getenv("BASE_URL") + `/health');
		check(res, {
			'status is 200': (r) => r.status === 200,
		});
		sleep(1);
		}
		`

	// Write script to temporary file
	tmpFile, err := os.CreateTemp("", "k6-test-*.js")
	if err != nil {
		h.logger.Error("failed to create temp file", "error", err.Error())
		return
	}
	defer os.Remove(tmpFile.Name())
	
	if _, err := tmpFile.WriteString(script); err != nil {
		h.logger.Error("failed to write script", "error", err.Error())
		return
	}
	tmpFile.Close()

	// Execute k6
	cmd := exec.Command("k6", "run", tmpFile.Name())
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		h.logger.Error("k6 test failed",
			"error", err.Error(),
			"output", string(output),
		)
		return
	}
	
	h.logger.Info("k6 test completed successfully",
		"output_preview", string(output[:min(500, len(output))]),
	)
}


//min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}







//Define the store type in GraphQL
var storeType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Store",
	Fields: graphql.Fields{
		"id": &graphql.Field{Type: graphql.Int,},
		"name": &graphql.Field{Type: graphql.String,},
		"revenue": &graphql.Field{Type: graphql.Float,},
		"total_orders": &graphql.Field{Type: graphql.Int,},
		"active": &graphql.Field{Type: graphql.Boolean,},

	},
})

//Define the delete result type in GraphQL
var deleteResultType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DeleteResult",
	Fields: graphql.Fields{
		"success": &graphql.Field{Type: graphql.Boolean},
		"id":      &graphql.Field{Type: graphql.Int},
	},
})


//Function that creates the GraphQL schema with a Handler
func createSchema(h *Handler) (graphql.Schema, error) {
	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"store": &graphql.Field{
				Type: storeType,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
				},
				Resolve: h.storeResolver, //Use the Handler's method!
			},
			"stores": &graphql.Field{
				Type:    graphql.NewList(storeType),
				Resolve: h.storesResolver,
			},
		},
	})

	//Define mutations
	mutationType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Mutation",
		Fields: graphql.Fields{
			"createStore": &graphql.Field{
				Type: storeType,
				Args: graphql.FieldConfigArgument{
					"name": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					"revenue": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.Float),
					},
					"active": &graphql.ArgumentConfig{
						Type: graphql.Boolean,
					},
				},
				Resolve: h.createStoreResolver,
			},
			"updateStore": &graphql.Field{
				Type: storeType,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.Int),
					},
					"name": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"revenue": &graphql.ArgumentConfig{
						Type: graphql.Float,
					},
					"total_orders": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"active": &graphql.ArgumentConfig{
						Type: graphql.Boolean,
					},
				},
				Resolve: h.updateStoreResolver,  
			}, 
			"deleteStore": &graphql.Field{
				Type: deleteResultType,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.Int),
					},
				},
				Resolve: h.deleteStoreResolver,
			}, 
		},
	})

	return graphql.NewSchema(graphql.SchemaConfig{
		Query:    queryType,
		Mutation: mutationType,
	})
}




func main() {
	var err error

	//Register Prometheus metrics
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration, cacheHits, cacheMisses)
	fmt.Println("Prometheus metrics registered")

	//Initialize OpenTelemetry tracing
	tp, err := initTracer()
	if err != nil {
		log.Fatal("Failed to initialize tracer:", err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()
	fmt.Println("OpenTelemetry tracing initialized")


	//Connect to Postgres
	fmt.Print("Connecting to Postgres...")
	//Load environment variables from .env file (optional in production)
	err = godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables") //Warn instead of crash
	}

	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Fatal("DATABASE_URL not set in .env file")
	}

	//sql.Open generates the expensive connection from the connStr info and an error which will be nil if everything connects
	db, err = sql.Open("postgres", connStr)

	//if the error is not an all clear nil then we failed to connect
	if err != nil {
		fmt.Println("Failed to connect:", err)
		return
	}
	defer db.Close()

	//Test connection
	err = db.Ping()
	if err != nil {
		fmt.Println("Cannot reach database:", err)
		return
	}
	fmt.Println("Connected to Postgres!")

	//Register endpoints
	fmt.Print("Registering endpoints...")

	//Create base JSON handler
	baseHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	//Wrap with sampling (5% of INFO logs, 100% of WARN/ERROR)
	samplingHandler := &SamplingHandler{
		handler:      baseHandler,
		samplingRate: 0.05, //5% sampling for INFO logs 0.05, changed to 1.0 for testing
	}

	logger := slog.New(samplingHandler)
	logger.Info("logging initialized with 5% sampling for INFO level")



	//Initialize Redis
	redisClient := initRedis()

	storeHandler := &Handler{
		database: db,
		logger:   logger,
		redis:    redisClient,
	}

	http.Handle("/health", 
		otelhttp.NewHandler(
			prometheusMiddleware(http.HandlerFunc(storeHandler.healthCheck)),
			"GET /health",
		),
	)
	http.Handle("/store", 
		otelhttp.NewHandler(
			prometheusMiddleware(http.HandlerFunc(storeHandler.getStoreInfo)),
			"GET /store",
		),
	)
	http.Handle("/demo/stress-test",
		otelhttp.NewHandler(
			prometheusMiddleware(http.HandlerFunc(storeHandler.stressTest)),
			"POST /demo/stress-test",
		),
	)


	http.Handle("/metrics", promhttp.Handler())

	//Create the schema using our Handler
	schema, err := createSchema(storeHandler)
	if err != nil {
    	log.Fatal("Failed to create GraphQL schema:", err)
	}

	//Create GraphQL HTTP handler
	graphqlHandler := handler.New(&handler.Config{
		Schema:   &schema,
		Pretty:   true,
		GraphiQL: true,
	})

	//Wrap GraphQL handler with CORS middleware
	http.Handle("/graphql", corsMiddleware(graphqlHandler))
	//Root endpoint
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte("StickerMule Portfolio API | Endpoints: /health, /store, /graphql, /metrics | CI/CD: Active"))
	})





	fmt.Println("Endpoints registered!")
	

	fmt.Println("Server listening on http://localhost:8080")
	fmt.Println("GraphQL endpoint: http://localhost:8080/graphql")
	fmt.Println("Waiting for requests...")
	http.ListenAndServe(":8080", nil)
}
