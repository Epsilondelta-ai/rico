# Changelog

## 2026-03-03 - Internationalization Cleanup

Major refactoring for open source release.

### Added

- **speak tag system**: Language enforcement via `[speak: X]` tag
  - Frontend adds tag based on i18n locale
  - Backend removes tag when saving to chat history
  - See `context/i18n.md` for details

- **English README**: `README-en.md` with language toggle links

- **Korean persona**: `context/souls/EB-ko.md` for Korean users who want Korean SOUL

### Changed

- **CLAUDE.md**: Converted to English
- **All context files**: Converted to English
- **SOUL.md & SOUL.default.md**: Converted to English with language directive rule
- **JSON schema**: Simplified to `response + suggestions` only (removed mood)

### Removed

- **mood detection feature**: Removed from settings, backend, and UI
  - `settings.json`: Removed `moodDetection` setting
  - `main.go`: Removed mood-related logic and JSON schema branching
  - `SoulScreen.svelte`: Removed mood toggle UI
  - `locales/*.json`: Removed mood translation keys

- **Documentation folders**: Deleted (not needed for open source)
  - `context/screens/` (13 files)
  - `context/modes/` (3 files including mood-detection)
  - `context/architecture/` (5 files, had personal domain exposure)

- **Personal info files**:
  - `context/environment.md` (local paths)
  - `context/DEV_GUIDE.md` (Tailscale domain)

### File Structure After Cleanup

```
context/
├── SOUL.md              # Current persona (English)
├── SOUL.default.md      # Default persona (English)
├── SOUL_GENERATOR.md    # Persona generation guide
├── souls/
│   ├── soul.default.md  # Backup default
│   └── EB-ko.md         # Korean persona option
├── file-sharing.md
├── i18n.md              # Updated with speak tag docs
├── interactive-commands.md
├── memory.md
├── memory-management.md
├── project-structure.md
├── qna-recording.md
└── CHANGELOG.md         # This file
```

### Key Code Changes

| File | Change |
|------|--------|
| `web/src/lib/i18n.ts` | Added `getCurrentLocale()` function |
| `web/src/App.svelte` | Added speak tag injection in `handleSendMessage()` |
| `server/main.go` | Added `speakTagRegex`, simplified `getSoulPromptFile()`, removed mood logic |
| `settings.json` | Removed `moodDetection` from modes |

### Migration Notes for Other Sessions

1. **SOUL files are now English**: If you see English SOUL content, this is expected
2. **No mood detection**: The feature was removed, don't try to enable it
3. **Language enforcement**: Claude should still respond in user's language due to speak tag
4. **Simplified JSON schema**: Only `response` and `suggestions` fields, no `mood`
