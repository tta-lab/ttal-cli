---
title: Custom Tag Routing
description: Build custom workflows with tag-based task routing
sidebar:
  order: 2
---

Tags drive how tasks get routed to agents in ttal. You can create custom tag routes to build specialized workflows.

## How tag routing works

When a task is created with tags, ttal's on-add hook checks if any tags match registered agents or predefined routes. Matching tasks get routed to the appropriate agent automatically.

## Built-in tag routes

ttal ships with some built-in tag routes:

- **`+newskill`** — routes to the skill creation workflow
- **`+newagent`** — routes to the agent creator

```bash
# This task gets picked up by the skill creation flow
task add "Create a deployment skill for Kubernetes" +newskill
```

## Creating custom routes

### 1. Register an agent with the relevant tag

```bash
ttal agent add debugger +bugfix +core
```

### 2. Create tasks with that tag

```bash
task add "Fix null pointer in auth handler" +bugfix
```

The task will be routed to `debugger` because they share the `+bugfix` tag.

### 3. Multiple matching agents

When multiple agents match a task's tags, the agent with the most matching tags wins. This lets you create specialized agents that handle narrow subsets of work.

```bash
# General backend agent
ttal agent add kestrel +backend +core

# Specialized database agent
ttal agent add dbadmin +backend +database

# This routes to dbadmin (2 matching tags vs 1)
task add "Optimize slow query in users table" +backend +database
```

## Patterns

### Domain-specific agents

```bash
ttal agent add frontend-dev +frontend +react
ttal agent add api-dev +backend +api
ttal agent add infra-dev +infrastructure +k8s
```

### Priority routing

Use tags to express urgency and route accordingly:

```bash
ttal agent add firefighter +urgent +hotfix
task add "Production auth is down" +urgent +hotfix
```

### Project-scoped routing

Combine project tags with role tags:

```bash
ttal agent add myapp-researcher +myapp +research
task add "Research caching strategies for myapp" +myapp +research
```
