# Remote Options Example

This document demonstrates how to use the remote options URL feature for select inputs in flowctl flows.

## Overview

Select inputs can now fetch options from a remote HTTP API endpoint. The URL supports variable interpolation, allowing dynamic option lists based on flow input values.

## API Response Format

The remote API should return a JSON array of option objects:

```json
[
  {"name": "Option 1", "selected": true},
  {"name": "Option 2"},
  {"name": "Option 3"}
]
```

- `name` (required): The display name of the option
- `selected` (optional): Boolean indicating if the option is pre-selected

## URL Variable Interpolation

URLs support variable interpolation using the `{{variable_name}}` syntax. Variables refer to input values from the same flow.

### Example

```yaml
inputs:
  - name: environment
    type: select
    label: Environment
    required: true
    options_url: "https://api.example.com/services?env={{environment}}&region={{region}}"

  - name: region
    type: select
    label: Region
    required: true
    options:
      - us-east-1
      - us-west-2
      - eu-west-1
```

In this example:
- The `region` input has static options
- The `services` input fetches options from a remote API using the selected region as a parameter
- When the user selects a region, the services dropdown will be populated from the API using that region value

## Complete Flow Example

```yaml
metadata:
  id: service_deployer
  name: Deploy Service
  description: Deploy a service to the selected environment and region

inputs:
  - name: environment
    type: select
    label: Environment
    description: Target environment
    required: true
    options:
      - development
      - staging
      - production

  - name: region
    type: select
    label: Region
    description: AWS Region
    required: true
    options_url: "https://api.example.com/regions?env={{environment}}"

  - name: service
    type: select
    label: Service
    description: Service to deploy
    required: true
    options_url: "https://api.example.com/services?env={{environment}}&region={{region}}"

actions:
  - id: deploy
    name: Deploy
    executor: script
    with:
      script: |
        echo "Deploying to {{environment}} in {{region}}"
        echo "Service: {{service}}"
```

## Backend Processing

When a flow is triggered:

1. The backend receives the input values
2. For any select input with an `options_url`:
   - Variable placeholders are interpolated using the provided input values
   - A GET request is sent to the interpolated URL
   - The JSON response is parsed to extract option names
   - Remote options are merged with any static options
3. The selected value is validated against the merged options list
4. Flow execution proceeds if validation passes

## Error Handling

If fetching remote options fails:
- The backend logs a warning but continues processing
- Validation uses static options if available
- If no static options exist and remote fetch failed, validation will fail

## API Endpoint Implementation

Here's an example Node.js/Express endpoint that serves options:

```javascript
app.get('/api/services', (req, res) => {
  const { env, region } = req.query;
  
  // Validate inputs
  if (!env || !region) {
    return res.status(400).json({ error: 'env and region are required' });
  }

  // Fetch services based on environment and region
  const services = getServices(env, region);
  
  // Return array of option objects
  res.json(services.map(service => ({
    name: service.name,
    selected: service.isDefault
  })));
});
```

## Frontend Behavior

When displaying a select input with `options_url`:

1. The component detects the `options_url` field
2. While loading, a spinner is shown
3. Once options are loaded, they are merged with static options (if any)
4. The control is then enabled for user selection
5. If the URL requires variables that are filled in by the user, options are re-fetched when those values change

## Limitations and Notes

1. **No CORS**: Ensure your API endpoint allows requests from the flowctl domain
2. **Timeout**: Remote option fetching has a 10-second timeout
3. **No Caching**: Options are fetched fresh each time validation is needed
4. **Required Variables**: If a URL contains a variable placeholder but the variable is not provided, an error will be returned during flow trigger
5. **Static Options Fallback**: If remote URL fails but static options exist, static options are used for validation

## Security Considerations

- URLs are interpolated only with input values from the same flow
- Only GET requests are made to fetch options
- Response timeout is enforced (10 seconds)
- HTTP error responses are logged but don't break the flow
