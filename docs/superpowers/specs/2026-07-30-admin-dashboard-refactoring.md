# Admin Dashboard Refactoring Design Specification

> **Date:** 2026-07-30
> **Topic:** Admin Dashboard Refactoring & Migration to Vue 3 / Vite

## 1. Goal & Context
The current administration dashboard is implemented via a single 188KB HTML file paired with a monolithic Go handler (~34KB). This centralized architecture is preventing scalable expansion and the planned integration of AI capabilities (e.g., generating pet infos, items, and pictures). 

The goal of this refactoring is to completely replace the monolithic UI and unorganized APIs with a modern, modular Frontend (Vue 3 + TypeScript + Vite) and routed Backend, while zeroing data loss from the current 13 SQLite tables (including 9 key config tables). 

## 2. Global Architecture & Tech Stack

### Frontend
- **Framework & Tooling**: Vue 3 `<script setup>`, TypeScript, Vite.
- **Build Output**: The Vite build artifact (`dist/`) will continue to be embedded directly into the Go executable via `go:embed`.
- **Theme System**: Deep dark aesthetic with multiple color palettes:
  1. Nebula Purple (`#775cff`)
  2. Noir Vermilion (`#ff513f`)
  3. Pure Monochrome (`#e9e6dd`)
  *Theme variations are saved via browser `localStorage` to ensure persistence across sessions without polluting existing schemas.*

### Backend
- **Framework**: Go standard library / existing Gin configuration.
- **Authentication**: Opaque Server-Side Cookies utilizing the completed Admin Account System (Task 1-4).
- **API Restructuring**: Refactor the massive unified handler into specific routed domains (e.g., `config`, `users`, `assets`).
- **Response Format**: Strict JSON contract `{ "code": int, "msg": string, "data": any }`.

## 3. Scope & Data Strategy

### Database Compatibility Strategy
- **No Data Deletion**: Maintain complete compatibility with the current SQLite structure. Operations will insert/update without removing existing operational or game logic columns.
- **AI Expansion Readiness**: Reserve scalable JSON blob fields or flexible text layouts in relevant asset/item tables so upcoming AI generators can inject content without DB schema changes.

### UI Information Architecture
The new layout abandons the traditional "CMS template view" for a workspace/dashboard paradigm featuring a left sidebar and top context menu.

1. **📊 Operations / Dashboard**
   - High-level data views: active users, total pets, group dynamics.
2. **⚙️ Configuration Center (Core Focus)**
   - CRUD interfaces for the 9 operational tables (Parameters, Levels, Personalities, Items, Dialogs, Quests, Achievements, Environments, Gacha Pools).
   - High-density table views and JSON-preview fallback forms for complex parameters.
3. **🐾 Pets & Users**
   - Direct management planes for player profiles, live pet statuses, group bounds, and backpack items.
4. **📦 Assets & Materials (AI Prep)**
   - Image and item-asset manifest.
   - Placeholder and layout readiness for the future AI content generation workbench.
5. **🛡️ Security & System**
   - Incorporating the existing Change Password and Session Logouts commands.

## 4. Subsystem Designs

### 4.1 Frontend Component Architecture
- **Layout Shell**: `App.vue` providing the Sidebar and Topbar; rendering a `<router-view>`.
- **Router**: `vue-router` mappings for `Dashboard`, `ConfigManager`, `EntityTracker`, and `Settings`.
- **Styling**: Scoped CSS or Tailwind-flavor utility classes, enforcing the dark-mode palette variables (defined in a `:root` / `html` CSS scope logic controlled by a global store/hook).

### 4.2 Backend API Segregation
The current handler will be deprecated incrementally:
- `GET/POST/PUT/DELETE /api/admin/config/:schema`
- `GET/POST/PUT/DELETE /api/admin/pets/:id`
- `GET/POST/PUT/DELETE /api/admin/assets/:id`
All wrapped in the standard `RequireAdminSession` middleware built previously.

## 5. Security & Error Handling
- Only authenticated requests (via Admin Cookie) are allowed in the `admin/` API space.
- The UI handles `401 Unauthorized` globally, purging localStorage auth states and forcing navigation to `/login`.
- Form inputs validate against constraints set by Go APIs; responses display intuitive toast notifications matching the new branding palette.
