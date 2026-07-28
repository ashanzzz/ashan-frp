# Changelog

All notable changes to the Ashan FRP project will be documented in this file.

## [Unreleased]

### Added
- **Cloudflare Credential Auto-Detection**: Added the authenticated `POST /api/v1/settings/integrations/cloudflare/configure` flow. It distinguishes scoped API Tokens from Global API Keys, requests the account email for Global Key headers, discovers accessible Zones, auto-selects a single Zone, asks the user to select among multiple Zones, validates DNS read access, and atomically saves the verified secret plus Zone metadata.
- **Personal Settings Secret Display**: The authenticated single-admin Settings Center now intentionally returns and displays full Cloudflare and ChmlFrp secrets in plaintext with `Cache-Control: no-store`; encrypted-at-rest storage and log/audit redaction remain mandatory.
- **Settings Center (UI)**: Restored the complete Settings Center with automatic interactive credential mapping.
- **ChmlFrp OAuth Integration (UI)**: Implemented automated device-code based OAuth login for ChmlFrp. It features a one-click authorization pop-up and backend polling to automatically exchange tokens, eliminating manual copying.
- **Permanent Status Cards (UI)**: Added persistent Cloudflare and ChmlFrp service connection status badges to the bottom of the left sidebar for real-time upstream tracking.
- **Dynamic Node Filtering**: Added `webSupported` smart calculation evaluating `HTTP/HTTPS` ports and notes text to filter Nodes specifically built for web proxies ("🌐 只看支持建站节点").
- **Unauthenticated Alert Banners**: Added immediate frontend awareness for invalid or missing ChmlFrp tokens alongside quick-jump links.

### Changed
- **Embedded UI Rendering Performance**: Replaced translucent backdrop-blur panels, header, cards, and modal layers with opaque dark surfaces. This avoids repeated GPU backdrop recomposition during native select and modal interactions while preserving the existing visual hierarchy.
- **Embedded UI Cache Busting**: The Vite production build now appends reproducible SHA-256 content hashes to embedded JavaScript and CSS URLs so browser caches cannot retain stale control logic after an upgrade.
- **Frontend Architecture Rewrite**: Fully migrated the legacy static HTML frontend to a modern responsive **Vue 3 + Tailwind CSS** SPA setup, bundled by Vite and embedded efficiently into the single Go binary.
- **Navigation Redesign**: Restructured the sidebar navigation tree. Nested `Tunnels` and `Nodes` under the `🔌 ChmlFrp Platform` section, and rebranded `Daemon` to `⚙️ FRPC 进程`.
- **Node Synchronous IP Resolution**: ChmlFrp nodes using domain name endpoints are now actively resolved to IPv4 locally using `net.LookupHost` with fallback to `nodeinfo` API for enhanced stability.
- **V2 Apifox Contract Alignment**: Restructured SQLite mapping and frontend rendering logic to adhere to the latest V2 camelCase JSON tags from ChmlFrp (e.g. `tunnelID`, `tunnelName`, `localIP`).
- **Uptime Kuma Style UI Forms**: Integrated Uptime-Kuma-style `.form-select` dropdown aesthetics, SVG arrows, ring focus effects, and `<optgroup>` categorizations for polished form interaction.

### Fixed
- **Provider Authentication Boundary**: The Vue console now probes `/api/v1/auth/session` before protected API work and provides a real re-login form. A local `401 UNAUTHORIZED` from Cloudflare configuration is explicitly shown as “not sent to Cloudflare”, rather than being misreported as an upstream Cloudflare failure.
- **ChmlFrp Current Credential Identity**: Manual Token and OAuth saves now validate ChmlFrp `/userinfo`, persist the verified account name, and show the authenticated Settings user both the current account and full current Token in plaintext. Global settings saves no longer re-submit an unchanged ChmlFrp Token.
- **Settings Save Contract**: The Vue settings form now uses the backend's `PATCH /api/v1/settings` contract instead of the unsupported `PUT` method and resubmits the complete settings snapshot to avoid resetting unrelated sections.
- **Credential Storage Boundary**: Removed the second raw integration-settings write that could persist submitted provider secrets in plaintext; Cloudflare secrets now enter SQLite only through encrypted credential storage after verification.
- **Credential Response Boundary**: Limited plaintext provider secrets to the authenticated settings snapshot and successful Cloudflare configure response; generic settings PATCH and Cloudflare verification responses remain secret-free.
- **Cloudflare Zone Discovery**: Paginate the full accessible Zone list instead of silently stopping after the first 50 Zones.
- **Cloudflare Zone Recognition**: Fixed a JSON property mapping bug in the frontend (`res.data.zones`) that prevented Cloudflare Zones from correctly populating the dropdown when verifying valid API Tokens.
- **Node WebSupported Flags**: Updated ChmlFrp node parsing logic to align with V2 API changes (where the `wed` field is deprecated), dynamically evaluating `web_supported` via port flags and nodegroup mapping.
- **Node IP Resolution Accuracy**: Improved node real IP retrieval to robustly resolve `{name}.ip.chmlfrp.cn` via DNS, addressing V2 API's omission of direct IP strings.
- Fixed ChmlFrp node IPs failing to load when node domains are not directly returned as IP strings.
- Fixed trailing whitespace and Vite hot-reload styling artifacts causing `verify.ps1` CI checks to fail.
