# Testing Remote Options Implementation

This guide walks you through testing the remote options feature for flowctl select inputs.

## Quick Start

### 1. Run Unit Tests

First, run the comprehensive unit tests to verify the implementation:

```bash
cd e:/gsoc/flowctl

# Run all options tests
go test -v ./internal/utils

# Or run individual test suites
go test -v ./internal/utils -run TestInterpolateVariables
go test -v ./internal/utils -run TestFetchRemoteOptions
go test -v ./internal/utils -run TestFetchRemoteOptionsWithVariables
```

**Expected Output:**
```
=== RUN   TestInterpolateVariables
=== RUN   TestInterpolateVariables/no_variables
=== RUN   TestInterpolateVariables/single_variable
=== RUN   TestInterpolateVariables/multiple_variables
=== RUN   TestInterpolateVariables/missing_variable
... (all tests should PASS)
```

### 2. Start Mock API Server

In a separate terminal, start the mock API server that provides test options:

```bash
# From project root
node test-api-server.js
```

**Expected Output:**
```
📡 Mock API Server listening on http://localhost:3000

📍 Available endpoints:
...
```

### 3. Test the Mock API Directly

While the server is running, test the API endpoints:

```bash
# Test basic endpoint
curl "http://localhost:3000/api/environments"

# Test endpoint with variable interpolation
curl "http://localhost:3000/api/cities?country=Canada"

# Test with multiple parameters
curl "http://localhost:3000/api/regions?city=Toronto&country=Canada"
```

## Manual Testing with flowctl

### Test Scenario 1: Simple Remote Options

1. **Create a test flow** with a select input using remote options:

```yaml
metadata:
  id: test_remote_options_basic
  name: Test Remote Options
  
inputs:
  - name: environment
    type: select
    label: Environment
    required: true
    options_url: "http://localhost:3000/api/environments"

actions:
  - id: echo_env
    name: Echo Environment
    executor: script
    with:
      script: |
        echo "Selected environment: {{environment}}"
```

2. **Save this flow** and create it in flowctl

3. **Check flow inputs** endpoint:
```bash
curl http://localhost:8080/api/flows/test_remote_options_basic/inputs
```

Should return options fetched from the remote API.

### Test Scenario 2: Variable Interpolation

1. **Create a flow with dependent inputs**:

```yaml
metadata:
  id: test_variable_interpolation
  name: Test Variable Interpolation
  
inputs:
  - name: country
    type: select
    label: Country
    required: true
    options:
      - "United States"
      - "Canada"
      - "Mexico"

  - name: city
    type: select
    label: City (depends on country)
    required: true
    options_url: "http://localhost:3000/api/cities?country={{country}}"

actions:
  - id: display_selection
    name: Display Selection
    executor: script
    with:
      script: |
        echo "Country: {{country}}"
        echo "City: {{city}}"
```

2. **Test through the UI**:
   - Load the flow in the web interface
   - Select a country from the static dropdown
   - Observe that the city dropdown fetches options based on the country
   - Verify options are correctly loaded

3. **Test through the API**:
```bash
# Trigger flow with specific values
curl -X POST http://localhost:8080/api/flows/test_variable_interpolation/trigger \
  -d "country=Canada&city=Toronto"
```

### Test Scenario 3: Multiple Variables and Regions

1. **Create a cascading selection flow**:

```yaml
metadata:
  id: test_cascading_selection
  name: Test Cascading Selection
  
inputs:
  - name: country
    type: select
    label: Country
    required: true
    options:
      - "United States"
      - "Canada"
      - "Mexico"

  - name: city
    type: select
    label: City
    required: true
    options_url: "http://localhost:3000/api/cities?country={{country}}"

  - name: region
    type: select
    label: Region
    required: true
    options_url: "http://localhost:3000/api/regions?country={{country}}&city={{city}}"

actions:
  - id: show_all
    name: Show Selection
    executor: script
    with:
      script: |
        echo "Country: {{country}}"
        echo "City: {{city}}"
        echo "Region: {{region}}"
```

2. **Test the cascading behavior**:
   - Load flow in UI
   - Select country → city dropdown updates
   - Select city → region dropdown updates with both values

## Test Cases

### Unit Test Cases (Already Automated)

1. **Variable Interpolation Tests**
   - ✓ URL with no variables
   - ✓ URL with single variable
   - ✓ URL with multiple variables
   - ✓ Missing variable error handling
   - ✓ Numeric variable interpolation
   - ✓ Boolean variable interpolation
   - ✓ Variables with spaces

2. **Remote Options Fetching Tests**
   - ✓ Successful fetch
   - ✓ Empty options array
   - ✓ Options with empty names skipped
   - ✓ HTTP error handling
   - ✓ Invalid JSON response
   - ✓ Empty URL handling

3. **Integration Tests**
   - ✓ Variable interpolation + remote fetch combined
   - ✓ Multiple variables with remote fetch
   - ✓ Missing variable error with remote fetch

### Manual Test Cases

1. **Backend Functionality**
   - [ ] Flow creation with options_url succeeds
   - [ ] GET /api/flows/{id}/inputs returns options_url field
   - [ ] Variable interpolation happens before fetch
   - [ ] Remote options are merged with static options
   - [ ] Flow validation uses merged options
   - [ ] Graceful fallback when remote fetch fails

2. **Frontend Functionality**
   - [ ] options_url field appears in select input editor
   - [ ] FlowInputFields component detects options_url
   - [ ] Loading spinner shows while fetching
   - [ ] Options are correctly rendered after fetch
   - [ ] Variable interpolation works in frontend
   - [ ] Mixed static and remote options display correctly

3. **Error Cases**
   - [ ] Invalid URL format handled gracefully
   - [ ] Network timeout doesn't crash UI
   - [ ] Malformed JSON response handled
   - [ ] HTTP errors logged and static options used
   - [ ] Missing variables show validation error

## Debugging

### Enable Verbose Logging

For backend debugging, you can check logs:

```bash
# View handler logs
FLOWCTL_LOG_LEVEL=debug go run main.go
```

### Check Frontend Network Activity

In browser developer tools:
1. Open DevTools (F12)
2. Go to Network tab
3. Look for requests to `http://localhost:3000/api/*`
4. Verify response contains expected option objects

### Inspect Fetched Options

In Firefox/Chrome console:
```javascript
// Test fetch directly
fetch('http://localhost:3000/api/cities?country=Canada')
  .then(r => r.json())
  .then(data => console.log(data))
```

## Expected Results

### Successful Unit Tests
All 12 test functions should pass:
- 7 InterpolateVariables tests
- 5 FetchRemoteOptions tests  
- 3 FetchRemoteOptionsWithVariables tests

### Successful Integration
When creating and triggering a flow:
1. Remote options are fetched on flow info retrieval
2. Options are merged with static options if present
3. Variable interpolation replaces `{{variable}}` with actual values
4. Selected value is validated against merged options
5. Flow executes successfully with remote-sourced options

## Cleanup

After testing:

1. **Stop the mock API server**:
   ```bash
   # Ctrl+C in the terminal running test-api-server.js
   ```

2. **Stop flowctl**:
   ```bash
   # Ctrl+C in the terminal running flowctl
   ```

3. **Clean up test flows**:
   - Delete test flows from the database or filesystem

## Troubleshooting

### "Connection refused" error
- Ensure mock API server is running: `node test-api-server.js`
- Check port 3000 is not in use: `netstat -an | grep 3000`

### Remote options not appearing
- Check network tab in browser DevTools
- Verify API response format: `[{"name": "...", "selected": false}]`
- Check backend logs for interpolation errors

### Variable interpolation not working
- Ensure variable names in URL match input names exactly
- Check for typos in `{{variable_name}}`
- Verify variable values are actually provided when triggering flow

### Static options not showing
- Must provide both `options:` and `options_url:` for fallback
- Check YAML indentation
- Verify options are string array

## Performance Testing

To test with large option sets:

```javascript
// Modify test-api-server.js to return many options
const manyOptions = Array.from({ length: 1000 }, (_, i) => ({ 
  name: `option-${i}` 
}));
```

Then test:
- Load time with 1000+ options
- UI responsiveness during fetch
- Memory usage

---

For more information, see [REMOTE_OPTIONS_EXAMPLE.md](REMOTE_OPTIONS_EXAMPLE.md) and [IMPLEMENTATION_SUMMARY.md](IMPLEMENTATION_SUMMARY.md)
