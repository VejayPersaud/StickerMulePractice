/* Testing
Business Logic Coverage:** 96.4% (getStoreInfo), 93.8% (storeResolver), 100% (healthCheck)
Overall Coverage:** 61.6% (includes untestable main() and library setup code)
Test Strategy:** Unit tests with mocked database dependencies
*/


package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"fmt"
	"io"          
	"log/slog"
	"database/sql"

	"github.com/graphql-go/graphql"
	"github.com/DATA-DOG/go-sqlmock"
)

func TestHealthCheck(t *testing.T) {
	//ARRANGE: Create a test logger (discard output)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	
	//ARRANGE: Create Handler with logger
	handler := &Handler{
		database: nil, // Health check doesn't use DB
		logger:   logger,
	}
	
	//ARRANGE: Create a fake HTTP request
	req := httptest.NewRequest("GET", "/health", nil)
	
	//ARRANGE: Create a fake HTTP response recorder
	w := httptest.NewRecorder()
	
	//ACT: Call the method
	handler.healthCheck(w, req)
	
	//ASSERT: Check the response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	body := w.Body.String()
	if body != "OK" {
		t.Errorf("Expected body 'OK', got '%s'", body)
	}
}



func TestGetStoreInfo_Success(t *testing.T) {
	//ARRANGE: Create a mock database
	fakeDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer fakeDB.Close()

	//ARRANGE: Tell the mock what to expect and what to return
	rows := sqlmock.NewRows([]string{"name", "revenue", "total_orders", "active"}).
		AddRow("Test Store", 99999.99, 500, true)

	mock.ExpectQuery("SELECT name, revenue, total_orders, active FROM stores WHERE id = \\$1").
		WithArgs("1").
		WillReturnRows(rows)

	//ARRANGE: Create Handler with mock database
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := &Handler{
		database: fakeDB,
		logger:   logger,
	}

	//ARRANGE: Create fake HTTP request
	req := httptest.NewRequest("GET", "/store?id=1", nil)
	w := httptest.NewRecorder()

	//ACT: Call the function
	handler.getStoreInfo(w, req)

	//ASSERT: Check the response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	expectedBody := `{"store_id": "1", "name": "Test Store", "revenue": 99999.99, "total_orders": 500, "active": true}`
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body:\n%s\n\nGot:\n%s", expectedBody, w.Body.String())
	}

	//ASSERT: Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}


func TestGetStoreInfo_NotFound(t *testing.T) {
	//ARRANGE: Create mock database
	fakeDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer fakeDB.Close()

	//ARRANGE: Mock will return "no rows" error
	mock.ExpectQuery("SELECT name, revenue, total_orders, active FROM stores WHERE id = \\$1").
		WithArgs("999").
		WillReturnError(sql.ErrNoRows)  // Simulate store not found

	//ARRANGE: Create Handler and request
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := &Handler{
		database: fakeDB,
		logger:   logger,
	}




	req := httptest.NewRequest("GET", "/store?id=999", nil)
	w := httptest.NewRecorder()

	//ACT: Call the function
	handler.getStoreInfo(w, req)

	//ASSERT: Should return 404
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	expectedBody := `{"error": "store not found"}`
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body:\n%s\n\nGot:\n%s", expectedBody, w.Body.String())
	}

	//ASSERT: Verify mock was called
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}



func TestGetStoreInfo_DatabaseError(t *testing.T) {
	//ARRANGE: Create mock database
	fakeDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer fakeDB.Close()

	//ARRANGE: Mock will return a generic database error
	mock.ExpectQuery("SELECT name, revenue, total_orders, active FROM stores WHERE id = \\$1").
		WithArgs("1").
		WillReturnError(fmt.Errorf("connection timeout"))  // Simulate DB failure

	//ARRANGE: Create Handler and request
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := &Handler{
		database: fakeDB,
		logger:   logger,
	}



	req := httptest.NewRequest("GET", "/store?id=1", nil)
	w := httptest.NewRecorder()

	//ACT: Call the function
	handler.getStoreInfo(w, req)

	//ASSERT: Should return 500
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	expectedBody := `{"error": "Database error"}`
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body:\n%s\n\nGot:\n%s", expectedBody, w.Body.String())
	}

	//ASSERT: Verify mock was called
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}


func TestStoreResolver_Success(t *testing.T) {
	//ARRANGE: Create mock database
	fakeDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer fakeDB.Close()

	//ARRANGE: Set up mock expectation
	rows := sqlmock.NewRows([]string{"id", "name", "revenue", "total_orders", "active"}).
		AddRow(1, "GraphQL Store", 75000.50, 300, true)

	mock.ExpectQuery("SELECT id, name, revenue, total_orders, active FROM stores WHERE id = \\$1").
		WithArgs(1).
		WillReturnRows(rows)

	//ARRANGE: Create Handler and GraphQL params
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := &Handler{
		database: fakeDB,
		logger:   logger,
	}



	params := graphql.ResolveParams{
		Args: map[string]interface{}{
			"id": 1,
		},
	}

	//ACT: Call the resolver
	result, err := handler.storeResolver(params)

	//ASSERT: Check no error
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	//ASSERT: Check result structure
	storeMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	//ASSERT: Check individual fields
	if storeMap["id"] != 1 {
		t.Errorf("Expected id=1, got %v", storeMap["id"])
	}
	if storeMap["name"] != "GraphQL Store" {
		t.Errorf("Expected name='GraphQL Store', got %v", storeMap["name"])
	}
	if storeMap["revenue"] != 75000.50 {
		t.Errorf("Expected revenue=75000.50, got %v", storeMap["revenue"])
	}

	//ASSERT: Verify mock expectations
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestStoreResolver_NotFound(t *testing.T) {
	//ARRANGE: Create mock database
	fakeDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer fakeDB.Close()

	//ARRANGE: Mock returns "no rows"
	mock.ExpectQuery("SELECT id, name, revenue, total_orders, active FROM stores WHERE id = \\$1").
		WithArgs(999).
		WillReturnError(sql.ErrNoRows)

	//ARRANGE: Create Handler and params
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := &Handler{
		database: fakeDB,
		logger:   logger,
	}



	params := graphql.ResolveParams{
		Args: map[string]interface{}{
			"id": 999,
		},
	}

	//ACT: Call the resolver
	result, err := handler.storeResolver(params)

	//ASSERT: Should return error
	if err == nil {
		t.Error("Expected error for store not found, got nil")
	}
	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	//ASSERT: Verify mock
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestStoreResolver_InvalidID(t *testing.T) {
	//ARRANGE: Create Handler (no DB needed for this test)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := &Handler{
		database: nil,  //No database needed, test fails before DB access
		logger:   logger,
	}
	
	//ARRANGE: Create params with invalid ID (string instead of int)
	params := graphql.ResolveParams{
		Args: map[string]interface{}{
			"id": "not-an-int",
		},
	}

	//ACT: Call the resolver
	result, err := handler.storeResolver(params)

	//ASSERT: Should return error
	if err == nil {
		t.Error("Expected error for invalid id type, got nil")
	}
	if err.Error() != "invalid id" {
		t.Errorf("Expected 'invalid id' error, got: %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}
}

func TestGetStoreInfo_DefaultID(t *testing.T) {
	//ARRANGE: Create mock database
	fakeDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer fakeDB.Close()

	//ARRANGE: Set up mock for default ID "1"
	rows := sqlmock.NewRows([]string{"name", "revenue", "total_orders", "active"}).
		AddRow("Default Store", 12345.67, 100, true)

	mock.ExpectQuery("SELECT name, revenue, total_orders, active FROM stores WHERE id = \\$1").
		WithArgs("1").  //Should default to 1 when no ID provided
		WillReturnRows(rows)

	//ARRANGE: Create Handler and request WITH NO ID PARAMETER
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := &Handler{
		database: fakeDB,
		logger:   logger,
	}




	req := httptest.NewRequest("GET", "/store", nil)  // No ?id=X
	w := httptest.NewRecorder()

	//ACT: Call the function
	handler.getStoreInfo(w, req)

	//ASSERT: Should succeed with default ID
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	expectedBody := `{"store_id": "1", "name": "Default Store", "revenue": 12345.67, "total_orders": 100, "active": true}`
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body:\n%s\n\nGot:\n%s", expectedBody, w.Body.String())
	}

	//ASSERT: Verify mock
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestCreateSchema_Success(t *testing.T) {
	//ARRANGE: Create a Handler with mock DB (schema creation doesn't use DB)
	fakeDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer fakeDB.Close()
	
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := &Handler{
		database: fakeDB,
		logger:   logger,
	}

	//ACT: Create the schema
	schema, err := createSchema(handler)

	//ASSERT: Schema should be created without error
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	//ASSERT: Schema should have a Query type
	if schema.QueryType() == nil {
		t.Error("Expected schema to have a Query type")
	}

	//ASSERT: Schema should have the "store" query field
	queryFields := schema.QueryType().Fields()
	if queryFields["store"] == nil {
		t.Error("Expected schema to have 'store' query field")
	}
}



// _____________________________________ GraphQL CRUD ________________________________________



//CREATE
func TestCreateStoreResolver_Success(t *testing.T){
	//ARRANGE: Create mock database
	fakeDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer fakeDB.Close()

	//ARRANGE: Expect INSERT query and return new ID
	mock.ExpectQuery("INSERT INTO stores").
	WithArgs("Brand New Store",25000.00,0,true).
	WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(99))

	//ARRANGE: Create handler and GraphQL params
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := &Handler{
		database: fakeDB,
		logger:   logger,
	}



	params := graphql.ResolveParams{
		Args: map[string]interface{}{
			"name": "Brand New Store",
			"revenue": 25000.00,
			"active": true,
		},
	}

	//ACT: Call the resolver
	result, err := handler.createStoreResolver(params)

	//ASSERT: Check success
	if err != nil {
		t.Errorf("Expected no error, got: %v ", err)
	}

	//ASSERT: Check returned data 
	storeMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be map")
	}

	if storeMap["id"] != 99 {
		t.Errorf("Expected id = 99, got %v", storeMap["id"])
	}
	if storeMap["name"] != "Brand New Store" {
		t.Errorf("Expected name = 'Brand New Store', got %v", storeMap["name"])
	}


	//ASSERT: Verify mock was even called
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}


}




//Update
func TestUpdateStoreResolver_Success(t *testing.T){
	//ARRANGE: Create mock database
	fakeDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer fakeDB.Close()

	//ARRANGE: Expected UPDATE query
	mock.ExpectExec("UPDATE stores SET (.+) WHERE id = \\$").
	WithArgs("Updated Store", 75000.00, 500, false, 1).
	WillReturnResult(sqlmock.NewResult(0,1))

	//ARRANGE: Expect SELECT query to return updated data
	rows := sqlmock.NewRows([]string{"id","name","revenue","total_orders","active"}).
	AddRow(1,"Updated Store", 75000.00, 500, false)

	mock.ExpectQuery("SELECT id, name, revenue, total_orders, active FROM stores WHERE id = \\$1").
	WithArgs(1).
	WillReturnRows(rows)

	//ARRANGE: Create Handler and GraphQL params
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := &Handler{
		database: fakeDB,
		logger:   logger,
	}



	params := graphql.ResolveParams{
		Args: map[string]interface{}{
			"id":		1,
			"name":		"Updated Store",
			"revenue":		75000.00,
			"total_orders":		500,
			"active":		false,
		},
	}

	//ACT: Call the resolver
	result, err := handler.updateStoreResolver(params)

	//ASSERT: Check success
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	//ASSERT: Check returned data
	storeMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	if storeMap["id"] != 1 {
		t.Errorf("Expected id=1, got %v", storeMap["id"])
	}
	if storeMap["name"] != "Updated Store" {
		t.Errorf("Expected name='Updated Store', got %v", storeMap["name"])
	}
	if storeMap["revenue"] != 75000.00 {
		t.Errorf("Expected revenue=75000.00, got %v", storeMap["revenue"])
	}

	//ASSERT: Check mock expectations
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v",err)
	}


 

}


//Delete
func TestDeleteStoreResolver_Success(t *testing.T) {
	//ARRANGE: Create mock database
	fakeDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer fakeDB.Close()

	//ARRANGE: Expect DELETE query
	mock.ExpectExec("DELETE FROM stores WHERE id = \\$1").
		WithArgs(99).
		WillReturnResult(sqlmock.NewResult(0, 1)) // 1 row affected

	//ARRANGE: Create Handler and GraphQL params
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := &Handler{
		database: fakeDB,
		logger:   logger,
	}


	params := graphql.ResolveParams{
		Args: map[string]interface{}{
			"id": 99,
		},
	}

	//ACT: Call the resolver
	result, err := handler.deleteStoreResolver(params)

	//ASSERT: Check success
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	//ASSERT: Check result
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	if resultMap["success"] != true {
		t.Errorf("Expected success=true, got %v", resultMap["success"])
	}
	if resultMap["id"] != 99 {
		t.Errorf("Expected id=99, got %v", resultMap["id"])
	}

	//ASSERT: Verify mock expectations
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestDeleteStoreResolver_NotFound(t *testing.T) {
	//ARRANGE: Create mock database
	fakeDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer fakeDB.Close()

	//ARRANGE: Expect DELETE query with 0 rows affected
	mock.ExpectExec("DELETE FROM stores WHERE id = \\$1").
		WithArgs(999).
		WillReturnResult(sqlmock.NewResult(0, 0)) // 0 rows affected

	//ARRANGE: Create Handler and GraphQL params
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := &Handler{
		database: fakeDB,
		logger:   logger,
	}


	params := graphql.ResolveParams{
		Args: map[string]interface{}{
			"id": 999,
		},
	}

	//ACT: Call the resolver
	result, err := handler.deleteStoreResolver(params)

	//ASSERT: Should return error
	if err == nil {
		t.Error("Expected error for store not found, got nil")
	}
	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	//ASSERT: Verify mock expectations
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}