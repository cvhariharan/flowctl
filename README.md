# flowctl

An open-source self-service workflow execution platform for running automated tasks.

## Features

- **Workflows** - Define complex workflows using simple YAML/[HUML](https://huml.io) configuration with inputs, actions, and approvals
- **SSO** - Secure authentication using OIDC
- **Approvals** - Add approvals to sensitive operations
- **Teams** - Organize workflows by teams or projects with isolated namespaces and built-in RBAC
- **Remote Execution** - Execute workflows on remote nodes via SSH
- **Secure Secrets** - Store SSH keys, passwords, and secrets securely with encrypted storage
- **Real-time Logs** - Track workflow executions with streaming logs
- **Scheduling** - Automate workflows with cron-based scheduling

## Quick Start

### Prerequisites

- PostgreSQL database
- Docker (optional, for Docker executor)

### Installation

1. Download the latest binary from [releases](https://github.com/cvhariharan/flowctl/releases)

2. Start PostgreSQL:

   ```bash
   docker run -d \
     --name flowctl-postgres \
     -e POSTGRES_USER=flowctl \
     -e POSTGRES_PASSWORD=flowctl \
     -e POSTGRES_DB=flowctl \
     -p 5432:5432 \
     postgres:17-alpine
   ```

3. Generate configuration:

   ```bash
   flowctl --new-config
   ```

   Create the default `flows_directory` directory as specified in the `config.toml` file. This is where the flow files will be saved.

4. Start flowctl:

   ```bash
   flowctl start
   ```

5. Access the UI at [http://localhost:7000](http://localhost:7000)

## Example Workflow

```yaml
metadata:
  id: hello_world
  name: Hello World
  description: A simple greeting flow

inputs:
  - name: username
    type: string
    label: Username
    required: true

actions:
  - id: greet
    name: Greet User
    executor: docker
    variables:
      - username: "{{ input.username }}"
    with:
      image: docker.io/alpine
      script: |
        echo "Hello, $username!"
```

## Documentation

Full documentation is available at [flowctl.net](https://flowctl.net)

## Configuration

flowctl is configured via `config.toml`:

```toml
[app]
  admin_username = "flowctl_admin"
  admin_password = "your_secure_password"
  flows_directory = "flows"
  root_url = "http://localhost:7000"

[db]
  host = "127.0.0.1"
  port = 5432
  dbname = "flowctl"
  user = "flowctl"
  password = "flowctl"

[app.scheduler]
  workers = 20
  cron_sync_interval = "5m0s"
```

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.
