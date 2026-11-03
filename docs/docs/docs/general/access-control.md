---
title: Access Control
description: Understanding roles, permissions, and namespaces in flowctl
---

## Overview

Flowctl implements a comprehensive Role-Based Access Control (RBAC) system that manages user permissions through roles and namespaces. Access control is enforced using [Casbin](https://casbin.org/).

## Key Concepts

Flowctl has two levels of user roles:

1. **Global User Roles** - Applied at the application level
   - **Superuser**: Has admin access to all namespaces and can manage all resources
   - **User**: Standard user who only has flow execution access in the default namespace

2. **Namespace Roles** - Applied within specific namespaces
   - **User**: Can view and execute flows, can only see their own executions
   - **Operator**: Everything a User can do, plus view all executions and view flow configurations
   - **Reviewer**: Can view flows, executions, and approve flow actions
   - **Admin**: Full control over namespace resources

!!! note
      The global "superuser" role automatically grants admin access to all
      namespaces. New user by default join the default namespace with **User** role.

## Namespaces

Namespaces are logical containers that organize and isolate resources in flowctl. Each namespace can contain:

- Flows
- Executions
- Nodes (remote servers)
- Credentials
- Secrets
- Members (users and groups with roles)

### Default Namespace

When you first install flowctl, a `default` namespace is automatically created. You can organize your flows into additional namespaces as needed.

## Namespace Roles and Permissions

### User Role

The **User** role is the most restrictive namespace role, suitable for team members who need to run flows but not modify them.

**Permissions:**

- ✓ View flows
- ✓ Execute flows
- ✓ View own executions only
- ✓ View namespace information
- ✓ View namespace members
- ✗ View other users' executions
- ✗ View flow configuration/definition
- ✗ Create, update, or delete flows
- ✗ Approve flow actions
- ✗ Manage nodes, credentials, or secrets
- ✗ Manage namespace members

### Operator Role

The **Operator** role inherits all User permissions and adds visibility into all executions and flow configurations. Suitable for team members who need operational oversight without the ability to approve or modify flows.

**Permissions:**

- ✓ All User role permissions
- ✓ View all executions (not restricted to own)
- ✓ View flow configurations (read-only)
- ✗ Create, update, or delete flows
- ✗ Approve flow actions
- ✗ Manage nodes, credentials, or secrets
- ✗ Manage namespace members

### Reviewer Role

The **Reviewer** role inherits all Operator permissions and adds approval capabilities.

**Permissions:**

- ✓ All User role permissions
- ✓ View approval requests
- ✓ Approve or reject flow actions
- ✗ Create, update, or delete flows
- ✗ Manage nodes, credentials, or secrets
- ✗ Manage namespace members

### Admin Role

The **Admin** role has full control over all resources within a namespace.

**Permissions:**

- ✓ All Reviewer role permissions
- ✓ Create, update, and delete flows
- ✓ View, create, update, and delete nodes
- ✓ View, create, update, and delete credentials
- ✓ View, create, update, and delete secrets
- ✓ Add and remove namespace members
- ✓ Update member roles

!!! warning
      Admin users can manage all resources within their namespace, including
      sensitive credentials and secrets. Grant this role carefully.

## Permission Matrix

Here's a complete breakdown of what each namespace role can do:

| Resource           | User    | Operator | Reviewer | Admin |
| ------------------ | ------- | -------- | -------- | ----- |
| **Flows**          |
| View               | ✓       | ✓        | ✓        | ✓     |
| View Config        | ✗       | ✓        | ✓        | ✓     |
| Create             | ✗       | ✗        | ✗        | ✓     |
| Update             | ✗       | ✗        | ✗        | ✓     |
| Delete             | ✗       | ✗        | ✗        | ✓     |
| Execute            | ✓       | ✓        | ✓        | ✓     |
| **Executions**     |
| View               | Own only| ✓        | ✓        | ✓     |
| **Approvals**      |
| View               | ✗       | ✗        | ✓        | ✓     |
| Approve/Reject     | ✗       | ✗        | ✓        | ✓     |
| **Nodes**          |
| View               | ✗       | ✗        | ✗        | ✓     |
| Create             | ✗       | ✗        | ✗        | ✓     |
| Update             | ✗       | ✗        | ✗        | ✓     |
| Delete             | ✗       | ✗        | ✗        | ✓     |
| **Credentials**    |
| View               | ✗       | ✗        | ✗        | ✓     |
| Create             | ✗       | ✗        | ✗        | ✓     |
| Update             | ✗       | ✗        | ✗        | ✓     |
| Delete             | ✗       | ✗        | ✗        | ✓     |
| **Secrets**        |
| View               | ✗       | ✗        | ✗        | ✓     |
| Create             | ✗       | ✗        | ✗        | ✓     |
| Update             | ✗       | ✗        | ✗        | ✓     |
| Delete             | ✗       | ✗        | ✗        | ✓     |
| **Members**        |
| View               | ✓       | ✓        | ✓        | ✓     |
| Add                | ✗       | ✗        | ✗        | ✓     |
| Update Role        | ✗       | ✗        | ✗        | ✓     |
| Remove             | ✗       | ✗        | ✗        | ✓     |

## Managing Namespace Members

### Adding Members to a Namespace

Namespace admins can add users or groups to their namespace:

1. Go to the "Members" section
2. Click "Add Member"
3. Select the user or group
4. Assign a role (User, Operator, Reviewer, or Admin)
5. Save

## Groups

Flowctl supports groups for easier permission management. Instead of assigning roles to individual users, you can:

1. Create a group (e.g., "DevOps Team", "QA Team")
2. Add users to the group
3. Assign the group to namespaces with specific roles

When a user is added to a group, they automatically inherit all namespace access that the group has.

## Authentication

### Admin User

The initial admin user configured in `config.toml` can authenticate using username and password. This user automatically has the "superuser" global role.

```toml
[app]
  admin_username = "admin"
  admin_password = "secure_password"
```

!!! warning
      Only the admin user can use username/password authentication. All other users
      must use OIDC.

### OIDC Authentication

All non-admin users authenticate via SSO using OIDC. Configure OIDC in your `config.toml`:

```toml
[[oidc]]
name = "oidc"
client_id = "your-client-id"
client_secret = "your-client-secret"
issuer = "https://your-oidc-provider.com/"
```

In your SSO provider's application settings, register `<app.root_url>/auth/callback` as an allowed callback URL (also called a redirect URI). For example, if `app.root_url` is `https://flowctl.example.com`, register `https://flowctl.example.com/auth/callback`. If you set `redirect_url` in the provider's `[[oidc]]` section, register that exact URL instead.

Configure the SSO application to use OpenID Connect with the authorization code flow. The token response must include an `id_token` containing a signed JWT. flowctl verifies this ID token and uses its `email` claim to identify the user, so ensure the provider includes that claim. flowctl requests the `openid`, `profile`, and `email` scopes.

By default, logging out ends only the flowctl session. Set `logout_enabled = true` in a provider's `[[oidc]]` section to also log out of that SSO provider. This uses the provider's discovered logout endpoint; set `logout_url` to override it. If no logout endpoint is available, flowctl still ends the local session.

### Auto-Creating Users on Login

By default, a user must already exist in flowctl before they can log in via OIDC. Enable `auto_create_users` on an OIDC provider to create accounts automatically on first login.

```toml
[[oidc]]
name = "oidc"
client_id = "your-client-id"
client_secret = "your-client-secret"
issuer = "https://your-oidc-provider.com/"

auto_create_users.enabled = true
auto_create_users.namespace = "default"
auto_create_users.role = "user"
auto_create_users.groups = ["developers"]
auto_create_users.allowed_domains = ["example.com"]
```

**Options:**

| Option | Type | Description |
| --- | --- | --- |
| `enabled` | bool | Enable auto user creation. Default: `false`. |
| `namespace` | string | Namespace to add the new user to. Default: `"default"`. |
| `role` | string | Namespace role assigned to the new user: `user`, `operator`, `reviewer`, or `admin`. Default: `"user"`. |
| `groups` | array | List of existing group names to add the new user to. |
| `allowed_domains` | array | If set, only email addresses from these domains can create accounts. Leave empty to allow any domain. |

!!! warning
      If `allowed_domains` is empty, any user who can authenticate with your OIDC
      provider will have an account created automatically. Set it to restrict
      sign-ups to your organisation's domain.
