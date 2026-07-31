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

Each field is optional and independent — configure only what you need, the rest falls back to defaults.

## Custom Theme (colors)

Place a `theme.css` file in the branding directory to override CSS variables. It loads after the bundled CSS, so standard cascade rules apply:

```css
/* /app/branding/theme.css */
:root { --primary: #e4002b; }
:root[data-theme="dark"], body[data-theme="dark"] { --primary: #ff5a4d; }
```

The two selectors mirror what `app.css` uses. See all available variables in [app.css](../../site/src/app.css).

## Environment Variables

Use the `FLOWCTL_APP__BRANDING__` prefix:

```bash
FLOWCTL_APP__BRANDING__APP_NAME="ACME Flowctl"
FLOWCTL_APP__BRANDING__LOGO_URL="/branding/logo.svg"
FLOWCTL_APP__BRANDING__ICON_URL="/branding/icon.svg"
FLOWCTL_APP__BRANDING__FAVICON_URL="https://example.com/favicon.png"
FLOWCTL_APP__BRANDING__BRANDING_DIR="/app/branding"
```

## Docker/Podman Setup

```yaml
volumes:
  - ./branding:/app/branding:ro
environment:
  FLOWCTL_APP__BRANDING__APP_NAME: "ACME Flowctl"
  FLOWCTL_APP__BRANDING__LOGO_URL: "/branding/logo.svg"
  FLOWCTL_APP__BRANDING__LOGO_LIGHT_URL: "/branding/logo-light.svg"
  FLOWCTL_APP__BRANDING__ICON_URL: "/branding/icon.svg"
  FLOWCTL_APP__BRANDING__BRANDING_DIR: "/app/branding"
```

The branding directory should contain your custom assets (logos, favicon, `theme.css`). Files are served at `/branding/` and can be referenced in the config using that path.
