package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"fmt"
	"database/sql"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHealthCheck(t *testing.T) {

	fmt.Println("DEBUGDEBUG NOW RUNNING TESTHEALTHCHECK")

	//t.Error("something failed") - marks test as failed
    //t.Fatal("critical failure") - stops test immediately 

	//ARRANGE: Create a fake HTTP request

	//Creates a fake HTTP request,no real network call
	req := httptest.NewRequest("GET", "/health", nil)
	
	//ARRANGE: Create a fake HTTP response recorder to capture what gets written
	//can check w.Code, w.Body, w.Header later
	w := httptest.NewRecorder()
	
	//ACT: Call the function we're testing with the fake request/response
	healthCheck(w, req)
	
	//ASSERT: Check the response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	body := w.Body.String()
	if body != "OK" {
		t.Errorf("Expected body 'OK', got '%s'", body)
	}

	fmt.Println("DEBUGDEBUG END OF TESTHEALTHCHECK")


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
	handler := &Handler{database: fakeDB}

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

	// ASSERT: Verify all expectations were met
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
	handler := &Handler{database: fakeDB}
	req := httptest.NewRequest("GET", "/store?id=999", nil)
	w := httptest.NewRecorder()

	//ACT: Call the function
	handler.getStoreInfo(w, req)

	//ASSERT: Should return 404
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	expectedBody := `{"error": "Store not found"}`
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body:\n%s\n\nGot:\n%s", expectedBody, w.Body.String())
	}

	//ASSERT: Verify mock was called
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}