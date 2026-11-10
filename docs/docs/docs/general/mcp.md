---
title: MCP
description: Connect AI agents to flowctl with personal API tokens
---

# MCP

Flowctl can serve a [Model Context Protocol](https://modelcontextprotocol.io) endpoint at `<root_url>/mcp`. Any MCP client, such as Claude Code, Claude Desktop, IDE agents or an agent built on an MCP SDK, can then list flows, trigger them, wait for results, read logs, write flows and manage schedules.

An agent acts as the owner of the API token it connects with. It has exactly that user's permissions in each namespace.

## Enabling the endpoint

The endpoint is off by default. Turn it on in `config.toml`:

```toml
[mcp]
enabled = true
max_body_size = 1048576
```

`max_body_size` caps the size of a request to the endpoint, in bytes. The default is 1 MiB. Raise it if agents need to send flow definitions larger than that to `create_flow` or `update_flow`.

Or with environment variables:

```bash
FLOWCTL_MCP__ENABLED=true
FLOWCTL_MCP__MAX_BODY_SIZE=1048576
```

## Creating a token

Agents authenticate with a personal API token. Open the user menu, choose **Credentials**, and create a token.

## Connecting a client

### Claude Code

```bash
claude mcp add --transport http flowctl https://flowctl.example.com/mcp \
  --header "Authorization: Bearer fctl_pat_..."
```

### Other clients

Any client that supports the Streamable HTTP transport works. Configure:

- **URL:** `https://flowctl.example.com/mcp`
- **Header:** `Authorization: Bearer fctl_pat_...`

For clients that use a JSON config file:

```json
{
  "mcpServers": {
    "flowctl": {
      "type": "http",
      "url": "https://flowctl.example.com/mcp",
      "headers": { "Authorization": "Bearer fctl_pat_..." }
    }
  }
}
```

## Tools

| Tool | What it does |
|------|--------------|
| `list_namespaces` | List the namespaces you belong to |
| `list_flows` | List the flows in a namespace |
| `get_flow` | Get a flow's metadata, actions and inputs |
| `trigger_flow` | Run a flow, now or at a scheduled time |
| `get_execution` | Get an execution's status |
| `list_executions` | List the executions of a flow or namespace |
| `wait_for_execution` | Wait up to 120 seconds for an execution to finish |
| `get_execution_logs` | Get the last lines of a finished execution's logs |
| `cancel_execution` | Cancel an execution |
| `retry_execution` | Retry an execution, optionally from a given action |
| `list_executors` | List the executors available to actions |
| `get_executor_schema` | Get the config schema of an executor |
| `list_nodes` | List the nodes in a namespace |
| `list_credentials` | List credential names and key types (never key material) |
| `get_flow_definition` | Get a flow's full definition |
| `create_flow` | Create a flow |
| `update_flow` | Update a flow, saving changed source as a version |
| `delete_flow` | Delete a flow from a namespace |
| `list_flow_versions` | List a flow's saved versions |
| `get_flow_version` | Get the source of one version |
| `diff_flow_versions` | Diff a flow's source between two versions |
| `activate_flow_version` | Make an earlier version the active one |
| `list_schedules` | List a flow's cron schedules |
| `create_schedule` | Add a cron schedule |
| `update_schedule` | Change a cron schedule |
| `delete_schedule` | Delete a cron schedule |

Things an agent can't do:

- **Approve or reject approval gates.** When an execution reaches an approval, `wait_for_execution` stops and returns a link to the execution. A person approves it in the flowctl UI.
- **Use file inputs.** Flows with a required file input can't be triggered over MCP.
- **Read logs of a running execution.** Logs are available once the execution has completed, errored or been cancelled.
- **Manage** secrets, nodes, credentials, members, users or namespaces.

Changes to a flow's source are saved as [flow versions](/docs/general/flow-versions), so a bad edit
by an agent can be rolled back from the UI or with `activate_flow_version`.

## Flow authoring skill

The repository includes a [flowctl skill](https://github.com/cvhariharan/flowctl/tree/master/skills/flowctl)
for agents that support `SKILL.md` skills. Copy the whole `skills/flowctl` directory into your
client's skills directory, keeping its `references` and `assets` subdirectories. Connect the MCP
server above before using the skill.

## Data sent to the model

Every tool result goes to the model provider your agent uses. That includes execution logs, flow definitions, node names and input values. This is no different from piping `curl` output into a language model, but check that it fits your organization's data policies before you connect an agent to a production instance.
