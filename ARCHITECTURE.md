# Architecture Diagram - Remote Options Feature

## Data Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                         User Interface                          │
│  (site/src/lib/components/shared/FlowInputFields.svelte)        │
│                                                                  │
│  1. Detects options_url field                                  │
│  2. Shows loading spinner                                       │
│  3. Makes HTTP request to remote API                           │
│  4. Parses JSON response                                        │
│  5. Merges with static options                                 │
│  6. Renders populated dropdown                                  │
└─────────────────────────────────────────────────────────────────┘
                              ↑
                              │
                    Variable Interpolation
                 replace {{variable}} with values
                              │
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                    Remote API Endpoint                          │
│             (e.g., https://api.example.com/options)             │
│                                                                  │
│  Input:  GET /options?country=Canada&city=Toronto               │
│  Output: [                                                       │
│           {"name": "option1", "selected": false},               │
│           {"name": "option2", "selected": true}                 │
│          ]                                                       │
└─────────────────────────────────────────────────────────────────┘
                              ↑
                              │
                      Backend Processing
┌─────────────────────────────────────────────────────────────────┐
│                    Flow Trigger Handler                         │
│        (internal/handlers/flows.go - HandleFlowTrigger)         │
│                                                                  │
│  1. Receives input values from request                          │
│  2. Calls mergeRemoteOptions()                                  │
│  3. For each select input with OptionsURL:                      │
│     a. Interpolate variables in URL                             │
│     b. Fetch remote options                                     │
│     c. Merge with static options                                │
│  4. Validates selected value against merged options             │
│  5. Executes flow if validation passes                          │
└─────────────────────────────────────────────────────────────────┘
                              ↑
                              │
                   Utility Functions
┌─────────────────────────────────────────────────────────────────┐
│                  (internal/utils/options.go)                    │
│                                                                  │
│  ┌────────────────────────────────────────────────────────┐    │
│  │ InterpolateVariables(url, vars) -> string              │    │
│  │   Replaces {{variable}} with actual values             │    │
│  └────────────────────────────────────────────────────────┘    │
│                           ↓                                      │
│  ┌────────────────────────────────────────────────────────┐    │
│  │ FetchRemoteOptions(url) -> []string                    │    │
│  │   Makes HTTP GET request (timeout: 10s)                │    │
│  │   Parses JSON response                                 │    │
│  │   Extracts option names                                │    │
│  └────────────────────────────────────────────────────────┘    │
│                           ↓                                      │
│  ┌────────────────────────────────────────────────────────┐    │
│  │ FetchRemoteOptionsWithVariables(url, vars) -> []string│    │
│  │   Combines interpolation + fetching                    │    │
│  └────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
                              ↑
                              │
                      Flow Definition
┌─────────────────────────────────────────────────────────────────┐
│                   Flow YAML Structure                           │
│                                                                  │
│  inputs:                                                        │
│    - name: service                                             │
│      type: select                                              │
│      options_url: "https://api.example.com/options              │
│                    ?env={{environment}}&type={{service_type}}" │
│      options:                                                   │
│        - fallback_option_1                                      │
│        - fallback_option_2                                      │
│                                                                  │
│  Note: options_url + options can be combined                   │
│        Remote options are merged with static options            │
└─────────────────────────────────────────────────────────────────┘
```

## Request/Response Flow

```
┌─────────────┐
│   Browser   │
│     UI      │
└──────┬──────┘
       │
       │ 1. User loads flow
       │
       ↓
┌─────────────────────────────────────────┐
│  GET /api/flows/{id}/inputs             │
│  (HandleGetFlowInputs)                  │
└──────┬──────────────────────────────────┘
       │
       │ 2. Backend fetches remote options
       │    and merges with static options
       │
       ↓
┌─────────────────────────────────────────┐
│  Backend: mergeRemoteOptions()           │
│                                         │
│  For each select input with options_url:│
│    - interpolateVariables(url)          │
│    - fetchRemoteOptions(url)            │
│    - merge with static options          │
└──────┬──────────────────────────────────┘
       │
       │ 3. Return merged options
       │
       ↓
┌──────────────────────────────────────────────┐
│  Response: FlowInputsResp                    │
│  {                                           │
│    "inputs": [                               │
│      {                                       │
│        "name": "service",                    │
│        "type": "select",                     │
│        "options": [                          │
│          "remote-option-1",                  │
│          "remote-option-2",                  │
│          "static-option"                     │
│        ],                                    │
│        "options_url": "https://..."          │
│      }                                       │
│    ]                                        │
│  }                                          │
└──────┬───────────────────────────────────────┘
       │
       │ 4. Frontend displays options
       │
       ↓
┌─────────────────────────────┐
│   Select Dropdown           │
│   [Select an option]        │
│   - remote-option-1         │
│   - remote-option-2         │
│   - static-option           │
└──────┬──────────────────────┘
       │
       │ 5. User selects option
       │
       ↓
┌──────────────────────────────────────┐
│  POST /api/flows/{id}/trigger        │
│  {                                   │
│    "service": "remote-option-1",     │
│    "other_input": "value"            │
│  }                                   │
└──────┬───────────────────────────────┘
       │
       │ 6. Backend validates
       │
       ↓
┌───────────────────────────────────────┐
│  Backend: HandleFlowTrigger()          │
│  - mergeRemoteOptions(variables)      │
│  - validate selected value in options │
│  - queue flow for execution           │
└───────────────────────────────────────┘
```

## Component Interaction

```
Flow Input Definition (YAML)
    │
    ├─ name: "service"
    ├─ type: "select"
    ├─ options: ["opt1", "opt2"]              ← Static options
    └─ options_url: "https://api.../options"  ← Remote options URL
                 │
                 ↓
        Backend: Input struct
        (internal/core/models/flow.go)
                 │
                 ├─ Gets attached to Flow
                 │
                 ↓
        Frontend: FlowInput type
        (site/src/lib/types.ts)
                 │
                 ├─ Displayed in UI
                 │
                 ↓
        FlowInputFields Component
        (site/src/lib/components/shared/)
                 │
                 ├─ 1. Detects options_url
                 ├─ 2. interpolateVariables()
                 ├─ 3. fetchRemoteOptions()
                 ├─ 4. Merge with options
                 └─ 5. Render dropdown
                 
        User selects option
                 │
                 ↓
        HandleFlowTrigger()
                 │
                 ├─ 1. mergeRemoteOptions()
                 ├─ 2. validate the selection
                 ├─ 3. queue execution
                 └─ 4. success response
```

## Error Handling Flow

```
┌─────────────────────────────────┐
│  Start: Process Remote Options  │
└────────┬────────────────────────┘
         │
         ↓
    ┌─────────────┐
    │   URL? NO   │
    ├─────────────┤
    │Return empty │
    └─────────────┘
         │
         ├─ YES
         ↓
    ┌──────────────────────────┐
    │ Interpolate Variables    │
    └────────┬─────────────────┘
             │
             └─ Error? ──────────────┐
                                     │
               YES                   ↓
         Log warning            ┌─────────────┐
         Use static options     │ Return err  │
             │                  └─────────────┘
             NO
             ↓
    ┌──────────────────────────┐
    │ Fetch Remote Options     │
    │ (timeout: 10s)           │
    └────────┬─────────────────┘
             │
             └─ Error? ──────────────┐
                                     │
               YES                   ↓
         Log warning            ┌─────────────────┐
         Use static options     │ Continue process│
         (fallback)             │ May fail later  │
             │                  └─────────────────┘
             NO
             ↓
    ┌──────────────────────────┐
    │ Parse JSON Response      │
    └────────┬─────────────────┘
             │
             └─ Error? ──────────────┐
                                     │
               YES                   ↓
         Log warning            ┌─────────────┐
         Skip remote options    │ Continue    │
             │                  └─────────────┘
             NO
             ↓
    ┌──────────────────────────┐
    │ Merge Options            │
    │ (remote + static)        │
    └────────┬─────────────────┘
             │
             ↓
    ┌──────────────────────────┐
    │ Validate Selection       │
    └────────┬─────────────────┘
             │
             ├─ Valid? ──→ Return Success
             │
             └─ Invalid? ─→ Return Validation Error
```

## Testing Strategy

```
┌────────────────────────────────────────────┐
│           Unit Tests                       │
│   (internal/utils/options_test.go)         │
│                                            │
│  ├─ Variable Interpolation (7 tests)      │
│  │  ├─ No variables                       │
│  │  ├─ Single variable                    │
│  │  ├─ Multiple variables                 │
│  │  ├─ Missing variable error             │
│  │  ├─ Numeric variables                  │
│  │  ├─ Boolean variables                  │
│  │  └─ Variables with spaces              │
│  │                                         │
│  ├─ Remote Fetching (5 tests)             │
│  │  ├─ Successful fetch                   │
│  │  ├─ Empty options                      │
│  │  ├─ Skip empty names                   │
│  │  ├─ HTTP error handling                │
│  │  └─ Invalid JSON handling              │
│  │                                         │
│  └─ Integration (3 tests)                 │
│     ├─ Interpolation + fetch              │
│     ├─ Multiple variables                 │
│     └─ Missing variable error             │
└────────────────────────────────────────────┘
                    │
                    ↓
┌────────────────────────────────────────────┐
│        Mock API Server                     │
│    (test-api-server.js)                    │
│                                            │
│  ├─ GET /api/environments                 │
│  ├─ GET /api/cities?country=               │
│  ├─ GET /api/regions?city=&country=        │
│  ├─ GET /api/services?env=                 │
│  └─ GET /health                           │
└────────────────────────────────────────────┘
                    │
                    ↓
┌────────────────────────────────────────────┐
│     Manual Integration Tests               │
│                                            │
│  ├─ API endpoint testing (curl)           │
│  ├─ Backend flow validation               │
│  ├─ Frontend option loading               │
│  ├─ Variable interpolation e2e            │
│  └─ Error handling scenarios              │
└────────────────────────────────────────────┘
```

---

This architecture provides:
- **Separation of concerns**: Utility functions, handlers, and UI components
- **Error handling**: Graceful degradation when remote fetch fails
- **Flexibility**: Support for both static and remote options
- **Performance**: Efficient HTTP requests with timeouts
- **Testability**: Comprehensive unit tests with mocked HTTP responses
