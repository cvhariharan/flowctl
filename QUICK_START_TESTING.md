# Quick Start - Testing Remote Options

## Overview

This guide helps you quickly test the remote options implementation for flowctl. Several test files and utilities have been created.

## Files Created for Testing

### 1. Unit Tests
- **`internal/utils/options_test.go`** - Comprehensive Go unit tests
  - 7 tests for variable interpolation
  - 5 tests for remote options fetching
  - 3 tests for combined functionality
  - Can be run with: `go test -v ./internal/utils`

### 2. Mock API Server
- **`test-api-server.js`** - Node.js mock API server
  - Provides test endpoints for cities, regions, and services
  - Simulates variable-dependent option lists
  - Start with: `node test-api-server.js`
  - Runs on localhost:3000

### 3. Test Scripts
- **`test-options.bat`** (Windows) - Runs all Go unit tests
- **`test-options.sh`** (Linux/Mac) - Runs all Go unit tests
- **`test-api-calls.bat`** (Windows) - Tests mock API endpoints
- **`example-remote-options-flow.yaml`** - Sample flow configuration

### 4. Documentation
- **`TESTING_GUIDE.md`** - Comprehensive testing guide with manual test cases
- **`REMOTE_OPTIONS_EXAMPLE.md`** - API format and usage examples
- **`IMPLEMENTATION_SUMMARY.md`** - Technical implementation details

---

## Step-by-Step Testing

### Prerequisites

- Go 1.21+ installed and in PATH
- Node.js installed (for mock API server)
- curl installed (for API testing)
- Your preferred code editor

### Step 1: Run Unit Tests

**Option A: Run All Tests**
```bash
cd e:/gsoc/flowctl
go test -v ./internal/utils
```

**Option B: Run Specific Test Suite**
```bash
# Test variable interpolation
go test -v ./internal/utils -run TestInterpolateVariables

# Test remote option fetching
go test -v ./internal/utils -run TestFetchRemoteOptions

# Test integrated functionality
go test -v ./internal/utils -run TestFetchRemoteOptionsWithVariables
```

**Expected Output:**
```
=== RUN   TestInterpolateVariables
=== RUN   TestInterpolateVariables/no_variables
    --- PASS: TestInterpolateVariables/no_variables (0.00s)
=== RUN   TestInterpolateVariables/single_variable
    --- PASS: TestInterpolateVariables/single_variable (0.00s)
...
ok      github.com/cvhariharan/flowctl/internal/utils   0.042s
```

### Step 2: Start Mock API Server

In a new terminal:
```bash
cd e:/gsoc/flowctl
node test-api-server.js
```

**Expected Output:**
```
📡 Mock API Server listening on http://localhost:3000

📍 Available endpoints:

  GET /api/environments
    Returns: development, staging, production

  GET /api/cities?country=<country>
    Example: http://localhost:3000/api/cities?country=United%20States
    Returns: Cities in the specified country

  GET /api/regions?city=<city>&country=<country>
    Example: http://localhost:3000/api/regions?city=New%20York&country=United%20States
    Returns: Regions for the specified city and country

  GET /api/services?env=<environment>
    Example: http://localhost:3000/api/services?env=production
    Returns: Services available in the specified environment

  GET /health
    Returns: health check status
```

### Step 3: Test API Endpoints

In another terminal:
```bash
# Test basic endpoint
curl http://localhost:3000/api/environments

# Test with parameter
curl "http://localhost:3000/api/cities?country=Canada"

# Test multiple parameters
curl "http://localhost:3000/api/regions?city=Toronto&country=Canada"

# Test production services
curl "http://localhost:3000/api/services?env=production"
```

**Expected Output for Cities:**
```json
[
  {"name":"Toronto"},
  {"name":"Vancouver"},
  {"name":"Montreal"},
  {"name":"Calgary"}
]
```

### Step 4: Manual Integration Test

1. **Build and start flowctl server** (in a third terminal):
```bash
cd e:/gsoc/flowctl
go build
./flowctl start
```

2. **Create a test flow** with the sample configuration:
   - Save the content of `example-remote-options-flow.yaml`
   - Create it through the flowctl UI or API

3. **Test through the Web UI**:
   - Navigate to the flow
   - Verify that:
     - Select input shows "Remote Options URL" field
     - You can set an options_url value
     - Options are fetched when the flow is loaded

4. **Test through the API**:
```bash
# Get flow inputs (should include options_url)
curl http://localhost:8080/api/flows/test_remote_options_basic/inputs

# Trigger the flow with remote options
curl -X POST http://localhost:8080/api/flows/test_remote_options_basic/trigger \
  -d "environment=production"
```

---

## Test Outputs

### Successful Unit Tests
```
=== RUN   TestInterpolateVariables
=== RUN   TestInterpolateVariables/no_variables
    --- PASS: TestInterpolateVariables/no_variables (0.00s)
=== RUN   TestInterpolateVariables/single_variable
    --- PASS: TestInterpolateVariables/single_variable (0.00s)
=== RUN   TestInterpolateVariables/multiple_variables
    --- PASS: TestInterpolateVariables/multiple_variables (0.00s)
... (all tests passing)

=== RUN   TestFetchRemoteOptions
=== RUN   TestFetchRemoteOptions/successful_fetch
    --- PASS: TestFetchRemoteOptions/successful_fetch (0.01s)
... (all tests passing)

=== RUN   TestFetchRemoteOptionsWithVariables
=== RUN   TestFetchRemoteOptionsWithVariables/successful_interpolation_and_fetch
    --- PASS: TestFetchRemoteOptionsWithVariables/successful_interpolation_and_fetch (0.01s)
... (all tests passing)

ok      github.com/cvhariharan/flowctl/internal/utils   0.150s
```

### Successful API Response
```json
[
  {"name": "option1", "selected": false},
  {"name": "option2", "selected": true},
  {"name": "option3", "selected": false}
]
```

---

## Troubleshooting

### "go: command not found"
- Install Go from https://golang.org/dl/
- Add Go to your PATH environment variable
- Verify with `go version`

### "node: command not found"  
- Install Node.js from https://nodejs.org/
- Verify with `node --version`

### "Connection refused" for API server
- Ensure mock server is running: `node test-api-server.js`
- Check port 3000 is available: no other service using it
- Try another port: `PORT=4000 node test-api-server.js`

### "Port 3000 already in use"
```bash
# Linux/Mac: Find and kill process
lsof -i :3000
kill -9 <PID>

# Windows: Kill via Command Prompt
netstat -ano | findstr :3000
taskkill /PID <PID> /F
```

### Remote options not showing in UI
1. Check browser console for fetch errors
2. Verify options_url format matches: `https://api.example.com/options?param={{variable}}`
3. Check backend logs for interpolation errors
4. Verify mock API is running and responds to curl requests

---

## Next Steps

After testing:

1. **Read Full Documentation**:
   - [TESTING_GUIDE.md](TESTING_GUIDE.md) - Comprehensive manual test cases
   - [REMOTE_OPTIONS_EXAMPLE.md](REMOTE_OPTIONS_EXAMPLE.md) - API format and usage
   - [IMPLEMENTATION_SUMMARY.md](IMPLEMENTATION_SUMMARY.md) - Technical details

2. **Try Advanced Scenarios**:
   - Create flows with cascading selects (city depends on country)
   - Test with large option sets (1000+ options)
   - Test error handling (invalid URLs, timeouts)

3. **Integrate into Your Workflows**:
   - Replace mock API with your actual backend
   - Test with production data
   - Monitor performance with real-world option counts

---

## Quick Reference

| Test | Command | Result |
|------|---------|--------|
| All unit tests | `go test -v ./internal/utils` | 12 tests pass |
| Variable interpolation | `go test -v ./internal/utils -run TestInterpolateVariables` | 7 tests pass |
| Remote fetching | `go test -v ./internal/utils -run TestFetchRemoteOptions` | 5 tests pass |
| Integration | `go test -v ./internal/utils -run TestFetchRemoteOptionsWithVariables` | 3 tests pass |
| Mock API | `node test-api-server.js` | Server running on :3000 |
| Test API | `curl http://localhost:3000/api/environments` | JSON array returned |

---

For detailed testing procedures, see [TESTING_GUIDE.md](TESTING_GUIDE.md)
