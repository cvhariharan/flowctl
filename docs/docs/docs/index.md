---
title: Getting Started
---

Flowctl uses PostgreSQL as its database. This is the only external dependency. Everything else is included in the binary.

## Quick Start

For a quick setup, you can use the provided [docker-compose.yml](https://raw.githubusercontent.com/cvhariharan/flowctl/refs/heads/master/docker-compose.yaml) file.
Alternatively, you can set up flowctl manually with the following steps.

1. **Download the binary**

   Download the latest release for your platform from [GitHub Releases](https://github.com/cvhariharan/flowctl/releases).

2. **Configure flowctl**

   ```bash
   flowctl --new-config
   ```

   This will generate a default `config.toml` file in the current directory. Customize it if required.
   Create the flows directory as specified in `flows_directory` in `config.toml`.

3. **DB migration**

   ```bash
   flowctl install
   ```

   This will perform DB migrations. It is safe to run this multiple times.

4. **Start flowctl**

   ```bash
   flowctl start
   ```

   This will start the server and the background workers.
   The application will be available at [http://localhost:7000](http://localhost:7000). This will use the `config.toml` in the current working directory by default. You can pass `--config` flag to use a different config.

5. **Login**

   Use the admin credentials you configured in `config.toml`.

## Configuration

The `config.toml` file controls all aspects of flowctl's behavior. Here's a comprehensive guide to the available settings:

### Application Settings

```toml
[app]
  admin_username = "flowctl_admin"
  admin_password = "your_secure_password"
  flows_directory = "flows"
  root_url = "http://localhost:7000"
  address = ":7000"
  use_tls = false
  http_tls_cert = "server_cert.pem"
  http_tls_key = "server_key.pem"
  max_file_upload_size = 104857600
  plugin_dir = "/opt/flowctl/plugins"
```

- **`admin_username`** (required): Admin user account username.
- **`admin_password`** (required): Admin user account password. Change the default password for security.
- **`root_url`** (required): The base URL where flowctl is accessible. Update this if running behind a proxy or on a different port.
- **`address`** (required): Address for the server to listen on (default: `:7000` listens on all interfaces).
- **`flows_directory`** (required): Directory path where flow definitions are stored. Can be relative or absolute.
- **`use_tls`** (optional): Enable HTTPS (default: `false`).
- **`http_tls_cert`** (required if `use_tls` is true): Path to TLS certificate file.
- **`http_tls_key`** (required if `use_tls` is true): Path to TLS key file.
- **`max_file_upload_size`** (required): Maximum file upload size in bytes (default: 104857600 = 100MB).
- **`plugin_dir`** (optional): Directory to load external executor plugin binaries from. See [Writing Executor Plugins](/docs/advanced/executor-plugins).

### Database Settings

```toml
[db]
  host = "127.0.0.1"
  port = 5432
  dbname = "flowctl"
  user = "flowctl"
  password = "flowctl"
  sslmode = ""
  sslcert = ""
  sslkey = ""
  sslrootcert = ""
  # dsn = ""
```

- **`dsn`** (optional): Complete PostgreSQL connection string. If set, overrides all other database settings.
- **`host`** (required unless `dsn` is set): PostgreSQL server hostname or IP address.
- **`port`** (required unless `dsn` is set): PostgreSQL server port (default: `5432`).
- **`dbname`** (required unless `dsn` is set): Name of the database to use.
- **`user`** (required unless `dsn` is set): Database authentication username.
- **`password`** (required unless `dsn` is set): Database authentication password.
- **`sslmode`** (optional): Postgres SSL mode (`disable`, `allow`, `prefer`, `require`, `verify-ca`, `verify-full`). Default: `disable`.
- **`sslcert`** / **`sslkey`** / **`sslrootcert`** (optional): Paths to SSL certificate, key, and root certificate respectively.

### Keystore Configuration

```toml
[keystore]
  keeper_url = "base64key://..."
```

- **`keeper_url`** (required): The keystore manages encryption keys for sensitive data. Uses a URI scheme to specify the storage backend. This uses the [gocloud](https://gocloud.dev/howto/secrets/) package which supports multiple KMS backends. The default config uses a local encryption key.

!!! warning
      Ensure that this key is backed-up and saved. Losing this key will make the
      secrets and credentials inaccessible.

### Scheduler Settings

```toml
[scheduler]
  workers = 20
  cron_sync_interval = "5m0s"
  flow_execution_timeout = "1h"
```

- **`workers`** (required): Number of concurrent workers for executing flows (default: number of CPU threads).
- **`cron_sync_interval`** (required): How often to sync scheduled flows from the database (default: `5m0s`).
- **`flow_execution_timeout`** (required): Maximum duration for flow execution before termination (default: `1h`).

### Logger Configuration

```toml
[logger]
  backend = "file"
  log_directory = "/var/log/flowctl"
  max_size_bytes = 0
  retention_time = "0s"
  scan_interval = "1h"
```

- **`backend`** (required): Log storage backend. Currently only `file` is supported.
- **`log_directory`** (required): Directory for log files when using file backend. This directory should exist.
- **`max_size_bytes`** (required): Maximum size per log file in bytes (0 = unlimited).
- **`retention_time`** (required): How long to keep log files (0 = unlimited). Format: duration string (e.g., `24h`, `7d`).
- **`scan_interval`** (required): Interval between scans for the log manager to delete / manage logs.

### Email Notifications (SMTP)

```toml
[messengers.email]
  enabled = true
  host = "smtp.example.com"
  port = 587
  username = "username"
  password = "password"
  from_address = "flowctl@example.com"
  from_name = "Flowctl Notifications"
  max_conns = 10
  ssl = "starttls"
```

Configure SMTP settings to enable email notifications for flow events. Email notifications are required for the notification feature to work.

- **`enabled`** (optional): Enable or disable email notifications (default: `false`).
- **`host`** (required): SMTP server hostname.
- **`port`** (required): SMTP server port.
- **`username`** (optional): SMTP authentication username.
- **`password`** (optional): SMTP authentication password.
- **`from_address`** (required): Email address that notifications will be sent from.
- **`from_name`** (optional): Display name for the sender.
- **`max_conns`** (required): Maximum number of concurrent SMTP connections (default: `10`).
- **`ssl`** (required): SSL/TLS mode - `none`, `tls` (implicit TLS), or `starttls` (explicit TLS).

!!! note
      SMTP configuration is required for email notifications to work. See the [Flow
      Notifications documentation](/docs/general/flow-notifications) for how to
      configure notifications in your flows.

### Webhook Notifications

```toml
[messengers.webhook]
  enabled = true
  signing_key = "whsk_your_base64_encoded_ed25519_key"
  timeout = "30s"
```

Configure webhook settings to enable webhook notifications for flow events. Webhooks use the [Standard Webhooks](https://www.standardwebhooks.com/) format with Ed25519 asymmetric signing.

- **`enabled`** (optional): Enable or disable webhook notifications (default: `false`).
- **`signing_key`** (required): Base64-encoded Ed25519 private key seed used to sign webhook payloads. Can optionally be prefixed with `whsk_`. Generate one with:
  ```bash
  openssl genpkey -algorithm ed25519 -outform DER | tail -c 32 | base64
  ```
- **`timeout`** (optional): HTTP request timeout for webhook delivery (default: `30s`).

### OIDC Authentication

```toml
[[oidc]]
  name = "oidc"
  client_id = "your-client-id"
  client_secret = "your-client-secret"
  issuer = "https://your-oidc-provider.com/"
  auth_url = ""
  token_url = ""
  redirect_url = ""
  label = "Sign in with OIDC"
  auto_create_users.enabled = false
  auto_create_users.namespace = "default"
  auto_create_users.role = "user"
  auto_create_users.groups = []
  auto_create_users.allowed_domains = ["example.com"]
```

!!! note
      Currently only the default admin user can use a username and password to log
      into flowctl. All other users are expected to use OIDC.

Configure OpenID Connect for single sign-on. Multiple OIDC providers can be configured by adding additional `[[oidc]]` sections:

- **`name`** (required): Unique identifier for the OIDC provider.
- **`client_id`** (required): OAuth2 client ID from your OIDC provider.
- **`client_secret`** (required): OAuth2 client secret.
- **`issuer`** (required): OIDC provider's issuer URL.
- **`auth_url`** (optional): Authorization URL of the OIDC provider.
- **`token_url`** (optional): Token URL of the OIDC provider.
- **`redirect_url`** (optional): Callback URL for OIDC authentication.
- **`label`** (optional): Label for the SSO button.
- **`auto_create_users.enabled`** (optional): Automatically create a flowctl account for users logging in for the first time (default: `false`). See [Auto-Creating Users on Login](/docs/general/access-control#auto-creating-users-on-login).
- **`auto_create_users.namespace`** (optional): Namespace to add new users to (default: `"default"`).
- **`auto_create_users.role`** (optional): Namespace role for new users: `user`, `operator`, `reviewer`, or `admin` (default: `"user"`).
- **`auto_create_users.groups`** (optional): List of existing group names to add new users to.
- **`auto_create_users.allowed_domains`** (optional): Restrict auto-creation to specific email domains. Leave empty to allow any domain.

### Metrics

```toml
[metrics]
  enabled = true
  path = "/metrics"
```

Export Prometheus metrics

- **`enabled`** (optional): Enable or disable metrics export (default: `false`).
- **`path`** (optional): URL path where metrics will be exposed (default: `/metrics`).

## Next Steps

- Learn how to create your [first flow](/docs/general/flows)
- Configure [executors and nodes](/docs/general/nodes-and-executors)
