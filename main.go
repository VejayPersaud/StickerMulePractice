package main

import (
	"database/sql"
	"fmt"
	"net/http"

	_ "github.com/lib/pq"
)

var db *sql.DB

func healthCheck(w http.ResponseWriter, r *http.Request) {
	fmt.Println("/health called")
	w.Write([]byte("OK"))
}

func getStoreInfo(w http.ResponseWriter, r *http.Request) {
	fmt.Println("/store called")

	storeID := r.URL.Query().Get("id")
	if storeID == "" {
		storeID = "1"
	}
	fmt.Println("Looking up store ID:", storeID)

	//Query Postgress
	var name string
	var revenue, totalOrders int
	var active bool

	query := "SELECT name, revenue, total_orders, active FROM stores WHERE id = $1"
	err := db.QueryRow(query, storeID).Scan(&name, &revenue, &totalOrders, &active)

	if err == sql.ErrNoRows {
		fmt.Println("Store not found\n")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "Store not found"}`))
		return
	} else if err != nil {
		fmt.Println("Database error:", err, "\n")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Database error"}`))
		return
	}

	response := fmt.Sprintf(
		`{"store_id": "%s", "name": "%s", "revenue": %d, "total_orders": %d, "active": %t}`,
		storeID, name, revenue, totalOrders, active,
	)

	fmt.Println("Sending:", response, "\n")
	w.Write([]byte(response))
}

func main() {
	var err error

	// Connect to Postgres
	fmt.Println("Connecting to Postgres...")
	connStr := "host=localhost port=5432 user=postgres password=postgres dbname=sticker_mule_practice sslmode=disable"

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
	fmt.Println("Connected to Postgres!\n")

	//Register endpoints
	fmt.Println("Registering endpoints")
	http.HandleFunc("/health", healthCheck)
	http.HandleFunc("/store", getStoreInfo)

	fmt.Println("Server listening on http://localhost:8080")
	fmt.Println("Waiting for requests...\n")
	http.ListenAndServe(":8080", nil)
}
