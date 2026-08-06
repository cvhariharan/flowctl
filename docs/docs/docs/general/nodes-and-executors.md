---
title: Nodes and Executors
description: Learn about remote nodes and execution environments in flowctl
---

## Overview

Flowctl allows you to execute workflows both locally and on remote nodes.

## Executors

Executors define how your actions run. Flowctl provides three built-in executors:

### Docker Executor

The Docker executor runs your scripts inside Docker containers.

**Configuration:**

```yaml
- id: build_app
  name: Build Application
  executor: docker
  variables:
    - version: "{{ inputs.version }}"
  with:
    image: docker.io/node:18
    script: |
      npm install
      npm run build
      echo "BUILD_ID=$(date +%s)" >> $FC_OUTPUT
```

**Config:**

- **`image`**: Docker image
- **`script`**: Bash script to execute inside the container

### Script Executor

The Script executor runs shell scripts directly on the host system (local or remote).

**Configuration:**

```yaml
- id: deploy
  name: Deploy Application
  executor: script
  variables:
    - app_name: "{{ inputs.app_name }}"
  with:
    script: |
      cd /opt/$app_name
      git pull origin main
      systemctl restart $app_name
      echo "DEPLOYED_AT=$(date -Iseconds)" >> $FC_OUTPUT
    interpreter: /bin/bash # Optional, defaults to /bin/bash
```

**Key Features:**

- **`script`**: Script to execute. Could be anything the interpreter can execute.
- **`interpreter`**: Path to interpreter

!!! warning
      Script executor actions run with the permissions of the flowctl process on
      local nodes and the user configured on remote nodes. Ensure proper security
      measures are in place.

### Flow Executor

The Flow executor triggers another flow as a child execution. Use it to compose workflows.

**Configuration:**

```yaml
- id: run_tests
  name: Run Test Suite
  executor: flow
  with:
    flow_id: my-test-flow
    params: '{"env": "staging"}'
    wait: true
```

**Config:**

- **`flow_id`** (required): ID of the flow to trigger
- **`params`**: JSON string of parameters to pass to the child flow
- **`wait`**: If `true`, blocks until the child flow completes. Defaults to `false`

**Outputs:**

- **`exec_id`**: Execution ID of the triggered flow
- **`status`**: Final status of the child flow (only when `wait: true`). One of `completed`, `errored`, or `cancelled`

!!! note
      The flow executor runs on the local node only and cannot be used with remote nodes.

## Remote Nodes

A remote node is any server or machine that flowctl can connect to via a remote client (SSH).

### Setting Up Remote Nodes

![Node Management](../assets/images/nodes.png)

#### Step 1: Create a Credential

![Create Credential](../assets/images/create-credential.png)

Before adding a node, create SSH credentials:

1. Navigate to **Credentials**
2. Click **Add Credential**
3. Provide:
   - **Name**: Descriptive name (e.g., "Production Server SSH Key")
   - **Type**: `private_key` or `password`
   - **Key Data**: SSH private key or password

![List Credential](../assets/images/credentials-list.png)

#### Step 2: Add a Node

1. Navigate to **Nodes**
2. Click **Add Node**
3. Configure the node:

![Add Node](../assets/images/add-node.png)

**Fields:**

- **Name**: Unique identifier used in flow definitions
- **Hostname**: IP address or domain name
- **Port**: SSH port (default: 22)
- **Username**: SSH username
- **Connection Type**: `ssh` or `qssh` (QUIC-based SSH)
- **Credential**: SSH authentication credential
- **Tags**: Optional labels for organization

### Using Remote Nodes in Flows

Execute actions on remote nodes using the `on` field. You can specify node names directly or use tags to target multiple nodes.

#### By Node Name

```yaml
actions:
  - id: remote_deploy
    name: Deploy to Production
    executor: script
    on:
      - WebServer1
      - WebServer2
```

#### By Tag

Use the `tag:` prefix to run on all nodes with a specific tag:

```yaml
actions:
  - id: deploy_to_dev
    name: Deploy to Dev Servers
    executor: script
    on:
      - tag:dev
```

You can mix node names and tags:

```yaml
on:
  - WebServer1
  - tag:backend
```

![Node Selector](../assets/images/node-selector.png)

In the UI, type `tag:` followed by the tag name to search by tag. You can either select the tag itself (targets all nodes with that tag) or pick individual nodes from the results.

**Key Points:**

- Actions run on **all specified nodes in parallel**
- Each node receives the same inputs and variables
- Outputs are collected from all nodes
- If any node action fails, the entire flow will fail
- When using tags, nodes are resolved at execution time. Add/remove nodes from a tag without updating flows

## Next Steps

- Learn about [Flow Secrets](/docs/general/flow-actions#flow-secrets) for secure credential management
- Review [Access Control](/docs/general/access-control) for multi-user environments
