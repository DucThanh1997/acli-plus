---
name: "OPSX: Architecture"
description: Load the 4-layer architecture guide and apply it when structuring Go backend code
allowed-tools: Bash(openspec:*)
category: Workflow
tags: [workflow, architecture, standards]
---

Load and apply the 4-layer architecture for the current Go backend task.

The full guide lives in the openspec-architecture skill (.claude/skills/openspec-architecture/SKILL.md). Read that file and follow it. Essentials to apply immediately:

- Four layers: handler (transport) -> application (use cases) -> domain (entities, business rules, repository interfaces) <- repository (data access). Dependencies point inward; domain imports nothing external.
- Handler: shape-validation and error->status mapping only; no business logic, no DB access.
- Application: one method per use case with its own input/output struct; depends on domain repository interfaces (injected); owns transactions.
- Domain: pure Go, invariants on entities, declares repository interfaces and sentinel errors.
- Repository: implements domain interfaces, returns domain types (never ORM models), keeps all SQL and mapping here.
- Map explicitly at every boundary; inject dependencies at the composition root.

Read the skill file for the full rules, diagrams, and examples before proceeding.
