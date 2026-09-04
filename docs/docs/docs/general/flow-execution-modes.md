---
title: Execution Modes
description: Run a flow's actions one after another or as a dependency graph with parallel branches
---

By default a flow runs its actions one after another, in the order they are listed. Set the execution
mode to `dag` and each action declares what it waits for, so independent branches run at the same
time.

```yaml
metadata:
  id: my_flow
  name: My Flow
  execution_mode: dag # sequential (default) or dag
```

## Sequential

`sequential` is the default, used whenever `execution_mode` is not set. Actions run top to bottom,
one at a time, and the flow stops at the first action that fails.

Actions cannot declare dependencies in this mode. The order in the file is the order they run in.

## Dependency Graph

In `dag` mode an action lists the actions it depends on under `needs`. Actions without a `needs`
entry start straight away, and everything else waits for its dependencies to complete.

```yaml
metadata:
  id: deploy_service
  name: Deploy Service
  execution_mode: dag

actions:
  - id: build
    name: Build
    executor: docker
    with:
      image: docker.io/node:18
      script: npm run build

  - id: test_unit
    name: Unit Tests
    executor: docker
    needs: [build]
    with:
      image: docker.io/node:18
      script: npm test

  - id: test_integration
    name: Integration Tests
    executor: docker
    needs: [build]
    with:
      image: docker.io/node:18
      script: npm run test:integration

  - id: deploy
    name: Deploy
    executor: script
    needs: [test_unit, test_integration]
    with:
      script: ./deploy.sh
```

Here `build` runs first, the two test actions run together once it completes, and `deploy` waits for
both of them.

### Parallelism

How many actions a dag flow runs at once is capped by `max_parallel` under `[scheduler]` in
`config.toml`. It applies to every flow on the instance and defaults to the number of CPU cores.

```toml
[scheduler]
  max_parallel = 4
```

When more actions are eligible than the cap allows, they are dispatched in the order they appear in
the flow.

!!! note
      This is separate from the `on` field on an action, which fans a single action out across
      several remote nodes. Both can be used together.

### Outputs Across Branches

An action can read the outputs of any action that had already finished when it was dispatched. Only
rely on the outputs of the actions you have listed in `needs`. Anything else depends on which branch
happened to finish first, which can change from one run to the next.

```yaml
- id: deploy
  name: Deploy
  executor: script
  needs: [build] # build's outputs are guaranteed to be available
  variables:
    - build_id: "{{ outputs.BUILD_ID }}"
  with:
    script: ./deploy.sh $build_id
```

!!! warning
      Actions running in parallel share the same `$FC_ARTIFACTS` directory. Two branches writing
      the same filename will overwrite each other.

### When an Action Fails

A failed action stops the scheduler from dispatching anything new, but actions that are already
running are left to finish, since killing a half-applied deployment is usually worse than letting it
complete. Once they settle, any action that never started is marked **skipped** and the execution is
marked errored.

Cancelling instead interrupts the actions that are running and stops anything new from being
dispatched.

### Approvals in a Graph

An approval-gated action blocks its own branch, not the whole flow, so other branches keep running.
A dag execution can therefore be waiting on more than one approval at a time, and the approvals page
lists each one separately.

Approving an action while other branches are still running is fine; the execution resumes once it
has settled into the waiting state. Rejecting cancels the whole execution.

If one branch has already failed, that failure wins: approving the remaining action will not resume
an execution that is on its way to errored.

## Action States

Both modes track the state of each action. The execution page shows it per action, and the
`action_states` field on the execution API carries the same information.

| Status      | Meaning                                       |
| ----------- | --------------------------------------------- |
| `pending`   | Not dispatched yet                            |
| `running`   | Currently executing                           |
| `completed` | Finished successfully                         |
| `failed`    | Finished with an error                        |
| `blocked`   | Waiting for an approval decision              |
| `skipped`   | Never ran because the execution stopped first |
| `cancelled` | Interrupted by a cancellation                 |


## Next Steps

- Configure [Actions](/docs/general/flow-actions)
- Learn about [Approvals](/docs/general/flow-actions#approvals)
- [Re-run a flow from an action](/docs/general/flows#re-running-from-an-action)
- Back to [Flows overview](/docs/general/flows)
