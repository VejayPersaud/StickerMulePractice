package main

import (
	"database/sql"
	"fmt"
	"log"
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
}

//Method on Handler, can create test Handler with mock db
func(h *Handler) getStoreInfo(w http.ResponseWriter, r *http.Request) {
	fmt.Println("/store called")
	fmt.Println("Request Details: ")
	fmt.Println("  - Method:", r.Method)
	fmt.Println("  -Path:", r.URL.Path)
	fmt.Println("  -From:", r.RemoteAddr)

	storeID := r.URL.Query().Get("id")
	if storeID == "" {
		storeID = "1"
	}
	fmt.Println("Looking up store ID in database:", storeID)

	//Query Postgres
	var name string
	var revenue float64
	var totalOrders int
	var active bool

	query := "SELECT name, revenue, total_orders, active FROM stores WHERE id = $1"
	err := h.database.QueryRow(query, storeID).Scan(&name, &revenue, &totalOrders, &active)

	if err == sql.ErrNoRows {
		fmt.Println("Store not found")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "Store not found"}`))
		return
	} else if err != nil {
		fmt.Println("Database error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Database error"}`))
		return
	}

	response := fmt.Sprintf(
		`{"store_id": "%s", "name": "%s", "revenue": %.2f, "total_orders": %d, "active": %t}`,
		storeID, name, revenue, totalOrders, active,
	)

	fmt.Println("Sending:", response)
	w.Write([]byte(response))



}

//Method that returns a GraphQL resolver function
func (h *Handler) storeResolver(p graphql.ResolveParams) (interface{}, error){
	//Extract ID from query
	id, ok := p.Args["id"].(int)
	if !ok {
		return nil, fmt.Errorf("invalid id")
	}

	fmt.Printf("GraphQL querying store with id = %d \n", id)

	//Query database - using h.database instead of global db
	var storeID int
	var name string
	var revenue float64
	var totalOrders int
	var active bool

	query := "SELECT id, name, revenue, total_orders, active FROM stores WHERE id = $1"
	err := h.database.QueryRow(query, id).Scan(&storeID, &name, &revenue, &totalOrders, &active)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("Store not found")
	}

	if err != nil {
		return nil, err
	}

	//Return as a map
	return map[string]interface{}{
		"id":           storeID,
		"name":         name,
		"revenue":      revenue,
		"total_orders": totalOrders,
		"active":       active,
	}, nil



}


//createStoreResolver handles creating a new store
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


func (h *Handler) updateStoreResolver(p graphql.ResolveParams) (interface{}, error)




func healthCheck(w http.ResponseWriter, r *http.Request) {
	fmt.Println("/health called")
	fmt.Println("Request Details: ")
	fmt.Println("  - Method:", r.Method)
	fmt.Println("  -Path:", r.URL.Path)
	fmt.Println("  -From:", r.RemoteAddr)

	fmt.Print("Sending response: OK"+" ... ")
	w.Write([]byte("OK"))
	fmt.Println("Response sent!")
	
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
		Name:"Mutation",
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
		},
	})

    

	return graphql.NewSchema(graphql.SchemaConfig{
		Query: queryType,
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
	http.HandleFunc("/health", healthCheck)


	storeHandler := &Handler{database: db}
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
