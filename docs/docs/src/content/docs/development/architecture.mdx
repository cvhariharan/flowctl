---
title: Architecture
description: Technical architecture and internal package dependencies
---

## Overview

flowctl is a Go-based workflow execution platform using a **modular architecture** with separate SDKs for executors and remote clients. All modules are compiled into the binary.

## Core Components

### Application Layer

- **cmd/**: CLI entry points (start, migrate)
- **internal/core**: Business logic (flows, credentials, approvals, RBAC)
- **internal/handlers**: REST API endpoints
- **internal/scheduler**: Task queue and worker pool
- **internal/repo**: PostgreSQL data access layer
- **internal/streamlogger**: Real-time log streaming

### Module SDKs

- **sdk/executor**: Executor interface and NodeDriver abstraction
- **sdk/remoteclient**: Remote connection protocol interface

### Executor Implementations

- **executors/docker**: Docker container executor
- **executors/script**: Shell script executor

### Remote Client Implementations

- **remoteclients/ssh**: Standard SSH connections
- **remoteclients/qssh**: QUIC-based SSH alternative

### Frontend

- **site/**: SvelteKit UI (TypeScript/Tailwind)

## Internal Package Dependencies

```mermaid
graph TD
    CMD[cmd/start] --> CORE[internal/core]
    CMD --> HANDLERS[internal/handlers]
    CMD --> SCHED[internal/scheduler]
    CMD --> EXEC_REG[Executor Registry]
    CMD --> RC_REG[RemoteClient Registry]

    HANDLERS --> CORE

    CORE --> REPO[internal/repo]
    CORE --> SCHED
    CORE --> LOGGER[internal/streamlogger]

    SCHED --> REPO
    SCHED --> LOGGER

    REPO --> DB[(PostgreSQL)]

    EXEC_REG --> DOCKER[executors/docker]
    EXEC_REG --> SCRIPT[executors/script]

    DOCKER --> SDK_EXEC[sdk/executor]
    SCRIPT --> SDK_EXEC

    SDK_EXEC --> NODE_DRIVER[NodeDriver]

    NODE_DRIVER --> SDK_RC[sdk/remoteclient]

    RC_REG --> SSH[remoteclients/ssh]
    RC_REG --> QSSH[remoteclients/qssh]

    SSH --> SDK_RC
    QSSH --> SDK_RC

    style CMD fill:#e1f5ff
    style CORE fill:#fff4e1
    style SDK_EXEC fill:#f0e1ff
    style SDK_RC fill:#f0e1ff
    style DB fill:#e8f5e9
```
