# flowctl features, and how to use them in a flow

How each feature behaves at run time, with the JSON to use it. Snippets are fragments of a
`create_flow` definition (see `definition.md` for the full shape).

## Contents

- [Variables and expressions](#variables-and-expressions)
- [Inputs inside scripts](#inputs-inside-scripts)
- [Secrets](#secrets)
- [Outputs](#outputs)
- [Artifacts](#artifacts)
- [Remote nodes and tags](#remote-nodes-and-tags)
- [Operator-selected nodes](#operator-selected-nodes)
- [Approvals](#approvals)
- [DAG mode](#dag-mode)
- [Child flows](#child-flows)
- [Retries, resumes and idempotency](#retries-resumes-and-idempotency)
- [Schedules and overlap](#schedules-and-overlap)
- [Notifications](#notifications)

## Variables and expressions

Variables are the only place flowctl evaluates expressions at run time. Each is a one-key object;
the key becomes an environment variable in the script.

```json
"variables": [
  { "DATABASE": "{{ inputs.database }}" },
  { "PGPASSWORD": "{{ secrets.PG_PASSWORD }}" },
  { "DUMP_FILE": "{{ outputs.DUMP_FILE }}" },
  { "TARGET": "{{ inputs.env == 'production' ? 'prod-db' : 'staging-db' }}" },
  { "OBJECT_KEY": "{{ meta.Namespace + '/' + inputs.database + '-' + inputs.backup_name + '.dump' }}" },
  { "HOSTS": "{{ join(inputs.hosts, ',') }}" },
  { "MODE": "fast" }
]
```

- Expressions are [expr](https://expr-lang.org/docs/language-definition). They can read `inputs`,
  `secrets`, `outputs` and `meta` (`meta.ID`, `meta.Name`, `meta.Namespace`, `meta.Description` of
  the flow). `upper`, `lower`, `trim`, `split`, `join`, `len`, `now()`, the ternary `?:` and
  `matches` are the common ones. Strings can use single quotes, which keeps the JSON readable.
- Only the **first** `{{ }}` in a value is evaluated, and the text around it is dropped. Build
  strings inside one expression, as `OBJECT_KEY` does above.
- A value with no `{{ }}` is passed literally.
- The value must be a JSON string. A number or boolean becomes an empty string.
- Every action also gets `FC_EXEC_ID` and `FC_ATTEMPT` (see [Retries](#retries-resumes-and-idempotency)).

## Inputs inside scripts

Inputs reach a script only through variables. Their types at run time:

| Input type | `{{ inputs.x }}` is | In the script |
| --- | --- | --- |
| `string`, `password`, `select`, `datetime` | string | as entered |
| `number` | int or float | `3`, `2.5` |
| `checkbox` | bool | `true` or `false` |
| `node` | list of names / `tag:` targets | use `join(inputs.x, ',')` |
| `file` | path of the uploaded file | already copied to the node; also under `$FC_ARTIFACTS/uploads/` |

`password` only hides the value in the trigger form. Anything the flow always needs, like an API
token, belongs in a secret, not in an input the operator has to paste.

Give inputs a `validation` whenever a bad value could do harm: an environment name, a version tag, a
count. The run is refused before anything executes, which beats a script failing halfway.

## Secrets

```json
"variables": [ { "AWS_SECRET_ACCESS_KEY": "{{ secrets.AWS_SECRET_ACCESS_KEY }}" } ]
```

- A flow reads the **namespace secrets merged with its own flow secrets**. The flow's value wins on
  a clash. Credentials shared by several flows (a registry token, a cloud key) are best kept as
  namespace secrets; per-flow ones as flow secrets.
- Secrets are added in the UI only (the flow's **Secrets** tab, or the namespace's secrets page).
  Flow secrets can only be added after the flow exists. The MCP tools can't set either, so list the
  keys for the user.
- A missing secret evaluates to an empty string. Have the script fail fast when a required value is
  empty: `: "${PGPASSWORD:?PG_PASSWORD secret is not set}"`.
- Secrets also work in `remote_options.headers`.
- Never echo a secret. Scripts' stdout and stderr go into the execution log.

## Outputs

An action publishes a value by appending `KEY=value` lines to the file named by `$FC_OUTPUT`. Keys
are sanitised to letters, digits and `_`.

```sh
echo "DUMP_FILE=$FILE" >> "$FC_OUTPUT"
echo "DUMP_BYTES=$(wc -c < "$FC_ARTIFACTS/$FILE")" >> "$FC_OUTPUT"
```

Later actions read it through a variable. Where the value lives depends on where the producer ran:

| Producer ran on | Written to | Read as |
| --- | --- | --- |
| the flowctl host (no `on`) | `$FC_OUTPUT` | `{{ outputs.KEY }}` |
| node `web1` | `$FC_OUTPUT` | `{{ outputs.web1.KEY }}`, one value per node |
| any node, one value for the whole action | `$FC_OUTPUT_GLOBAL` | `{{ outputs.global.<action_id>.KEY }}` |

- Outputs from the flowctl host share one flat namespace, so a later action that writes the same key
  overwrites it. Use distinct key names.
- With a node selected at trigger time you don't know the node names in advance. Write values that
  later steps need to `$FC_OUTPUT_GLOBAL` and read them by action id. If several nodes write the same
  global key, the last one to finish wins.
- `global` is reserved, so no node can be called that.
- Outputs are for short values. Put anything large or binary in an artifact.

## Artifacts

Every execution has an artifact store that moves files between actions and nodes.

- An action writes files to the top level of `$FC_ARTIFACTS`. Subdirectories are ignored.
- After it finishes, its files are stored under the name of where it ran: `local` for the flowctl
  host, or the node name.
- Every later action, on any node, finds them at `$FC_ARTIFACTS/<where>/<file>`. Files from the
  flowctl host are copied to remote nodes before a remote action starts, which is how you ship a
  build to servers.

```json
{ "id": "build", "name": "Build", "executor": "docker",
  "with": { "image": "golang:1.23-alpine", "script": "set -eu\ngo build -o \"$FC_ARTIFACTS/app\" ./cmd/app" } },
{ "id": "install", "name": "Install", "executor": "script", "on": ["tag:web"],
  "with": { "script": "set -euo pipefail\ninstall -m 0755 \"$FC_ARTIFACTS/local/app\" /usr/local/bin/app" } }
```

- In DAG mode, parallel actions share the store: two branches writing the same filename overwrite
  each other. Include the action or node in the filename when that can happen.
- The store is removed when the execution ends. Upload anything that must outlive the run (to S3,
  a registry, a server).

## Remote nodes and tags

```json
"on": ["db-primary"]
"on": ["tag:web"]
"on": ["web1", "tag:canary"]
```

- Use names and tags exactly as `list_nodes` returns them. Tags resolve when the run starts, so
  adding a node to a tag changes later runs without editing the flow.
- A multi-node action runs on all targets in parallel and fails if any one fails. For a rolling
  change, use one action per batch (`tag:canary`, then `tag:web`), with an approval between them.
- `script` on a node runs as the node's SSH user. `docker` on a node needs Docker on that host.
- The `flow` executor ignores `on`; it always runs on the flowctl host.

## Operator-selected nodes

To let the operator choose the targets at trigger time, add one `node` input and set
`allow_node_override: true` on each action that should follow it. Keep a sensible `on` as the
fallback when the input is left empty.

```json
"inputs": [
  { "name": "hosts", "type": "node", "label": "Target hosts", "description": "Nodes or tag:<tag>" }
],
"actions": [
  { "id": "restart", "name": "Restart", "executor": "script", "on": ["tag:web"], "allow_node_override": true,
    "with": { "script": "set -euo pipefail\nsystemctl restart app" } }
]
```

The operator can pick node names or `tag:` targets. Only one `node` input is allowed per flow.

## Approvals

```json
{ "id": "apply_migration", "name": "Apply Migration", "executor": "docker", "approval": true, "with": { "…": "…" } }
```

- The run pauses before this action until a namespace **Reviewer** or **Admin** approves it in the
  UI. Rejecting cancels the whole execution.
- Put read-only checks in earlier actions so their logs are in front of the approver.
- In DAG mode an approval blocks only its own branch; other branches keep running.
- Pair approvals with an `on_waiting` notification so the reviewers learn something is waiting.
- Re-running from an action asks for the approval again.

## DAG mode

```json
"metadata": { "name": "Deploy Service", "execution_mode": "dag" },
"actions": [
  { "id": "build", "name": "Build", "executor": "docker", "with": { "…": "…" } },
  { "id": "unit_tests", "name": "Unit Tests", "executor": "docker", "needs": ["build"], "with": { "…": "…" } },
  { "id": "lint", "name": "Lint", "executor": "docker", "needs": ["build"], "with": { "…": "…" } },
  { "id": "deploy", "name": "Deploy", "executor": "script", "needs": ["unit_tests", "lint"], "approval": true, "with": { "…": "…" } }
]
```

- Actions with no `needs` start right away; the rest wait for every action they list.
- Read outputs only from actions in your `needs`, directly or transitively. Anything else depends on
  which branch finished first.
- When an action fails, running actions finish, nothing new starts, and the rest are marked skipped.
- How many actions run at once is capped server-wide (`scheduler.max_parallel`).

## Child flows

Use the `flow` executor to reuse a flow that already exists in the namespace, for example a shared
"notify release" or "run smoke tests" flow.

```json
{ "id": "smoke_tests", "name": "Smoke Tests", "executor": "flow",
  "with": { "flow_id": "run_smoke_tests", "params": "{\"env\": \"staging\"}", "wait": true } }
```

- `params` is a static JSON string. It can't carry values computed in this run. If the child needs
  this run's values, make it a regular action instead.
- With `wait: true` the action fails when the child errors or is cancelled. Then
  `{{ outputs.status }}` and `{{ outputs.exec_id }}` are available to the next action.
- The child execution is recorded as triggered by the same user who triggered the parent.

## Retries, resumes and idempotency

An action can start more than once: from `max_retries`, from a resume after a server restart, or
from an operator re-running from that action. Every time, it starts from the top.

- `FC_ATTEMPT` is `1` on the first start and goes up each time. Use it to clean up after a partial
  attempt.
- `FC_EXEC_ID` stays the same for the whole execution, including resumes and re-runs. Use it for
  idempotency keys and temporary names.
- Write steps as "make it so": check, then change. `mkdir -p`, `CREATE … IF NOT EXISTS`,
  `kubectl apply`, and skipping work already done.

```sh
if [ "$FC_ATTEMPT" -gt 1 ]; then
  echo "attempt $FC_ATTEMPT: removing partial upload"
  aws s3 rm "s3://$BUCKET/$KEY" || true
fi
```

## Schedules and overlap

```json
"metadata": {
  "name": "Postgres Backup",
  "schedules": [ { "name": "Nightly", "cron": "0 2 * * *", "timezone": "UTC" } ],
  "user_schedulable": true,
  "allow_overlap": false
}
```

- Scheduled and user-scheduled flows need a default for every input, and no file inputs. Dynamic
  defaults (`"{{ now().Format('2006-01-02') }}"`) are evaluated when each run is queued.
- `allow_overlap: false` (the default) refuses a run while another is pending or running. Keep it
  for anything touching shared state; set it `true` only for independent, parallel-safe work.
- An operator can also defer a single manual run (`scheduled_at` on `trigger_flow`).

## Notifications

```json
"notify": [
  { "channel": "email", "config": { "receivers": ["group:reviewers"] }, "events": ["on_waiting"] },
  { "channel": "email", "config": { "receivers": ["group:dba"] }, "events": ["on_failure"] }
]
```

- Use `on_waiting` whenever the flow has an approval, and `on_failure` for anything scheduled
  (nobody is watching a 2 a.m. run).
- Receivers are emails or `group:<name>`; chat and webhook channels take a `url`. Ask the user for
  these.
- Each channel has to be enabled in the server's config. If notifications never arrive, that's the
  first thing to check.
