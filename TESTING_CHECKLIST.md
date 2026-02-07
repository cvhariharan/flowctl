# Testing Checklist

Use this checklist to systematically test the remote options feature.

## Pre-Testing Setup

- [ ] Navigate to project directory: `cd e:/gsoc/flowctl`
- [ ] Verify Go is installed: `go version`
- [ ] Verify Node.js is installed: `node --version`
- [ ] Verify curl is available: `curl --version`

## Phase 1: Unit Tests (5 minutes)

### Run Tests
- [ ] Open terminal
- [ ] Run: `go test -v ./internal/utils -timeout 10s`

### Verify Results
- [ ] All tests show "PASS"
- [ ] No failures or errors
- [ ] Execution time < 1 second
- [ ] Test count = 15+ tests

### Expected Test Cases
- [ ] TestInterpolateVariables appears
- [ ] TestFetchRemoteOptions appears
- [ ] TestFetchRemoteOptionsWithVariables appears

## Phase 2: Mock API Server (5 minutes)

### Start Server
- [ ] Open new terminal
- [ ] Run: `node test-api-server.js`
- [ ] See message: "📡 Mock API Server listening on http://localhost:3000"

### Verify Server Health
- [ ] Can access endpoints list
- [ ] Server shows "Available endpoints"
- [ ] Health check available at /health
- [ ] All 4 main endpoints listed:
  - [ ] /api/environments
  - [ ] /api/cities
  - [ ] /api/regions
  - [ ] /api/services

### Keep Server Running
- [ ] Do NOT close this terminal
- [ ] Note the port (3000)
- [ ] Remember to Ctrl+C to stop later

## Phase 3: API Testing (5 minutes)

In a new terminal:

### Test 1: Environments Endpoint
```bash
curl http://localhost:3000/api/environments
```
- [ ] Returns JSON array
- [ ] Contains development, staging, production
- [ ] Valid JSON format

### Test 2: Cities by Country
```bash
curl "http://localhost:3000/api/cities?country=Canada"
```
- [ ] Returns JSON array
- [ ] Contains: Toronto, Vancouver, Montreal, Calgary
- [ ] Each object has "name" field

### Test 3: Services by Environment
```bash
curl "http://localhost:3000/api/services?env=production"
```
- [ ] Returns JSON array
- [ ] Contains service names
- [ ] Includes "selected" field
- [ ] At least one has selected=true

### Test 4: Regions with Multiple Parameters
```bash
curl "http://localhost:3000/api/regions?city=Toronto&country=Canada"
```
- [ ] Returns correct regions
- [ ] Shows parameter dependency working
- [ ] Returns valid JSON

## Phase 4: Code Review

### Backend Implementation
- [ ] `internal/utils/options.go` exists
  - [ ] Contains InterpolateVariables function
  - [ ] Contains FetchRemoteOptions function
  - [ ] Contains FetchRemoteOptionsWithVariables function

- [ ] `internal/core/models/flow.go` modified
  - [ ] Input struct has OptionsURL field
  - [ ] YAML tags include options_url

- [ ] `internal/handlers/flows.go` modified
  - [ ] Has mergeRemoteOptions function
  - [ ] HandleFlowTrigger calls mergeRemoteOptions
  - [ ] HandleGetFlowInputs calls mergeRemoteOptions
  - [ ] Utils package imported

- [ ] `internal/handlers/types.go` modified
  - [ ] FlowInputReq has OptionsURL field
  - [ ] FlowInput has OptionsURL field
  - [ ] Conversion functions updated

### Frontend Implementation
- [ ] `site/src/lib/types.ts` modified
  - [ ] FlowInput interface has options_url
  - [ ] FlowInputReq interface has options_url

- [ ] `site/src/lib/components/flow-create/FlowInputs.svelte` modified
  - [ ] Select input shows options_url field
  - [ ] Help text visible

- [ ] `site/src/lib/components/shared/FlowInputFields.svelte` modified
  - [ ] Has fetchRemoteOptions function
  - [ ] Shows loading spinner
  - [ ] Fetches and merges options
  - [ ] Updates select dropdown

## Phase 5: Integration Testing (10 minutes)

### Build flowctl
```bash
go build
```
- [ ] Build completes without errors
- [ ] Executable created (flowctl.exe on Windows)
- [ ] No compilation warnings

### Start flowctl Server
```bash
./flowctl start
```
- [ ] Server starts successfully
- [ ] No error messages
- [ ] Server listening on expected port (usually 8080)
- [ ] Admin UI accessible

### Create Test Flow
- [ ] Access UI at http://localhost:8080
- [ ] Navigate to flow creation
- [ ] Create new flow with:
  - [ ] Name: "remote-options-test"
  - [ ] Input 1: country (select, static options)
  - [ ] Input 2: city (select, with options_url)
  - [ ] Action: simple echo script

### Configure Remote Options
- [ ] For city input:
  - [ ] Set Type to "select"
  - [ ] Add options_url: "http://localhost:3000/api/cities?country={{country}}"
  - [ ] Leave static options empty OR add defaults

- [ ] Flow saves successfully
- [ ] No validation errors

### Test Flow Inputs Endpoint
```bash
curl http://localhost:8080/api/flows/remote-options-test/inputs
```
- [ ] Returns flow inputs JSON
- [ ] Includes options_url field
- [ ] Options field populated with remote options

### Test Through Web UI
- [ ] Load flow in UI
- [ ] Country dropdown shows static options
- [ ] City dropdown shows loading spinner initially
- [ ] City dropdown populates with remote options
- [ ] Can select city value
- [ ] Dropdown closes after selection

### Test Flow Trigger
```bash
curl -X POST http://localhost:8080/api/flows/remote-options-test/trigger \
  -d "country=Canada&city=Toronto"
```
- [ ] Request accepted (200 status)
- [ ] Flow execution queued
- [ ] Returns execution ID
- [ ] No validation errors

### Check Execution
- [ ] View execution logs
- [ ] Script ran successfully
- [ ] Output shows selected values
- [ ] No errors in logs

## Phase 6: Error Cases (5 minutes)

### Test Missing Variables
- [ ] Trigger flow without country input
- [ ] Should get validation error
- [ ] Error message clear

### Test Invalid URL
- [ ] Edit flow with invalid options_url
- [ ] Trigger flow
- [ ] Should gracefully degrade to static options

### Test Server Unavailable
- [ ] Stop mock API server
- [ ] Try to load flow inputs
- [ ] Should fall back to static options
- [ ] No crash

### Test Invalid JSON Response
- [ ] Modify mock server to return invalid JSON
- [ ] Flow UI shows loading error gracefully
- [ ] Can still use static options

## Phase 7: Feature Testing (10 minutes)

### Static + Remote Options Mix
- [ ] Create input with both options and options_url
- [ ] Both sets appear in dropdown
- [ ] No duplicates
- [ ] All options work

### Complex Variable Interpolation
- [ ] URL with multiple variables: `?a={{var1}}&b={{var2}}&c={{var3}}`
- [ ] All variables interpolated correctly
- [ ] API receives correct parameters

### Cascading Selects
- [ ] Country → City → Region dependencies
- [ ] Each depends on previous selections
- [ ] Changes trigger updates
- [ ] All options fetched correctly

### Performance
- [ ] Flow loads in < 2 seconds
- [ ] With 100+ options: still fast
- [ ] No UI freezing
- [ ] Spinner shows while loading

## Phase 8: Documentation Check

- [ ] Verify all documentation files exist:
  - [ ] QUICK_START_TESTING.md
  - [ ] TESTING_GUIDE.md
  - [ ] ARCHITECTURE.md
  - [ ] REMOTE_OPTIONS_EXAMPLE.md
  - [ ] IMPLEMENTATION_SUMMARY.md
  - [ ] TESTING_RESOURCES.md

- [ ] Documentation is readable
- [ ] Examples are correct
- [ ] Instructions are clear

## Cleanup

- [ ] Stop flowctl server (Ctrl+C in terminal)
- [ ] Stop mock API server (Ctrl+C in terminal)
- [ ] Delete test flows created
- [ ] Clean up any test data

## Final Verification

### Results Summary
- [ ] Unit tests: **PASS** (15/15)
- [ ] Mock API: **WORKING** (all endpoints respond)
- [ ] API integration: **WORKING** (curl requests succeed)
- [ ] Code review: **COMPLETE** (all files modified correctly)
- [ ] Integration: **WORKING** (flow with remote options functions)
- [ ] Error handling: **WORKING** (graceful degradation)
- [ ] Documentation: **COMPLETE** (all guides written)

### Ready for Production?
- [ ] All tests pass
- [ ] No error messages
- [ ] Feature works as designed
- [ ] Backward compatible
- [ ] Well documented
- [ ] Error handling in place

## Sign-Off

Date Tested: _______________

Tested By: _______________

All Tests Passed: _____ YES _____ NO

Issues Found:
```
[List any issues here]
```

## Next Steps

- [ ] Merge feature to main branch
- [ ] Update release notes
- [ ] Notify team of feature availability
- [ ] Monitor for issues in production

---

**Testing Status: [  ] NOT STARTED  [  ] IN PROGRESS  [  ] COMPLETE**

Last Updated: _______________
