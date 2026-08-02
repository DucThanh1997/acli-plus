---
name: "OPSX: Go Skills"
description: Load the Go coding standards and apply them to the current Go backend task
allowed-tools: Bash(openspec:*)
category: Workflow
tags: [workflow, go, standards]
---

Load and apply the Go coding standards for the current Go backend task.

The complete standards live in the openspec-go-skills skill (.claude/skills/openspec-go-skills/SKILL.md). Read that file and treat every rule as binding. Apply these essentials immediately:

- No assumptions: when the schema, scope, naming, or a business rule is unclear, STOP and ask — never guess. List all callers before any change that could break them.
- Plan before code: research first, then present a plan (current vs proposed flow, exact files to change, impact with grep evidence, risks, ordered steps) and get approval before editing.
- Decompose God functions; keep repositories DRY; mind query performance (avoid full-table-scan subqueries; Postgres does NOT auto-index FK columns).
- Run tests after every change; regenerate mocks after interface changes; back every claim with evidence (grep/output); explain things newcomer-friendly.

Read the skill file for the full rules, examples, and diagrams before proceeding.
