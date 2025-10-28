package main

import (
	"fmt"
	"net/http" //HTTP communication tools
)

var storeDatabase = map[string]map[string]interface{}{
	"1": {
		"name":         "Awesome Stickers",
		"revenue":      50000,
		"total_orders": 342,
		"active":       true,
	},
	"2": {
		"name":         "Cool Decals",
		"revenue":      75000,
		"total_orders": 521,
		"active":       true,
	},
	"3": {
		"name":         "Vintage Prints",
		"revenue":      30000,
		"total_orders": 189,
		"active":       false,
	},
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	// TODO: You'll implement this
	fmt.Println("healthCheck function was called!")
	fmt.Println("Request Details: ")
	fmt.Println("  - Method:", r.Method)
	fmt.Println("  -Path:", r.URL.Path)
	fmt.Println("  -From:", r.RemoteAddr)

	fmt.Println("Sending response: OK")
	w.Write([]byte("OK"))
	fmt.Println("Response sent!\n")

}

func getStoreInfo(w http.ResponseWriter, r *http.Request) {
	fmt.Println("getStoreInfo function was called!")
	fmt.Println("  - Method:", r.Method)
	fmt.Println("  -Path:", r.URL.Path)
	fmt.Println("  -From:", r.RemoteAddr)

	//Get the store ID from the URL
	//Example: /store?id=123

	storeID := r.URL.Query().Get("id")
	if storeID == "" {
		storeID = "1"
	}
	fmt.Println("Store ID:", storeID)

	//Look up store by id in our database
	storeData, exists := storeDatabase[storeID]
	if !exists {
		fmt.Println("Store not found!")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "Store not found"}`))
		return
	}

	//Building response 1
	response := fmt.Sprintf(
		`{"store_id": "%s", "name": "%s", "revenue": %v, "total_orders": %v, "active": %v }`,
		storeID,
		storeData["name"],
		storeData["revenue"],
		storeData["total_orders"],
		storeData["active"],
	)

	fmt.Println("Sending store data:", response)
	w.Write([]byte(response))
	fmt.Println("Store data sent!\n")

}

func main() {
	fmt.Println("Server is starting...")
	fmt.Println("Registering /health endpoint")
	http.HandleFunc("/health", healthCheck)

	fmt.Println("Registering /store endpoint")
	http.HandleFunc("/store", getStoreInfo)

	fmt.Println("Listening on http://localhost:8080")
	fmt.Println("Try visiting: http://localhost:8080/health")
	fmt.Println("Waiting for requests...\n")
	http.ListenAndServe(":8080", nil)
}
