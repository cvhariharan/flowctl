---
name: flowctl
description: "Design and create flowctl flows from a described use case through the flowctl MCP server. Use this whenever someone wants to automate a task in flowctl: turn a runbook, shell steps or a manual procedure into a flow, build a deploy, backup, rollout, cleanup, maintenance or approval-gated workflow, or add a flow to a flowctl namespace, even if they never say the word flow. It discovers the server's executors, nodes and existing flows, drafts a definition that uses flowctl's own features (typed inputs, variables and expressions, secrets, outputs, artifacts, DAG mode, approvals, node targeting, child flows, schedules, notifications), creates it with create_flow, and test-runs it."
license: Apache-2.0
compatibility: "Requires the flowctl MCP server, connected with a personal API token (tools such as list_executors, get_executor_schema, create_flow and trigger_flow)."
metadata:
  project: flowctl
  version: "1"
---

# Authoring flowctl flows

You turn a use case into a working flow on a flowctl server, using the flowctl MCP tools. The job is
done when the flow exists on the server, the user knows which secrets to add, and (if they agree) a
test run has passed.

A good flow is not a shell script split into boxes. It uses flowctl for what flowctl is good at:
typed inputs the operator fills in, approvals in front of anything irreversible, secrets that never
appear in the definition, outputs and artifacts that carry results between steps, and parallel
branches where steps are truly independent. Reach for those features whenever the use case has a
place for them.

## How flowctl runs a flow

Keep this model in mind while designing; most mistakes come from getting one of these wrong.

- A flow is a list of **actions**. Each action has an **executor** (`docker`, `script`, `flow`, or a
  plugin) and a `with` block that configures it. It runs on the flowctl host, or on every node in
  its `on` list in parallel.
- **`with` is passed to the executor verbatim.** `{{ }}` templates inside a script are never
  evaluated. Only `variables` are evaluated, and each variable becomes an environment variable in the
  script. Every value an action needs (inputs, secrets, earlier outputs) goes in through `variables`.
- Actions pass **small values** through outputs (lines of `KEY=value` written to `$FC_OUTPUT`) and
  **files** through artifacts (files written to `$FC_ARTIFACTS`).
- **Secrets** live on the server, not in the definition. A flow sees the namespace's secrets merged
  with its own flow secrets (the flow's value wins on a clash). They can only be added in the UI.
- Actions run top to bottom by default. With `execution_mode: dag`, each action lists what it
  `needs` and independent branches run in parallel.
- An action with `approval: true` pauses the run until a namespace Reviewer or Admin approves it in
  the UI. You (the agent) can't approve, and neither can the MCP tools.

## Workflow

### 1. Discover

Make these calls together, before designing anything:

- `list_namespaces`, unless the user named the namespace.
- `list_flows` in that namespace: avoid a name clash, and find flows a `flow` action could reuse.
- `list_executors`, then `get_executor_schema` for each executor you plan to use. Plugins can add
  executors beyond `docker`, `script` and `flow`, and the schema is the source of truth for `with`.
- `list_nodes`: real node names and their tags, if anything runs on remote hosts.

If the flowctl MCP tools aren't available, stop and tell the user. This skill creates flows through
the server: it needs `[mcp] enabled = true` in flowctl's config and the server added to the agent
with a personal API token (see flowctl's "AI Agents (MCP)" docs page).

### 2. Design

Break the use case into steps, then decide for each step how it runs and what it hands on. Use
`references/features.md` to map needs to features:

| The use case needs… | Use |
| --- | --- |
| values that change per run | typed `inputs`, with `validation` and defaults |
| a credential, token or password | a secret, read through `variables` |
| a value computed by one step and used by a later one | `$FC_OUTPUT` and `{{ outputs.KEY }}` |
| one value from a step that runs on several nodes | `$FC_OUTPUT_GLOBAL` and `{{ outputs.global.<action_id>.KEY }}` |
| a file produced by one step and used by a later one | `$FC_ARTIFACTS` |
| work on specific hosts, or "all web servers" | `on` with node names or `tag:<tag>` |
| the operator picks the hosts at trigger time | a `node` input and `allow_node_override: true` |
| steps that don't depend on each other | `execution_mode: dag` and `needs` |
| a human sign-off before a risky step | `approval: true` on that step |
| reuse of an existing flow | the `flow` executor |
| recurring runs | `metadata.schedules` or `user_schedulable: true` |
| people told about failures or pending approvals | `notify` |

Design rules that keep flows safe to operate:

- **Read-only checks first, ungated.** The approver should see the current state in the logs before
  approving the change.
- **Gate the destructive action itself.** Put `approval: true` on the step that changes production,
  not on the step after it.
- **One action per step that can fail on its own.** A failure or a rejected approval should leave
  the system in a state someone can reason about, and the operator can re-run from that action.
- **Make every step safe to repeat.** Retries, resumes and re-runs start an action again from the
  top. Check before you change, and use `$FC_ATTEMPT` when a repeat needs cleanup first.
- **Use DAG mode only for real parallelism.** If every step depends on the previous one, keep the
  default sequential mode.

### 3. Ask only what you can't infer

Draft first. Infer whatever follows from the use case and what discovery found: input types,
defaults, which steps need approval, which values are secrets, sequential or DAG. Ask the user only
for things that block a correct flow, and batch the questions:

- target hosts or tags that discovery didn't settle,
- names of secrets that may already exist in the namespace,
- who should be notified, and on which events,
- anything where a wrong guess would touch the wrong system.

Then show the plan compactly and wait for a yes before creating anything:

```
Flow: Postgres Backup (slug postgres_backup, namespace ops, sequential)
Inputs: database:select=orders, backup_name:string=<today>, verify_restore:checkbox=true
Actions:
  1. dump_database   docker postgres:16-alpine, local    writes artifact + DUMP_FILE
  2. verify_dump     docker postgres:16-alpine, local    reads the artifact
  3. upload_to_s3    docker alpine, local                reads the artifact
Secrets to add: PG_HOST, PG_USER, PG_PASSWORD, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, BACKUP_BUCKET
Schedule: nightly 02:00 UTC; notify group:dba on failure
```

### 4. Write the definition

`create_flow` takes a JSON definition. Read `references/definition.md` for its exact shape and the
rules the server enforces, and `references/features.md` for how each feature behaves. Two complete,
validated definitions are in `assets/examples/`; read the one closest to your use case before writing:

- `assets/examples/postgres_backup.json`: sequential, secrets, dynamic default, outputs, artifacts,
  schedule, notification.
- `assets/examples/service_release.json`: DAG, operator-selected nodes, node-scoped and global
  outputs, artifacts pushed to remote nodes, approval gate, child flow, notifications.

The rules that most often break a flow, because the server accepts the definition but the run goes
wrong:

- **Action ids come from action names.** `create_flow` ignores the `id` you send and uses the name,
  lowercased, with runs of spaces turned into `_` ("Run DB Migrations" → `run_db_migrations`).
  `needs` and `outputs.global.<action_id>` must use that id. Set `id` to the same value so the
  definition reads correctly.
- **A variable value is one string holding at most one `{{ }}` expression.** Text around the
  expression is dropped, so `"backup-{{ inputs.db }}.sql"` yields just the database name. Build the
  string inside the expression instead: `"{{ 'backup-' + inputs.db + '.sql' }}"`. Non-string values
  (numbers, booleans) are turned into empty strings, so quote them.
- **The Docker executor runs scripts with `/bin/sh`.** Debian-based images use dash, which has no
  `set -o pipefail` and no bash syntax. Use POSIX sh with `set -eu`, or set `interpreter: /bin/bash`
  on an image that has bash. Images that set their own `ENTRYPOINT` (for example `amazon/aws-cli`
  or `hashicorp/terraform`) get the script as arguments to that entrypoint and won't run it, so
  start from a shell image and install the tool.
- **Never put a credential in the definition**, not in `with`, `variables` or `remote_options`.
  Reference `{{ secrets.NAME }}` and tell the user to add `NAME`.

### 5. Create

Call `create_flow` with the namespace and the definition. A validation error names the field: fix
it and call again. If a flow with that name already exists, ask whether to pick another name or to
change the existing flow with `update_flow`.

### 6. Hand off the secrets

The MCP tools can't create secrets. List every `secrets.NAME` the flow reads and ask the user to add
the ones that don't exist: under the flow's **Secrets** tab, or as namespace secrets if other flows
share them. A missing secret evaluates to an empty string, so the run starts and then fails
somewhere confusing. Wait for the user to confirm before you test.

### 7. Test

Ask before triggering anything that touches real systems, and suggest the safest inputs (a staging
target, a dry-run flag). Then:

1. `get_flow`, to see the inputs as the server resolved them.
2. `trigger_flow` with the agreed inputs.
3. `wait_for_execution`, calling it again while `done` is false. At an approval gate it stops and
   returns the execution URL; tell the user to approve there.
4. `get_execution_logs` once it has finished. Read them even on success: an empty variable or a
   skipped branch shows up there first.

To fix the flow, call `get_flow_definition`, change it, and call `update_flow`. `update_flow` takes
a flatter shape than `create_flow` (see `references/definition.md`). Every update creates a new
version: `diff_flow_versions` shows what changed, and `activate_flow_version` rolls back.

### 8. Report

Finish with a short summary:

- the flow's name, slug, namespace, and a link: `<root_url>/view/<namespace>/flows/<slug>`,
- what each action does, in one line each,
- the secrets, nodes and tags the flow depends on, and which of them still need setting up,
- who can approve the gated steps (namespace Reviewers and Admins),
- the test result, or why it wasn't run.
