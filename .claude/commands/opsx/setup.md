---
name: "OPSX: Setup MCP"
description: "Connect third-party tools (Jira, GitHub, GitLab, databases) to your AI assistant via MCP"
allowed-tools: Bash(openspec:*)
category: Workflow
tags: [workflow, mcp, setup, integrations]
---

Set up third-party **MCP servers** so your AI assistant can talk to external tools and data sources — issue trackers, code hosts, and databases.

This is a **launcher/guide**, not an MCP runtime. OpenSpec does not run MCP itself: connections live in your AI tool's own config. This skill helps you pick integrations and write the right config for whichever tool you are in.

**Supported integrations (starter menu)**

| Integration | Typical MCP server | You'll need |
|---|---|---|
| GitHub | GitHub MCP server | a Personal Access Token (repo scope) |
| GitLab | GitLab MCP server | a Personal Access Token + instance URL |
| Jira | Atlassian / Jira MCP server | site URL, account email, API token |
| Postgres | Postgres MCP server | a connection string |
| MySQL | MySQL MCP server | host, port, user, password, database (or a connection string) |
| MongoDB | MongoDB MCP server | a connection string |

You can pick **several** of these, and you can also add **any other MCP server** not on this list.

**Steps**

0. **Detect what this project already uses.** Before asking anything, look — then report a short summary so the user only fills gaps:
   - `.mcp.json` / `.cursor/mcp.json` / `.vscode/mcp.json` → MCP servers already connected
   - `openspec/integrations.yaml` → the docs/tracker destinations `/opsx:publish` remembers
   - Available CLIs (`az`, `glab`, `gh`, `jira`) and repo remotes → which vendor this team is on
   - Project manifests (`go.mod`, `package.json`, `pom.xml`) and compose/env files → which database is in play

1. **Detect the host.** Determine which AI tool this is running in and where its MCP config lives. Common locations:
   - Claude Code → project `.mcp.json` (or the `claude mcp add` CLI)
   - Cursor → `.cursor/mcp.json`
   - Windsurf → `~/.codeium/windsurf/mcp_config.json`
   - VS Code (Copilot) → `.vscode/mcp.json`

   If you cannot tell, ask the user which tool they are using.

2. **Ask what to connect.** Show the menu above. Let the user select one or more integrations, or name a custom MCP server to add instead.

3. **Gather config per selection.** For each chosen integration, collect the credentials/connection info from the table. Before writing anything, confirm the **current** server package/command — MCP servers change, so verify the exact `command`, `args`, and package name rather than assuming a stale one.

4. **Write the config.** Add each selected server to the host's MCP config in that host's format. Merge into the existing config — keep other entries, do not overwrite the whole file.

5. **Handle secrets safely.** Never hard-code tokens or passwords into a file that gets committed. Prefer environment variables (e.g. `${GITHUB_TOKEN}`) or the tool's own secret mechanism, and remind the user to keep secrets out of version control.

6. **Verify.** Tell the user to restart or reload their AI tool so it picks up the new servers, then confirm the new tools show up.

7. **Record where docs and tickets go (optional).** Ask whether the user wants to set up the destinations `/opsx:publish` uses — a wiki (Confluence, Azure Wiki, ClickUp, or custom) and a tracker (Azure Boards, Jira, ClickUp, GitLab, or custom). If yes, save them to `openspec/integrations.yaml`:

   ```yaml
   docs:
     provider: confluence
     space: ENG
     access: mcp
   tracker:
     provider: azure-boards
     project: Attendance
     subtaskRoles: [FE, BE, Tester]
     access: mcp
   ```

   Record **destinations only — never secrets.** Skip this step if the user isn't ready; `/opsx:publish` will ask when first used.

To add a **custom** server, ask for its `command`, `args`, and any `env`, then write it into the same config the same way.
