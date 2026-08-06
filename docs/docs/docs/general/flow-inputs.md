---
title: Flow Inputs
description: Define, validate, and populate input parameters for flowctl workflows
---

Inputs define parameters that users provide when triggering a flow. Flowctl supports multiple input types with validation.

## Input Types

=== "String"

    ```yaml
    inputs:
      - name: namespace
        type: string
        label: Namespace
        description: Target namespace
        required: true
        default: "default"
        validation: len(namespace) > 3
    ```

=== "Number"

    ```yaml
    inputs:
      - name: count
        type: number
        label: Retry Count
        description: Number of retries
        required: true
        validation: count > 0 && count < 10
    ```

=== "Select"

    ```yaml
    inputs:
      - name: environment
        type: select
        label: Environment
        description: Deployment environment
        options:
          - development
          - staging
          - production
        required: true
    ```

    Select inputs also support fetching options from a remote endpoint. See [Remote Options](#remote-options).

=== "Checkbox"

    ```yaml
    inputs:
      - name: enable_debug
        type: checkbox
        label: Enable Debug Mode
        description: Enable verbose logging
        default: false
    ```

=== "Password"

    ```yaml
    inputs:
      - name: api_token
        type: password
        label: API Token
        description: Authentication token
        required: true
    ```

=== "File"

    ```yaml
    inputs:
      - name: config_file
        type: file
        label: Configuration File
        description: Upload a configuration file
        required: true
        max_file_size: 10485760 # Optional: 10MB limit (default: 100MB)
    ```

## File Inputs

File inputs allow users to upload files when triggering a flow. The uploaded file is made available to actions via the artifacts system.

### Using File Inputs in Actions

Reference file inputs using the standard `{{ inputs.file_name }}` syntax:

```yaml
inputs:
  - name: data_file
    type: file
    label: Data File
    required: true

actions:
  - id: process_file
    name: Process Uploaded File
    executor: docker
    variables:
      - file_path: "{{ inputs.data_file }}"
    with:
      image: docker.io/alpine
      script: |
        echo "Processing: $file_path"
        cat "$file_path"

        # Files are also available in the uploads directory
        ls -la $FC_ARTIFACTS/uploads/
```

### File Input Limitations

- **No default values**: File inputs cannot have default values
- **Not schedulable**: Flows with file inputs cannot be scheduled (files must be provided at execution time)
- **Size limits**: Default maximum file size is 100MB, configurable per-input via `max_file_size` (in bytes) or globally in server config

### Remote Execution

When running on remote nodes, uploaded files are automatically transferred to the remote node before execution. The file path in your variable will point to the correct location on the remote system.

## Input Validation

Use the `validation` field with expressions to validate input values:

```yaml
inputs:
  - name: port
    type: number
    validation: port >= 1024 && port <= 65535

  - name: username
    type: string
    validation: len(username) >= 3 && len(username) <= 20
```

Validations are [expr](https://expr-lang.org/) statements that should evaluate to either `true` or `false`.

## Remote Options

Select inputs can fetch their options dynamically from a remote HTTP endpoint using `remote_options`. When configured, flowctl calls the endpoint when the input form is displayed and populates the dropdown with the returned values.

```yaml
inputs:
  - name: cluster
    type: select
    label: Target Cluster
    required: true
    remote_options:
      url: "https://api.internal/clusters"
      method: GET
      headers:
        Authorization: "Bearer {{ secrets.API_TOKEN }}"
```

The remote endpoint must return a JSON response with this structure:

```json
{
  "options": ["cluster-a", "cluster-b", "cluster-c"]
}
```

### Configuration

| Field     | Type              | Required | Description                                                      |
| --------- | ----------------- | -------- | ---------------------------------------------------------------- |
| `url`     | string            | Yes      | HTTP or HTTPS URL to fetch options from                          |
| `method`  | string            | No       | HTTP method (`GET` or `POST`). Defaults to `GET`                 |
| `headers` | `map[string]string` | No       | HTTP headers to include in the request                           |

### Header Interpolation

Header values support `{{ expression }}` placeholders evaluated with [expr](https://expr-lang.org/). The following variables are available in header expressions:

- `secrets` — flow secrets (e.g., `secrets.API_TOKEN`)
- `inputs` — current input values (available at trigger time)
- `outputs` — action outputs (available at trigger time)

```yaml
remote_options:
  url: "https://api.internal/environments"
  headers:
    Authorization: "Bearer {{ secrets.INTERNAL_TOKEN }}"
    X-Team: "{{ inputs.team }}"
```

## Next Steps

- Use inputs in [Actions](/docs/general/flow-actions)
- Back to [Flows overview](/docs/general/flows)
