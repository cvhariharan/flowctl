# Custom Branding

Flowctl supports custom branding to match your organization's identity. Configure it under `[app.branding]` in `config.toml` or via environment variables.

## Configuration

```toml
[app.branding]
  app_name = "ACME Flowctl"
  logo_url = "/branding/logo.svg"
  logo_light_url = "/branding/logo-light.svg"
  icon_url = "/branding/icon.svg"
  favicon_url = "/branding/favicon.png"
  branding_dir = "/app/branding"
```

| Field | Description | Default |
|-------|-------------|---------|
| `app_name` | Custom page title | "Flowctl" |
| `logo_url` | Logo for light theme (local path or URL) | Embedded logo |
| `logo_light_url` | Logo for dark theme | Falls back to `logo_url` |
| `icon_url` | Small icon for collapsed sidebar | Embedded icon |
| `favicon_url` | Favicon (local path or URL) | Embedded favicon |
| `branding_dir` | Local directory to serve at `/branding/` | Not set |

## Custom Theme

Place a `theme.css` file in the branding directory to override CSS variables. It loads after the bundled CSS:

```css
/* /app/branding/theme.css */
:root { --primary: #e4002b; }
:root[data-theme="dark"], body[data-theme="dark"] { --primary: #ff5a4d; }
```

See all available variables in [app.css](https://github.com/cvhariharan/flowctl/blob/master/site/src/app.css).

## Environment Variables

Use the `FLOWCTL_APP__BRANDING__` prefix:

```bash
FLOWCTL_APP__BRANDING__APP_NAME="ACME Flowctl"
FLOWCTL_APP__BRANDING__LOGO_URL="/branding/logo.svg"
FLOWCTL_APP__BRANDING__ICON_URL="/branding/icon.svg"
FLOWCTL_APP__BRANDING__FAVICON_URL="https://example.com/favicon.png"
FLOWCTL_APP__BRANDING__BRANDING_DIR="/app/branding"
```
