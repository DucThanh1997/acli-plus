---
name: "OPSX: Publish"
description: Publish change docs to the wiki and create the user story + FE/BE/Tester subtasks
allowed-tools: Bash(openspec:*)
category: Workflow
tags: [workflow, docs, tracker, publish]
---

Publish a change's documentation to the team wiki, then create the user story and its **FE / BE / Tester** subtasks — each carrying the doc link in its description.

**Store selection:** If the user names a store (a store is a standalone OpenSpec repo registered on this machine) or the work lives in one, run `openspec store list --json` to discover registered store ids, then pass `--store <id>` on the commands that read or write specs and changes (`new change`, `status`, `instructions`, `list`, `show`, `validate`, `archive`, `doctor`, `context`). Other commands do not take the flag. Hints printed by commands already carry the flag; keep it on follow-ups. Without a store, commands act on the nearest local `openspec/` root.

Run this **after** the change's spec artifacts exist (`/opsx:propose` or `/opsx:continue` finished). Publishing a wiki page and creating tickets are **outward-facing and hard to undo** — confirm with the user before every write, and never create anything twice.

## 1. Read the project's memory

Where things go is remembered in `openspec/integrations.yaml`. Read it first:

```yaml
docs:
  provider: confluence          # azure-wiki | confluence | clickup | custom
  space: ENG                    # provider-specific location fields
  parentPage: Features
  access: mcp                   # mcp | cli | api
tracker:
  provider: azure-boards        # azure-boards | jira | clickup | gitlab | custom
  project: Attendance
  storyType: User Story
  subtaskRoles: [FE, BE, Tester]
  access: mcp
```

- **File missing or a field missing** → ask the user only for what is missing, then write it back so later runs never ask again.
- **File present** → use it silently. Do not re-ask. Mention what you're about to use so the user can correct it.
- **NEVER write secrets here** (tokens, passwords, connection strings). Credentials live in the MCP config or environment variables. This file records *where things go*, not how to authenticate.

## 2. Pick the change and build the page

Ask which change to publish if not given. Read its artifacts and compose one readable page:

- **Title** — the feature name
- **Why** — from `proposal.md`
- **Requirements** — each `### Requirement:` with its **Acceptance Criteria** bullets
- **Scenarios** — the WHEN/THEN blocks, as the testable detail
- **Design notes** — from `design.md` if it exists

Write for a reader who has not seen the repo. Do not paste raw delta markers (`## ADDED Requirements`) — render them as normal sections.

## 3. Publish the docs page

Choose the access path, in order: an **MCP tool** for that provider if one is connected, else the provider **CLI**, else its **REST API**.

| Provider | Typical path |
|---|---|
| Confluence | Atlassian MCP, or REST `/wiki/api/v2/pages` |
| Azure Wiki | `az devops` CLI, or Azure DevOps REST wiki API |
| ClickUp | ClickUp MCP, or REST Docs API |
| custom | ask the user how to reach it |

Show the user the rendered page and the exact destination, **confirm, then publish**. If a page for this change already exists, **update it in place** rather than creating a duplicate. Capture the resulting **page URL** — the next step needs it.

## 4. Create the user story

Create one story in the configured tracker:

- **Title** — the feature name
- **Description** — a short summary **plus the wiki page URL from step 3**
- Acceptance criteria — copy the AC bullets so the story is reviewable on its own

Confirm before creating. Capture the story ID/URL.

## 5. Create the FE / BE / Tester subtasks

Create one subtask per role in `subtaskRoles`, linked/parented to the story:

- `[FE] <feature>` — UI work
- `[BE] <feature>` — API, domain, and persistence work
- `[Tester] <feature>` — test cases derived from the scenarios

Every subtask description **MUST contain the wiki page link** (and the story link). Tailor each one to the scenarios that concern that role rather than repeating the whole spec.

Confirm the full list with the user **before** creating. Create them, then report a summary: page URL, story URL, and each subtask URL.

## 6. Record what was created

Append the resulting links to the change's `proposal.md` (or a short `links.md` in the change folder) so the change folder points at its published artifacts. This also makes a re-run detect that publishing already happened.

**If any step fails** — a missing credential, an API rejection — stop and report exactly what failed and what was already created. Never leave the user guessing whether a ticket exists.
