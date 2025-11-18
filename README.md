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