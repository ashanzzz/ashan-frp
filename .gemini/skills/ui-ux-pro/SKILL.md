---
name: ui-ux-pro
description: High-utility UI/UX design intelligence skill. Enforces professional design systems, sleek dark modes, glassmorphism, harmonious color palettes, modern typography, micro-interactions, responsive accessibility, and anti-pattern prevention for web applications.
---

# UI/UX Pro Max Design Skill

The `ui-ux-pro` skill equips the AI assistant with top-tier product design, design system architecture, and visual aesthetics capabilities.

---

## 🎨 Core Design System Principles

### 1. Color Palette & Lighting
- **Never use generic default colors**: Avoid plain `#ff0000` (red), `#0000ff` (blue), or `#00ff00` (green).
- **Tailored HSL / RGB Systems**:
  - Primary Accent: Neon Cyan / Electric Blue (`#79d7ff` / `rgb(91, 140, 255)`)
  - Secondary Accent: Emerald Green (`#40e0a0` / `rgb(48, 201, 143)`)
  - Dark Glass Backdrop: Semi-transparent deep obsidian (`rgba(13, 18, 26, 0.88)` with `backdrop-filter: blur(16px)`).
  - Borders: Translucent white overlay (`border: 1px solid rgba(255, 255, 255, 0.08)`).

### 2. Typography & Spatial Rhythm
- **Modern Sans-Serif Stack**: Inter, system-ui, -apple-system, Segoe UI, Roboto, sans-serif.
- **Monospace Stack**: JetBrains Mono, Fira Code, SF Mono, Menlo, monospace for IDs and targets.
- **Proportional Scaling**: Use 4px/8px spatial grids for padding, margins, and border radii (e.g. 8px, 12px, 16px, 24px).

### 3. Glassmorphism & Micro-Interactions
- **Glass Panel Styling**:
  - `backdrop-filter: blur(12px)`
  - `box-shadow: 0 8px 32px rgba(0, 0, 0, 0.36), inset 0 1px 0 rgba(255, 255, 255, 0.08)`
  - `transition: all 0.22s cubic-bezier(0.4, 0, 0.2, 1)`
- **Interactive Feedback**:
  - Button hover: Light glow, subtle scale or background elevation.
  - Active/Open States: Distinct cyan outline (`border-color: rgba(91, 140, 255, 0.4)`).
  - Busy Loading: `⏳ 处理中...` inline disabled spinner with cursor feedback.

## 🇨🇳 China Network Accessibility Rule (国内零延迟网络原则)

> [!CRITICAL]
> **严禁在 CSS 中直接引入外部 `fonts.googleapis.com` 或国外 CDN 字体**。
> 在中国国内网络环境下，外部 Google Fonts 容易因代理或网络延迟导致页面加载阻塞、渲染卡顿。
>
> **替代方案**：
> 1. 优先采用零延迟的系统内置字体栈：
>    - 无衬线字体：`font-family: Inter, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;`
>    - 等宽代码字体：`font-family: ui-monospace, SFMono-Regular, Consolas, "Liberation Mono", Menlo, Monaco, "Fira Code", monospace;`
> 2. 如必须使用特定字体，需打包至本地 `/dist/fonts/` 静态资源目录或使用国内镜像 CDN，确保 0 延迟加载。

---

## 🚫 Anti-Pattern Checklist (Must Avoid)

- ❌ Stark, unstyled tables without clear sub-grouping or status badges.
- ❌ Automatic accordion collapse upon clicking inner row actions (preserve open state!).
- ❌ Sudden page jumps or scroll position resets on background actions.
- ❌ Plain black/white default browser form controls.
- ❌ Blinding raw colors without opacity blending.

---

## 🚀 Execution & Verification
When implementing or editing UI components:
1. Ensure full dark mode compatibility & responsiveness.
2. Verify all interactive buttons contain aria labels or clear tooltips.
3. Test smooth CSS transitions across browsers.
