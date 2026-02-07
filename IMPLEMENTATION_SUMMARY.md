# Remote URL Options Implementation Summary

This document summarizes the implementation of support for fetching options from remote URLs in flowctl flow inputs.

## Overview

The implementation allows select inputs in flowctl flows to dynamically fetch their available options from a remote HTTP API endpoint. URLs support variable interpolation, enabling options to depend on other input values in the flow.

## Changes Made

### 1. Backend Changes

#### Core Model Changes
**File: `internal/core/models/flow.go`**

- Added `OptionsURL` field to the `Input` struct:
  ```go
  OptionsURL  string    `yaml:"options_url" huml:"options_url" json:"options_url"`
  ```
  This field stores the remote URL from which options should be fetched.

#### API Types Changes
**File: `internal/handlers/types.go`**

- Added `OptionsURL` field to `FlowInputReq` struct (for API requests)
- Added `OptionsURL` field to `FlowInput` struct (for API responses)
- Updated `coreFlowInputToInput()` function to include OptionsURL in conversions
- Updated `convertFlowInputsReqToInputs()` and `convertFlowInputsToInputsReq()` functions to handle OptionsURL

#### Remote Options Utility
**File: `internal/utils/options.go` (NEW FILE)**

Created a new utility package with three functions:

1. **`InterpolateVariables(url string, variables map[string]interface{}) (string, error)`**
   - Replaces variable placeholders in URLs using `{{variable_name}}` syntax
   - Used to substitute input values into the remote API URL

2. **`FetchRemoteOptions(url string) ([]string, error)`**
   - Fetches options from a remote HTTP endpoint
   - Expects a JSON array response with objects containing a `name` field
   - Has a 10-second timeout for the HTTP request

3. **`FetchRemoteOptionsWithVariables(url string, variables map[string]interface{}) ([]string, error)`**
   - Combines variable interpolation with remote fetching
   - First interpolates variables, then fetches from the resulting URL

#### Handler Changes
**File: `internal/handlers/flows.go`**

- Added import for `internal/utils` package and `slices` package
- Added `mergeRemoteOptions(flow *models.Flow, variables map[string]interface{}) error` function:
  - Iterates through flow inputs to find select inputs with OptionsURL
  - Fetches remote options using variable interpolation
  - Merges remote options with static options (if both exist)
  - Logs warnings if fetching fails but continues processing
  
- Updated `HandleFlowTrigger()`:
  - Calls `mergeRemoteOptions()` before input validation
  - Ensures remote options are available for validation of select inputs

- Updated `HandleGetFlowInputs()`:
  - Calls `mergeRemoteOptions()` to fetch and merge remote options
  - Allows fetching flow inputs with pre-populated remote options

### 2. Frontend Changes

#### TypeScript Types
**File: `site/src/lib/types.ts`**

- Added `options_url?: string;` field to `FlowInput` interface
- Added `options_url?: string;` field to `FlowInputReq` interface

#### Flow Input Editor Component
**File: `site/src/lib/components/flow-create/FlowInputs.svelte`**

- Updated the select input type section to include two separate input fields:
  - "Options (one per line)" - for static options
  - "Remote Options URL" - for the API endpoint URL
- Added helpful text explaining variable interpolation syntax
- Allows users to specify both static and remote options

#### Flow Input Display Component
**File: `site/src/lib/components/shared/FlowInputFields.svelte`**

- Added comprehensive script section with:
  - State tracking for loading status per input
  - Merged options tracking
  - `interpolateVariables()` function for URL variable substitution
  - `fetchRemoteOptions()` async function to fetch from remote URLs
  
- Updated select input rendering:
  - Shows loading spinner while fetching remote options
  - Uses merged options (remote + static) when displaying options
  - Supports both form-based and reactive inputs
  - Gracefully handles fetch failures

## Implementation Details

### API Response Format

The remote API endpoint should return a JSON array with the following structure:

```json
[
  {"name": "Option 1", "selected": true},
  {"name": "Option 2"},
  {"name": "Option 3"}
]
```

Fields:
- `name` (required): The option value/display text
- `selected` (optional): Boolean indicating if the option is pre-selected

### Variable Interpolation

URLs support variable interpolation using `{{variable_name}}` syntax:

```yaml
options_url: "https://api.example.com/options?env={{environment}}&type={{service_type}}"
```

Variables reference input values from the same flow. When options are fetched, the current values of other inputs are used to interpolate the URL.

### Processing Flow

1. **On Flow Creation/Update**: User specifies `options_url` in the flow definition
2. **On Get Flow Inputs**: Backend fetches remote options and merges with static ones
3. **On Flow Trigger**: 
   - Backend receives input values from the request
   - For each select input with OptionsURL:
     - Variable interpolation happens
     - Remote options are fetched
     - Merged with static options
   - Validation checks selected value against merged options
4. **On Frontend Display**:
   - Component detects `options_url` field
   - Shows loading spinner
   - Fetches options from the URL
   - Renders dropdown with all available options

### Error Handling

- **Backend**: Warnings are logged if remote fetch fails; validation falls back to static options
- **Frontend**: Console errors are logged; UI gracefully degrades (shows loading state if cache exists, or empty dropdown)
- **Network Issues**: 10-second timeout prevents hanging requests
- **Invalid JSON**: Response parsing errors are caught and logged

## Usage Examples

### Simple Remote Options

```yaml
inputs:
  - name: service
    type: select
    label: Service
    options_url: "https://api.example.com/services"
```

### With Variable Interpolation

```yaml
inputs:
  - name: environment
    type: select
    label: Environment
    options:
      - development
      - staging
      - production

  - name: service
    type: select
    label: Service
    options_url: "https://api.example.com/services?env={{environment}}"
```

### Mixed Static and Remote

```yaml
inputs:
  - name: service
    type: select
    label: Service
    options:
      - static-option-1
      - static-option-2
    options_url: "https://api.example.com/services"
    # Both static and remote options will be available
```

## Benefits

1. **Dynamic Options**: Options can change based on API state
2. **Variable Interpolation**: Options can depend on other input values
3. **Scalability**: No need to hardcode large option lists
4. **Flexibility**: Supports any HTTP API with appropriate response format
5. **Robustness**: Graceful fallback to static options if remote fetch fails

## Testing Recommendations

1. Test with URLs requiring variable interpolation
2. Test with both static and remote options
3. Test network failure scenarios
4. Test with large option sets
5. Test with special characters in option names
6. Test with concurrent flow triggers

## Files Modified

1. `internal/core/models/flow.go` - Added OptionsURL field
2. `internal/handlers/types.go` - Updated request/response types
3. `internal/handlers/flows.go` - Added remote options fetching logic
4. `site/src/lib/types.ts` - Updated TypeScript interfaces
5. `site/src/lib/components/flow-create/FlowInputs.svelte` - Added options_url input field
6. `site/src/lib/components/shared/FlowInputFields.svelte` - Added remote options fetching

## Files Added

1. `internal/utils/options.go` - Remote options utility functions
2. `REMOTE_OPTIONS_EXAMPLE.md` - Comprehensive usage documentation
3. `example-remote-options-flow.yaml` - Example flow file

## Backward Compatibility

All changes are backward compatible:
- Existing flows without `options_url` continue to work
- Existing static options are fully supported
- Optional field in API responses (can be omitted)
