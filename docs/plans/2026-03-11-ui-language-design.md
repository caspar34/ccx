# UI Language Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add English, Indonesian, and Chinese UI support with a default language controlled by the `APP_UI_LANGUAGE` environment variable.

**Architecture:** The Go backend exposes the runtime UI language from env when serving the embedded frontend so deployments can switch language without changing source code. The Vue frontend uses a lightweight internal i18n layer plus a persisted locale preference that falls back to the runtime env value.

**Tech Stack:** Go, Gin, Vue 3, Pinia, TypeScript, Vitest

---

### Task 1: Add failing locale tests

**Files:**
- Create: `frontend/src/i18n/index.test.ts`
- Create: `backend-go/internal/config/env_ui_language_test.go`

**Step 1: Write the failing frontend locale test**

Cover:
- invalid locale falls back to `en`
- `zh` normalizes to `zh-CN`
- missing translation falls back to the English message

**Step 2: Run test to verify it fails**

Run: `cd frontend && npx vitest run src/i18n/index.test.ts`

**Step 3: Write the failing backend env test**

Cover:
- `APP_UI_LANGUAGE=id` is accepted
- `APP_UI_LANGUAGE=zh` normalizes to `zh-CN`
- invalid values fall back to `en`

**Step 4: Run test to verify it fails**

Run: `cd backend-go && go test ./internal/config -run TestNormalizeUILanguage -v`

### Task 2: Implement runtime env support

**Files:**
- Modify: `backend-go/internal/config/env.go`
- Modify: `backend-go/internal/handlers/frontend.go`
- Modify: `backend-go/main.go`

**Step 1: Add `UILanguage` to `EnvConfig`**

Read `APP_UI_LANGUAGE`, normalize to `en`, `id`, or `zh-CN`.

**Step 2: Inject runtime config when serving `index.html`**

Insert a small bootstrap script before `</head>`:
- `window.__CCX_RUNTIME_CONFIG__ = { uiLanguage: "..." }`

**Step 3: Pass env config into frontend serving**

Update `ServeFrontend` to receive `envCfg`.

### Task 3: Implement frontend i18n foundation

**Files:**
- Create: `frontend/src/i18n/index.ts`
- Create: `frontend/src/i18n/messages.ts`
- Modify: `frontend/src/env.d.ts`
- Modify: `frontend/src/stores/preferences.ts`
- Modify: `frontend/src/stores/index.ts`
- Modify: `frontend/src/main.ts`

**Step 1: Add locale helpers**

Implement:
- locale normalization
- runtime env lookup
- message lookup with English fallback

**Step 2: Extend preferences store**

Persist `uiLanguage` and add setters.

**Step 3: Initialize locale on app startup**

Use persisted preference if present, otherwise runtime env.

### Task 4: Translate core UI

**Files:**
- Modify: `frontend/src/App.vue`
- Modify: `frontend/src/views/ChannelsView.vue`
- Modify: `frontend/src/components/CapabilityTestDialog.vue`
- Modify: `frontend/src/components/ChannelStatusBadge.vue`

**Step 1: Replace visible hardcoded UI strings with translation keys**

Prioritize:
- auth flow
- header actions
- stat cards
- empty state
- capability test dialog
- status badge labels/tooltips

**Step 2: Update document language**

Set `document.documentElement.lang` from active locale.

### Task 5: Update env examples and docs

**Files:**
- Modify: `backend-go/.env.example`
- Modify: `frontend/.env.example`
- Modify: `README.md`

**Step 1: Document `APP_UI_LANGUAGE`**

Explain supported values and fallback behavior.

### Task 6: Verify

**Step 1: Run frontend tests**

Run: `cd frontend && npx vitest run src/i18n/index.test.ts`

**Step 2: Run frontend type-check**

Run: `cd frontend && npm run type-check`

**Step 3: Run backend tests**

Run: `cd backend-go && go test ./internal/config -run TestNormalizeUILanguage -v`
