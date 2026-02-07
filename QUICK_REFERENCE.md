# 📘 Complete Reference Guide - Remote Options Testing

## 🎯 Where to Start (Pick One)

### ⚡ **Fast Path (5 minutes)**
Just want to verify it works quickly?
```bash
go test -v ./internal/utils
# All tests should PASS ✓
```
→ Continue to [QUICK_START_TESTING.md](QUICK_START_TESTING.md)

### 🏃 **Standard Path (30 minutes)**
Want thorough testing with all steps documented?
→ Read [START_HERE_TESTING.md](START_HERE_TESTING.md)
→ Follow [TESTING_CHECKLIST.md](TESTING_CHECKLIST.md)

### 🗺️ **Explorer Path (1-2 hours)**
Want to understand everything?
→ Read [ARCHITECTURE.md](ARCHITECTURE.md) first
→ Then [IMPLEMENTATION_SUMMARY.md](IMPLEMENTATION_SUMMARY.md)
→ Then [TESTING_GUIDE.md](TESTING_GUIDE.md)

---

## 📚 Documentation Map

```
START_HERE_TESTING.md (3 min) ⭐ ENTRY POINT
├─ 30-second quick test
├─ Choose your path
├─ Quick reference
└─ Links to detailed docs

QUICK_START_TESTING.md (5 min)
├─ Prerequisites
├─ Step-by-step testing
├─ Expected outputs
└─ Troubleshooting

TESTING_CHECKLIST.md (20 min)
├─ 8 testing phases
├─ 100+ verification items
├─ Cleanup procedures
└─ Sign-off form

TESTING_GUIDE.md (20 min)
├─ Manual test scenarios
├─ Backend functionality tests
├─ Frontend functionality tests
├─ Performance testing
└─ Troubleshooting

ARCHITECTURE.md (10 min)
├─ Data flow diagram
├─ Request/response flow
├─ Component interaction
├─ Error handling flow
└─ Testing strategy

REMOTE_OPTIONS_EXAMPLE.md (10 min)
├─ API response format
├─ URL variable interpolation
├─ Complete flow examples
├─ Security notes
└─ Limitations

IMPLEMENTATION_SUMMARY.md (15 min)
├─ Backend changes overview
├─ Frontend changes overview
├─ Implementation details
├─ Error handling
├─ Testing recommendations

TESTING_RESOURCES.md (5 min)
├─ Resource index
├─ Quick navigation table
├─ Test coverage overview
└─ Next steps

FILE_SUMMARY.md (5 min)
├─ Changes at a glance
├─ File descriptions
├─ Statistics
└─ File relationships

QUICK_REFERENCE.md (THIS FILE) (2 min)
├─ Navigation paths
├─ Document map
├─ Command reference
└─ Troubleshooting
```

---

## 💻 Command Reference

### Unit Tests
```bash
# Run all tests
go test -v ./internal/utils

# Run specific test category
go test -v ./internal/utils -run TestInterpolateVariables
go test -v ./internal/utils -run TestFetchRemoteOptions
go test -v ./internal/utils -run TestFetchRemoteOptionsWithVariables

# Run with timeout
go test -v ./internal/utils -timeout 30s
```

### Mock API Server
```bash
# Start server
node test-api-server.js

# Start on different port
PORT=4000 node test-api-server.js

# Check health
curl http://localhost:3000/health
```

### Test API Endpoints
```bash
# Get environments
curl http://localhost:3000/api/environments

# Get cities for a country
curl "http://localhost:3000/api/cities?country=Canada"

# Get regions for city & country
curl "http://localhost:3000/api/regions?city=Toronto&country=Canada"

# Get services for environment
curl "http://localhost:3000/api/services?env=production"
```

### Build & Run flowctl
```bash
# Build
go build

# Start server
./flowctl start

# With debug logging
FLOWCTL_LOG_LEVEL=debug ./flowctl start

# Specific port
./flowctl start --port 9000
```

### Flow API Tests
```bash
# Get flow inputs
curl http://localhost:8080/api/flows/{flow-id}/inputs

# Trigger flow
curl -X POST http://localhost:8080/api/flows/{flow-id}/trigger \
  -d "input1=value1&input2=value2"

# Get execution status
curl http://localhost:8080/api/executions/{execution-id}
```

---

## 🧪 Quick Test Scenarios

### Test 1: Variable Interpolation
```bash
# URL with one variable
curl "http://localhost:3000/api/cities?country=Canada"

# URL with multiple variables
curl "http://localhost:3000/api/regions?city=Toronto&country=Canada"
```
Expected: Returns JSON array with filtered options

### Test 2: Unit Test Verification
```bash
go test -v ./internal/utils
```
Expected: All 15 tests PASS ✓

### Test 3: End-to-End Flow Test
```bash
# 1. Get flow inputs
curl http://localhost:8080/api/flows/test-flow/inputs

# 2. Trigger flow with values
curl -X POST http://localhost:8080/api/flows/test-flow/trigger \
  -d "country=Canada&city=Toronto"

# 3. Check result
# Should see execution ID in response
```

---

## 🎯 Testing Goals

✅ Verify unit tests pass
✅ Verify mock API responds correctly
✅ Verify variable interpolation works
✅ Verify remote options are fetched
✅ Verify UI loads options dynamically
✅ Verify flow validation uses options
✅ Verify error handling works
✅ Verify documentation is complete

---

## 📊 Expected Results

### Unit Tests
```
15 tests should PASS
- 7 interpolation tests
- 5 fetching tests  
- 3 integration tests
Time: < 1 second
```

### Mock API
```
All 4 endpoints should respond
- /api/environments → returns array
- /api/cities → returns filtered options
- /api/regions → returns filtered options
- /api/services → returns filtered options
Status: 200 OK
```

### Integration
```
Flow should:
- Accept options_url field
- Fetch remote options
- Merge with static options
- Validate selections
- Execute successfully
```

---

## 🔧 Troubleshooting Quick Fixes

| Problem | Solution |
|---------|----------|
| `go: command not found` | Install Go: https://golang.org/dl/ |
| Tests fail | Check [TESTING_GUIDE.md](TESTING_GUIDE.md) section "Troubleshooting" |
| API won't start | Port 3000 in use? Try: `PORT=4000 node test-api-server.js` |
| Options not showing | Browser console errors? Check Network tab in DevTools |
| Variable not interpolating | Check URL syntax: `{{variable_name}}` (double braces) |
| Remote fetch fails | Is mock API running? Check with `curl http://localhost:3000/health` |

---

## 📋 Phase Checklist

- [ ] **Setup** - All tools installed (Go, Node.js, curl)
- [ ] **Unit Tests** - Run and verify all 15 tests pass
- [ ] **Mock API** - Start server and verify responses
- [ ] **API Testing** - Test all 4 endpoints
- [ ] **Code Review** - Verify all files modified correctly
- [ ] **Integration** - Build and test with flowctl
- [ ] **Frontend** - Test UI with flows
- [ ] **Error Cases** - Verify graceful degradation
- [ ] **Documentation** - Read and verify
- [ ] **Final** - Sign off on testing complete

---

## 🎓 Learning Recommendations

**If you have 5 minutes:**
→ Run unit tests: `go test -v ./internal/utils`

**If you have 10 minutes:**
→ Read: [START_HERE_TESTING.md](START_HERE_TESTING.md)
→ Run: Tests + Mock API

**If you have 30 minutes:**
→ Follow: [TESTING_CHECKLIST.md](TESTING_CHECKLIST.md)

**If you have 1 hour:**
→ Read all documentation
→ Complete full testing [TESTING_CHECKLIST.md](TESTING_CHECKLIST.md)

**If you have 2 hours:**
→ Read: [ARCHITECTURE.md](ARCHITECTURE.md)
→ Read: Implementation details
→ Run: All manual tests
→ Create custom test scenarios

---

## 📞 Help Resources

| Need | Document | Time |
|------|----------|------|
| Quick start | [START_HERE_TESTING.md](START_HERE_TESTING.md) | 3 min |
| How to test | [QUICK_START_TESTING.md](QUICK_START_TESTING.md) | 5 min |
| Systematic verify | [TESTING_CHECKLIST.md](TESTING_CHECKLIST.md) | 20 min |
| All scenarios | [TESTING_GUIDE.md](TESTING_GUIDE.md) | 20 min |
| How it works | [ARCHITECTURE.md](ARCHITECTURE.md) | 10 min |
| How to use it | [REMOTE_OPTIONS_EXAMPLE.md](REMOTE_OPTIONS_EXAMPLE.md) | 10 min |
| Technical deep dive | [IMPLEMENTATION_SUMMARY.md](IMPLEMENTATION_SUMMARY.md) | 15 min |
| What changed | [FILE_SUMMARY.md](FILE_SUMMARY.md) | 5 min |

---

## ✅ Final Checklist

Before considering testing complete:

- [ ] Read START_HERE_TESTING.md
- [ ] Run: `go test -v ./internal/utils`
- [ ] Start mock API: `node test-api-server.js`
- [ ] Test API endpoints with curl
- [ ] Build flowctl: `go build`
- [ ] Create test flow with remote options
- [ ] Verify UI loads options
- [ ] Verify flow can be triggered
- [ ] Read at least one detailed guide
- [ ] All expected results match actual results

---

## 🚀 Next Steps

**Right Now:**
1. Open [START_HERE_TESTING.md](START_HERE_TESTING.md)
2. Pick your testing path
3. Follow the guide for your path

**When Tests Pass:**
1. Read [ARCHITECTURE.md](ARCHITECTURE.md) to understand design
2. Read [REMOTE_OPTIONS_EXAMPLE.md](REMOTE_OPTIONS_EXAMPLE.md) to learn usage
3. Consider deploying to production

---

## 📌 Important Files to Know

**Entry Points:**
- `START_HERE_TESTING.md` ← Read this first!

**Core Testing:**
- `QUICK_START_TESTING.md` - Fast testing guide
- `TESTING_CHECKLIST.md` - Systematic verification

**Understanding:**
- `ARCHITECTURE.md` - How it works
- `IMPLEMENTATION_SUMMARY.md` - What changed
- `REMOTE_OPTIONS_EXAMPLE.md` - How to use it

**Reference:**
- `FILE_SUMMARY.md` - Complete change log
- `TESTING_RESOURCES.md` - Resource index
- `TESTING_GUIDE.md` - Detailed scenarios

---

**Status: ✅ Ready for Testing**

**Start: [START_HERE_TESTING.md](START_HERE_TESTING.md)**

Good luck! 🎉
