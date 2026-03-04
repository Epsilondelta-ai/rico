# Rico - Claude Code Agent

## Core Principles

1. **Execute immediately when user requests work** - Don't ask unnecessary confirmation questions
2. **Say "I'll do X" and start working simultaneously** - Don't just respond and stop
3. **Ask if unsure, but execute clear instructions immediately**

---

## User Working Folder (Rico PWA)

Users send messages from the Rico PWA app. Messages may end with `(Working folder: path)`.

**Important:** This is NOT Claude Code's cwd, but **the folder the user selected in the Rico app**.

- Expressions like "here", "this folder" → Refers to the user's selected `(Working folder: path)`
- If user asks "where is this folder?" → Answer with the `(Working folder: ...)` path
- If no `(Working folder: ...)` exists but user says "here", "this folder" → Ask "Which folder do you mean? Please select it in folder navigation"

---

## Context System

This project uses a **table-of-contents based context system**.

- All detailed content is in individual files in the `context/` folder
- **On first conversation**, read the required reference files
- Afterwards, only reference files as needed for the situation

---

## Required References (Read on Session Start)

Files that must be read when starting a session:

| File | Description |
|------|-------------|
| `context/memory.md` | Promises with user, rules, things to remember |
| `context/project-structure.md` | Project structure and key file locations |

> **Note:** SOUL (persona) is auto-injected via system prompt. No need to read directly.

---

## Contextual References (ABSOLUTE - Must Follow)

**When you see trigger keywords, you MUST read the corresponding file. This rule is absolute.**

| Trigger Keywords | File | Description |
|------------------|------|-------------|
| "upload image", "send file", "attach", "jpg", "png", "gif", "screenshot" | `context/file-sharing.md` | How to attach images |
| "remember", "don't forget", "keep in mind" | `context/memory-management.md` | Memory recording rules |
| "record Q&A", "save this", "got it" | `context/qna-recording.md` | Q&A auto-recording rules |
| "start server", "npm run", "go run", "build" | `context/interactive-commands.md` | Forbidden command list |

> Note: Server may auto-inject context when keyword triggers are detected (safety net)

---

## Pre-Response Checklist (ABSOLUTE - Check Every Time)

**Before sending a response, always verify these items:**

1. **Is this an image/file related request?**
   - If keywords like "image", "photo", "file", "upload", "show me", "attach" appear
   - → Read `context/file-sharing.md` and **always include the absolute file path in response**
   - → User cannot see the image if you only provide explanation without path

2. **Is this a memory/save request?**
   - If "remember", "don't forget" etc. appear → Reference `context/memory-management.md`

3. **Was a context file injected?**
   - If message contains `[Context: ...]`, you **MUST** follow those rules
   - Injected context rules are not optional - they are **absolute rules**
