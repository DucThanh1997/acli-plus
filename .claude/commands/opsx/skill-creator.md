---
name: "OPSX: Skill Creator"
description: Create or improve a skill using the official skill-creator skill
allowed-tools: Bash(openspec:*)
category: Workflow
tags: [workflow, skills, meta]
---

Create or improve a skill using the official **skill-creator** skill.

This is a launcher - the real work is done by the skill-creator plugin skill: draft a new skill, edit or optimize an existing one, run evals, benchmark performance with variance analysis, and optimize the skill description for better triggering.

**Steps**

1. **Make sure the skill-creator skill is available.** It ships as the `skill-creator@claude-plugins-official` plugin. If it is enabled in this session, use it. If it is NOT available, tell the user to enable it: add `"skill-creator@claude-plugins-official": true` under `enabledPlugins` in `~/.claude/settings.json`, restart Claude Code, then run `/opsx:skill-creator` again.

2. **Ask what the user wants:** create a new skill from scratch, or edit / improve / evaluate an existing skill.

3. **Follow the skill-creator process:** draft the skill, write a few realistic test prompts, run them (with and without the skill), review the results, iterate on the skill, and optionally optimize its description for triggering.

Defer to the skill-creator skill for all the details (its eval-viewer, benchmark, and description-optimization scripts). Do not reimplement that tooling here.
