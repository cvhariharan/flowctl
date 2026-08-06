---
title: Flows
description: Learn how to create and configure workflows in flowctl
---

## What are Flows?

Flows are the core automation units in flowctl. A flow is a sequence of actions that execute in order, with support for inputs, variables, approvals. Flows are defined using YAML/[HUML](https://huml.io) files and can run locally or on remote nodes.

## Flow Structure

Every flow consists of five main sections:

1. **Metadata** - Flow identification and configuration
2. **[Inputs](/docs/general/flow-inputs)** - Parameters that users provide when triggering the flow
3. **[Actions](/docs/general/flow-actions)** - The actual tasks to execute
4. **Schedules** - List of cron schedules
5. **[Notifications](/docs/general/flow-notifications)** - Event-based notifications to users or groups

## Basic Flow Example

Here's a simple flow that greets a user:

```yaml
metadata:
  id: hello_world
  name: Hello World
  description: A simple greeting flow
  allow_overlap: true

inputs:
  - name: username
    type: string
    label: Username
    description: Your name
    required: true
    validation: len(username) > 0

actions:
  - id: greet
    name: Greet User
    executor: docker
    variables:
      - username: "{{ inputs.username }}"
    with:
      image: docker.io/alpine
      script: |
        echo "Hello, $username!"
        echo "message=Welcome!" >> $FC_OUTPUT
```

## Metadata

The metadata section defines the flow's identity and behavior:

```yaml
metadata:
  id: my_flow # Unique identifier (alphanumeric + underscore)
  name: My Flow # Human-readable name
  description: Flow description
  namespace: default # Namespace for organization
  allow_overlap: true # Allow executions to overlap
```

### Execution Overlap

If `allow_overlap` is set to true in a flow, executions for that flow can overlap. This is `false` by default which prevents executions from running if there is already an execution in running / pending state.

### Scheduling Flows

Flows can be scheduled using cron expressions.

```yaml
inputs:
  - name: environment
    type: string
    default: "production" # Required for scheduled flows

schedules:
  - cron: "0 2 * * *" # Run daily at 2 AM
    timezone: Asia/Kolkata
```

!!! note
      Only flows where all inputs have default values can be scheduled. Flows with
      file inputs cannot be scheduled.

### User Schedules

You can also create your own schedules from the UI, separate from the ones defined in the flow YAML. These let you run the same flow on different schedules with different input values.

To enable this, add `user_schedulable: true` to your flow metadata:

```yaml
metadata:
  id: my_flow
  name: My Flow
  user_schedulable: true
```

![User Schedules](../assets/images/schedules.png)

Once enabled, go to a flow's **Schedule** tab and click **Add** to create a schedule.

![Schedule Inputs](../assets/images/schedule-inputs.png)

!!! note
      Like flow-defined schedules, user schedules require all inputs to have default
      values. File inputs aren't supported.

### Scheduling a Flow for Later

When triggering a flow manually you can defer execution by enabling the **Run Later** toggle. Choose a date, time, and timezone. The flow is queued and starts at the specified time.

The timezone selector defaults to your browser's local timezone. You can search for any IANA timezone (e.g. `America/New_York`, `Europe/Berlin`).

## Log Download

Execution logs can be downloaded as a raw `.log` file once an execution has completed, errored, or been cancelled. The download button is available on the execution detail page.

## Duplicating a Flow

To create a copy of an existing flow, open the flow list, click the **...** menu on any flow, and select **Duplicate**. The create form opens pre-filled with the original flow's metadata, inputs, actions, and notifications.

!!! warning
      Secrets are not copied. Re-add any secrets under the **Secrets** tab after the
      duplicated flow is created.

## Next Steps

- Define [Inputs](/docs/general/flow-inputs)
- Configure [Actions](/docs/general/flow-actions)
- Set up [Notifications](/docs/general/flow-notifications)
- Configure [Remote Nodes](/docs/general/nodes-and-executors#remote-nodes)
- Learn about [Executors](/docs/general/nodes-and-executors)
- Explore [Access Control](/docs/general/access-control)
