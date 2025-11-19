package main

import (
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
	
)

var db *sql.DB

//Handler is a struct that holds dependencies
type Handler struct {
	database *sql.DB
	logger *slog.Logger
}

//Method on Handler, can create test Handler with mock db
func(h *Handler) getStoreInfo(w http.ResponseWriter, r *http.Request) {

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


	//Query Postgres
	var name string
	var revenue float64
	var totalOrders int
	var active bool

	query := "SELECT name, revenue, total_orders, active FROM stores WHERE id = $1"
	err := h.database.QueryRow(query, storeID).Scan(&name, &revenue, &totalOrders, &active)

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

	h.logger.Info("store query successful",
		"store_id", storeID,
		"name", name,
		"revenue", revenue,
	)
	w.Write([]byte(response))



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
		return nil, fmt.Errorf("invalid arguments: name and revenue are required")
	}

	//Default active to true if not provided
	if !activeOk {
		active = true
	}

	fmt.Printf("GraphQL creating store: name=%s, revenue=%.2f, active=%t\n", name, revenue, active)

	//Insert into database and return the new ID
	var newID int
	query := "INSERT INTO stores (name, revenue, total_orders, active) VALUES ($1, $2, $3, $4) RETURNING id"
	err := h.database.QueryRow(query, name, revenue, 0, active).Scan(&newID)

	if err != nil {
		fmt.Println("Database error during insert:", err)
		return nil, err
	}

	fmt.Printf("Successfully created store with ID=%d\n", newID)

	//Return the created store
	return map[string]interface{}{
		"id":           newID,
		"name":         name,
		"revenue":      revenue,
		"total_orders": 0,
		"active":       active,
	}, nil
}

//edits a existing store
func (h *Handler) updateStoreResolver(p graphql.ResolveParams) (interface{}, error){
	//Extract and Validate id
	id, idOk := p.Args["id"].(int)
	if !idOk {
		return nil, fmt.Errorf("invalid id")
	}

	//Extract optional fields
	name, _ := p.Args["name"].(string)
	revenue, _ := p.Args["revenue"].(float64)
	totalOrders, _ := p.Args["total_orders"].(int)
	active, _ := p.Args["active"].(bool)

	fmt.Printf("GraphQL updating store: id=%d\n", id)


	//Update the database
	query := "UPDATE stores SET name = $1, revenue = $2, total_orders = $3, active = $4 WHERE id = $5"
	result, err := h.database.Exec(query, name, revenue, totalOrders, active, id)

	if err != nil {
		fmt.Println("Database error during update:", err)
		return nil, err
	}


	// Check if any rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		return nil, fmt.Errorf("store with id %d not found", id)
	}

	fmt.Printf("Successfully updated store with ID=%d\n", id)


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

//deleteStoreResolver handles deleting a store
func (h *Handler) deleteStoreResolver(p graphql.ResolveParams) (interface{}, error) {
	//Extract and validate ID
	id, idOk := p.Args["id"].(int)
	if !idOk {
		return nil, fmt.Errorf("invalid id")
	}

	fmt.Printf("GraphQL deleting store: id=%d\n", id)

	//Delete from database
	query := "DELETE FROM stores WHERE id = $1"
	result, err := h.database.Exec(query, id)

	if err != nil {
		fmt.Println("Database error during delete:", err)
		return nil, err
	}

	//Check if any rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		return nil, fmt.Errorf("store with id %d not found", id)
	}

	fmt.Printf("Successfully deleted store with ID=%d\n", id)

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

	// Connect to Postgres
	fmt.Print("Connecting to Postgres...")
	// Load environment variables from .env file
	err = godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
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

	//Structure JSON logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	storeHandler := &Handler{
		database:		db,
		logger:		logger,		
	}

	http.HandleFunc("/health", storeHandler.healthCheck)
	http.HandleFunc("/store", storeHandler.getStoreInfo)


	//Create the schema using our Handler
	schema, err := createSchema(storeHandler)
	if err != nil {
    	log.Fatal("Failed to create GraphQL schema:", err)
	}

	//Create GraphQL HTTP handler
	graphqlHandler := handler.New(&handler.Config{
    	Schema: &schema,
    	Pretty: true,
    	GraphiQL: true,
	})
	http.Handle("/graphql", graphqlHandler)





	fmt.Println("Endpoints registered!")
	

	fmt.Println("Server listening on http://localhost:8080")
	fmt.Println("GraphQL endpoint: http://localhost:8080/graphql")
	fmt.Println("Waiting for requests...")
	http.ListenAndServe(":8080", nil)
}
