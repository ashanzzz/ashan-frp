# Changelog

All notable changes to the Ashan FRP project will be documented in this file.

## [Unreleased]

### Added
- **Settings Center (UI)**: Restored the complete Settings Center with automatic interactive credential mapping.
- **ChmlFrp OAuth Integration (UI)**: Implemented automated device-code based OAuth login for ChmlFrp. It features a one-click authorization pop-up and backend polling to automatically exchange tokens, eliminating manual copying.
- **Cloudflare DNS Domain Selection (UI)**: Restored Cloudflare intelligent token verification. Entering an API token now fetches and lists all accessible Zone domains (`.xyz`, etc.) for users to select, bypassing manual Zone ID input.
- **Permanent Status Cards (UI)**: Added persistent Cloudflare and ChmlFrp service connection status badges to the bottom of the left sidebar for real-time upstream tracking.
- **Dynamic Node Filtering**: Added `webSupported` smart calculation evaluating `HTTP/HTTPS` ports and notes text to filter Nodes specifically built for web proxies ("🌐 只看支持建站节点").
- **Unauthenticated Alert Banners**: Added immediate frontend awareness for invalid or missing ChmlFrp tokens alongside quick-jump links.

### Changed
- **Frontend Architecture Rewrite**: Fully migrated the legacy static HTML frontend to a modern responsive **Vue 3 + Tailwind CSS** SPA setup, bundled by Vite and embedded efficiently into the single Go binary.
- **Navigation Redesign**: Restructured the sidebar navigation tree. Nested `Tunnels` and `Nodes` under the `🔌 ChmlFrp Platform` section, and rebranded `Daemon` to `⚙️ FRPC 进程`.
- **Node Synchronous IP Resolution**: ChmlFrp nodes using domain name endpoints are now actively resolved to IPv4 locally using `net.LookupHost` with fallback to `nodeinfo` API for enhanced stability.
- **V2 Apifox Contract Alignment**: Restructured SQLite mapping and frontend rendering logic to adhere to the latest V2 camelCase JSON tags from ChmlFrp (e.g. `tunnelID`, `tunnelName`, `localIP`).
- **Uptime Kuma Style UI Forms**: Integrated Uptime-Kuma-style `.form-select` dropdown aesthetics, SVG arrows, ring focus effects, and `<optgroup>` categorizations for polished form interaction.

### Fixed
- **Cloudflare Zone Recognition**: Fixed a JSON property mapping bug in the frontend (`res.data.zones`) that prevented Cloudflare Zones from correctly populating the dropdown when verifying valid API Tokens.
- **Node WebSupported Flags**: Updated ChmlFrp node parsing logic to align with V2 API changes (where the `wed` field is deprecated), dynamically evaluating `web_supported` via port flags and nodegroup mapping.
- **Node IP Resolution Accuracy**: Improved node real IP retrieval to robustly resolve `{name}.ip.chmlfrp.cn` via DNS, addressing V2 API's omission of direct IP strings.
- Fixed ChmlFrp node IPs failing to load when node domains are not directly returned as IP strings.
- Fixed trailing whitespace and Vite hot-reload styling artifacts causing `verify.ps1` CI checks to fail.
