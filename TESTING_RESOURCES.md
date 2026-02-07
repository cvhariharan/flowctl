# Testing Resources Summary

Complete testing setup for the Remote Options feature is ready. Here's what you have:

## 📋 Quick Navigation

| Resource | Purpose | Location |
|----------|---------|----------|
| **Quick Start** | Get testing in 5 minutes | [QUICK_START_TESTING.md](QUICK_START_TESTING.md) |
| **Full Testing Guide** | Comprehensive test scenarios | [TESTING_GUIDE.md](TESTING_GUIDE.md) |
| **Architecture** | System design and data flow | [ARCHITECTURE.md](ARCHITECTURE.md) |
| **Usage Examples** | API format and flow configurations | [REMOTE_OPTIONS_EXAMPLE.md](REMOTE_OPTIONS_EXAMPLE.md) |
| **Implementation Details** | Technical implementation overview | [IMPLEMENTATION_SUMMARY.md](IMPLEMENTATION_SUMMARY.md) |

## 🧪 Testing Files

### Unit Tests
```
internal/utils/options_test.go
├─ TestInterpolateVariables (7 tests)
├─ TestFetchRemoteOptions (5 tests)
└─ TestFetchRemoteOptionsWithVariables (3 tests)
```
**Run:** `go test -v ./internal/utils`

### Mock API Server
```
test-api-server.js
├─ /api/environments - Static list of environments
├─ /api/cities?country=<name> - Cities by country
├─ /api/regions?city=<name>&country=<name> - Regions by location
├─ /api/services?env=<env> - Services by environment
└─ /health - Health check endpoint
```
**Run:** `node test-api-server.js`

### Test Scripts
```
test-options.sh      (Linux/Mac)  - Run all Go unit tests
test-options.bat     (Windows)    - Run all Go unit tests
test-api-calls.bat   (Windows)    - Test API endpoints with curl
```

### Example Flow
```
example-remote-options-flow.yaml
├─ Country selection (static options)
├─ City selection (depends on country via remote URL)
├─ Service selection (static options)
└─ Region selection (depends on country & city via remote URL)
```

## 📊 Test Coverage

### Test Categories
- **Variable Interpolation**: 7 tests covering all scenarios
- **Remote Fetching**: 5 tests for different response types
- **Integration**: 3 tests combining interpolation and fetching
- **Error Handling**: Tests for network errors, invalid JSON, missing variables
- **Total Unit Tests**: 15+ comprehensive test cases

### Test Scenarios Covered
✓ No variables in URL
✓ Single variable interpolation
✓ Multiple variable interpolation
✓ Missing variable error handling
✓ Numeric and boolean variable types
✓ Variables with special characters/spaces
✓ Successful remote option fetching
✓ Empty option arrays
✓ HTTP error responses (4xx, 5xx)
✓ Invalid JSON parsing
✓ Option name filtering

## 🚀 How to Test

### Option 1: Quick Unit Tests (2 minutes)
```bash
cd e:/gsoc/flowctl
go test -v ./internal/utils
```
All 15 tests should pass.

### Option 2: Full API Testing (10 minutes)
```bash
# Terminal 1: Start mock server
node test-api-server.js

# Terminal 2: Run API tests
test-api-calls.bat

# Terminal 3: Run unit tests
go test -v ./internal/utils
```

### Option 3: Complete Integration Test (20 minutes)
```bash
# Terminal 1: Mock API server
node test-api-server.js

# Terminal 2: Build and start flowctl
go build
./flowctl start

# Terminal 3: Unit tests + manual smoke test via UI/API
go test -v ./internal/utils
curl http://localhost:8080/api/flows/test_remote_options/inputs
```

## 📝 Documentation Structure

### For Quick Start
→ Read [QUICK_START_TESTING.md](QUICK_START_TESTING.md) (5-10 min read)

### For Detailed Testing
→ Read [TESTING_GUIDE.md](TESTING_GUIDE.md) (15-20 min read)

### For Understanding Design
→ Read [ARCHITECTURE.md](ARCHITECTURE.md) (10 min read)

### For API Format
→ Read [REMOTE_OPTIONS_EXAMPLE.md](REMOTE_OPTIONS_EXAMPLE.md) (10 min read)

### For Technical Details
→ Read [IMPLEMENTATION_SUMMARY.md](IMPLEMENTATION_SUMMARY.md) (15 min read)

## ✅ Expected Results

### Unit Tests Should Show:
```
PASS TestInterpolateVariables (0.00s)
PASS TestInterpolateVariables/no_variables (0.00s)
PASS TestInterpolateVariables/single_variable (0.00s)
... [12 more tests]
PASS TestFetchRemoteOptionsWithVariables/...
ok  github.com/cvhariharan/flowctl/internal/utils  0.150s
```

### Mock API Should Respond:
```json
[
  {"name": "option1", "selected": false},
  {"name": "option2", "selected": true}
]
```

### Flow Should Support:
- Creating select inputs with `options_url`
- Remote options merged with static options
- Variable interpolation in URLs
- Validation against merged options

## 🔍 What You Can Test

### Backend Functionality
- [ ] Variable interpolation in URLs
- [ ] Remote options fetching with HTTP
- [ ] Error handling (network errors, invalid JSON)
- [ ] Options merging (remote + static)
- [ ] Flow validation with remote options
- [ ] Input API responses include options_url

### Frontend Functionality
- [ ] options_url field in flow editor
- [ ] Loading spinner while fetching
- [ ] Options displayed in dropdown
- [ ] Variable interpolation in frontend
- [ ] Mixed static/remote options
- [ ] Error handling and fallbacks

### Integration
- [ ] End-to-end flow with remote options
- [ ] Variable-dependent cascading selects
- [ ] Flow execution with remote-sourced options
- [ ] API responses properly formatted

## 📦 Created Files Summary

### Code
- `internal/utils/options.go` - Core utility functions
- `internal/utils/options_test.go` - Comprehensive unit tests

### Test Scripts
- `test-api-server.js` - Mock API server
- `test-options.sh` - Test runner (Linux/Mac)
- `test-options.bat` - Test runner (Windows)
- `test-api-calls.bat` - API testing script

### Configuration
- `example-remote-options-flow.yaml` - Sample flow

### Documentation
- `QUICK_START_TESTING.md` - Quick testing guide
- `TESTING_GUIDE.md` - Comprehensive testing guide
- `ARCHITECTURE.md` - System architecture diagrams
- `REMOTE_OPTIONS_EXAMPLE.md` - API and usage examples
- `IMPLEMENTATION_SUMMARY.md` - Implementation details
- `TESTING_RESOURCES.md` - This file

## 🎯 Next Steps

1. **Read** [QUICK_START_TESTING.md](QUICK_START_TESTING.md)
2. **Run** `go test -v ./internal/utils`
3. **Start** `node test-api-server.js`
4. **Test** API endpoints with curl
5. **Build and test** full flow integration
6. **Verify** all scenarios work as expected

## 💡 Tips

- Start with unit tests (fastest feedback)
- Use mock API server for isolated testing
- Test both successful and error paths
- Check browser console for frontend debugging
- Use curl for quick API validation
- Review logs for backend processing details

## 🐛 Debugging

If tests fail:
1. Check test output for specific assertion
2. Review [TESTING_GUIDE.md](TESTING_GUIDE.md) troubleshooting section
3. Verify mock API is running (port 3000)
4. Check network tabs in browser DevTools
5. Review backend logs for errors

---

**Ready to test?** Start with [QUICK_START_TESTING.md](QUICK_START_TESTING.md) 🚀
