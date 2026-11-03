# Flow definition reference

The JSON that `create_flow` takes as `definition`, the flatter shape `update_flow` takes, and every
rule the server enforces. A definition that breaks one of these rules is rejected with a message
naming the field.

## Contents

- [Shape for create_flow](#shape-for-create_flow)
- [metadata](#metadata)
- [inputs](#inputs)
- [actions](#actions)
- [notify](#notify)
- [Shape for update_flow](#shape-for-update_flow)
- [Pre-flight checklist](#pre-flight-checklist)

## Shape for create_flow

```json
{
  "metadata": { "name": "Postgres Backup", "description": "…" },
  "inputs": [],
  "actions": [
    { "id": "dump_database", "name": "Dump Database", "executor": "docker", "with": { "image": "postgres:16-alpine", "script": "…" } }
  ],
  "notify": []
}
```

`metadata`, `inputs` and `actions` are required. Send `"inputs": []` when the flow takes no input.
Unknown keys are rejected, so use only the keys listed below.

## metadata

| Key | Type | Rules and meaning |
| --- | --- | --- |
| `name` | string | Required. Letters, digits and spaces only (no `-`, `_`, `.`), 1–150 chars. The flow's slug (its `flow_id`) is the name lowercased with spaces turned into `_`, and it can't change later. |
| `description` | string | Up to 255 chars, no `<` or `>`. |
| `prefix` | string | Optional flow group for access control. Letters, digits, `_`; up to 100 chars; not `_` on its own. Users granted the group can see and run the flow. |
| `execution_mode` | string | `sequential` (default) or `dag`. |
| `allow_overlap` | bool | Default `false`: a new run is refused while one is pending or running. Leave it `false` for anything that changes shared state. |
| `user_schedulable` | bool | Lets users add their own schedules from the UI. Every input then needs a default, and file inputs aren't allowed. |
| `max_retries` | int | 0–10. Times flowctl automatically retries a failed execution. Completed actions keep their results and the failed one starts again (`$FC_ATTEMPT` goes up). |
| `schedules` | array | `[{"name": "Nightly", "cron": "0 2 * * *", "timezone": "UTC"}]`. Standard 5-field cron and an IANA timezone; `name` (up to 150 chars) is optional. Every input needs a default; no file inputs. |
| `id`, `version`, `namespace` | | Ignored on create. The slug comes from `name` and the namespace from the tool argument. |

## inputs

| Key | Type | Rules and meaning |
| --- | --- | --- |
| `name` | string | Required. Letters, digits, `_`. The name used in `{{ inputs.<name> }}` and in `validation`. |
| `type` | string | Required. `string`, `number`, `password`, `file`, `datetime`, `checkbox`, `select`, `node`. |
| `label` | string | Shown in the trigger form. Up to 255 chars. |
| `description` | string | Help text. Up to 255 chars. |
| `required` | bool | The run is refused without it. |
| `default` | string | **Always a string**: `"4"`, `"true"`, `"orders"`. A checkbox default is `"true"` or `"false"`; a select default must be one of `options`; a number default must parse as a number; file inputs can't have one. Can be an expression (see below). |
| `options` | array of strings | Choices for `select`. |
| `validation` | string | An expr expression over this input only, by its own name, returning a boolean: `"port >= 1024 && port <= 65535"`, `"len(name) >= 3"`, `"tag matches '^v[0-9]+[.][0-9]+[.][0-9]+$'"`. |
| `max_file_size` | int | Bytes, for `file` inputs. |
| `remote_options` | object | For `select`: `{"url": "https://…", "method": "GET", "headers": {"Authorization": "Bearer {{ secrets.TOKEN }}"}}`. The URL must be http or https and return `{"options": [...]}`. |

- At most one `node` input per flow.
- **Dynamic defaults:** a default that is exactly one `{{ }}` expression is evaluated each time the
  trigger form opens or a scheduled run is queued, for example `"{{ now().Format('2006-01-02') }}"`.
  It can't read other inputs or secrets; `now()`, `date()`, `duration()` and `timezone()` are the
  useful builtins. A default containing `{{` that isn't a closed expression is rejected.
- File inputs can't be supplied over MCP, so a flow with a required file input can't be test-run by
  an agent. Prefer a `string` input holding a URL or path when the use case allows it.

## actions

| Key | Type | Rules and meaning |
| --- | --- | --- |
| `name` | string | Required. Letters, digits and spaces only, 1–150 chars. **The action's id is derived from it.** |
| `id` | string | Ignored on create: the server sets it to the name lowercased with runs of spaces turned into `_`. Send that same value, so `needs` and `outputs.global.<id>` read correctly. Ids must be unique, so names must be too. |
| `executor` | string | `docker`, `script`, `flow`, or a plugin name from `list_executors`. |
| `with` | object | Required, non-empty. Executor config, passed verbatim (never templated). See below, and `get_executor_schema` for anything else. |
| `variables` | array | `[{"NAME": "value or {{ expression }}"}]`: one key per object, the value always a string. Each becomes an environment variable called `NAME` in the script. |
| `on` | array of strings | Remote nodes to run on, by name or `tag:<tag>`. Empty means the flowctl host. With several targets the action runs on all of them in parallel and fails if any fails. |
| `allow_node_override` | bool | When the flow has a `node` input and the operator fills it, this action runs on the operator's selection instead of `on`. Has no effect on `flow` actions. |
| `needs` | array of strings | Action ids this one waits for. Only allowed with `execution_mode: dag`. No missing ids, self-references or cycles. |
| `approval` | bool | Pause before this action until a Reviewer or Admin approves. Rejecting cancels the run. |
| `condition` | string | Accepted but **ignored**: the server drops it. Don't rely on it; branch inside the script instead. |

### `with` for the built-in executors

**docker**: runs the script in a fresh container on the target host.

| Key | Meaning |
| --- | --- |
| `image` | Image reference, e.g. `postgres:16-alpine`. Pin a tag. |
| `script` | The script. |
| `interpreter` | Command that runs the script. Default `/bin/sh`. Set `/bin/bash` only if the image has bash. Use `python3` (with `extension: .py`) for Python. |
| `extension` | Script file extension. Default `.sh`. |

The container gets the variables as environment variables, plus `FC_OUTPUT`, `FC_OUTPUT_GLOBAL` and
`FC_ARTIFACTS`. The image's own `ENTRYPOINT` is kept and receives `<interpreter> <script>` as
arguments, so images whose entrypoint is a tool (aws-cli, terraform, kubectl images) won't run the
script. Start from a shell image and install the tool, or use an image with no entrypoint.

**script**: runs the script directly on the target host, as the flowctl process user locally or
the node's SSH user remotely.

| Key | Meaning |
| --- | --- |
| `script` | The script. |
| `interpreter` | Default `/bin/bash`. |
| `extension` | Default `.sh`. |

Prefer `docker` for work on the flowctl host itself; `script` there runs with the server's own
permissions.

**flow**: triggers another flow in the same namespace as a child execution. It runs on the
flowctl host only.

| Key | Meaning |
| --- | --- |
| `flow_id` | Required. The child flow's slug. |
| `params` | A JSON object **as a string**, e.g. `"{\"env\": \"staging\", \"dry_run\": \"true\"}"`. It's static: it can't use `{{ }}`, so the child can't receive values computed in this run. Values are sent as the child's inputs. |
| `wait` | `true` to wait for the child to finish; the action fails if the child errors or is cancelled. |

The action outputs `exec_id`, and `status` when `wait` is true: `{{ outputs.exec_id }}`,
`{{ outputs.status }}`. A later `flow` action overwrites them, so read them right after.

## notify

```json
"notify": [
  { "channel": "email", "config": { "receivers": ["group:dba", "oncall@example.com"] }, "events": ["on_failure", "on_waiting"] },
  { "channel": "chat", "config": { "url": "https://chat.example.com/hooks/xxxx" }, "events": ["on_failure"] },
  { "channel": "webhook", "config": { "url": "https://example.com/hooks/flowctl" }, "events": ["on_success", "on_failure"] }
]
```

- `channel` is `email`, `chat` (Slack or Mattermost incoming webhook) or `webhook` (Standard Webhooks,
  signed). Each needs that messenger enabled in the server's config.
- `events`: `on_success`, `on_failure`, `on_waiting` (an approval is pending), `on_cancelled`.
- Email receivers are addresses or `group:<flowctl group>`.
- A chat or webhook URL is stored in the definition, which anyone who can view the flow's config can
  read. Ask the user for the URL; don't invent one.

## Shape for update_flow

`update_flow` takes a flat object. Start from `get_flow_definition`, then:

- keep `inputs`, `actions` and `notify` as they are,
- move `prefix`, `description`, `schedules`, `allow_overlap`, `user_schedulable`, `max_retries` and
  `execution_mode` out of `metadata` to the top level,
- drop `metadata` (`name`, `id` and `namespace` can't change).

```json
{
  "description": "…", "prefix": "", "execution_mode": "dag", "allow_overlap": false,
  "user_schedulable": true, "max_retries": 1, "schedules": [],
  "inputs": [], "actions": [], "notify": []
}
```

`update_flow` replaces the whole definition. Anything you leave out is removed.

## Pre-flight checklist

Run through this before `create_flow`:

- [ ] `metadata.name` and every action `name` use only letters, digits and spaces; action names are unique.
- [ ] Every `id`, `needs` entry and `outputs.global.<id>` uses the derived id (lowercase, `_` for spaces).
- [ ] `needs` appears only with `execution_mode: dag`, and the graph has no cycles.
- [ ] Every input default is a string of the right kind; select defaults are among `options`.
- [ ] Scheduled or `user_schedulable` flows: every input has a default, none is `file`.
- [ ] Every variable is a one-key object with a string value holding at most one `{{ }}`.
- [ ] Every value a script uses arrives through `variables` or a `FC_*` variable, never a `{{ }}` in `with`.
- [ ] Every `{{ outputs.X }}` is written by an earlier action (and in DAG mode, one listed in `needs`).
- [ ] No credential appears anywhere; each is a `{{ secrets.NAME }}` the user will add.
- [ ] Destructive actions carry `approval: true`, and read-only checks come before them.
- [ ] Docker scripts are POSIX sh, or set an `interpreter` the image has.
