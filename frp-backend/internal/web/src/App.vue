<template>
  <!-- 1. DEDICATED FULL-SCREEN LOGIN PAGE (When unauthenticated) -->
  <div v-if="!isAuthenticated" class="min-h-screen bg-[#070a0f] flex flex-col items-center justify-center p-4 selection:bg-blue-500 selection:text-white">
    <div class="glass-panel max-w-md w-full p-8 space-y-6 border border-gray-800 shadow-2xl rounded-2xl bg-[#0b0f17]/95">
      <div class="text-center space-y-2">
        <div class="w-14 h-14 rounded-2xl bg-gradient-to-tr from-blue-600 to-emerald-500 flex items-center justify-center font-bold text-white shadow-xl shadow-blue-500/25 mx-auto text-2xl">
          AF
        </div>
        <h2 class="text-xl font-bold tracking-tight text-white">Ashan FRP · 管理员登录</h2>
        <p class="text-xs text-gray-400">单管理员私有控制面 · 需本地凭据鉴权保护</p>
      </div>

      <div v-if="loginError" class="p-3.5 rounded-xl bg-red-500/10 border border-red-500/30 text-red-400 text-xs flex items-center gap-2">
        <span>⚠️</span> {{ loginError }}
      </div>

      <form @submit.prevent="submitLogin" class="space-y-4">
        <div>
          <label class="block text-xs font-semibold text-gray-400 mb-1.5">管理员账号</label>
          <input v-model="loginForm.username" type="text" required placeholder="admin" class="w-full px-4 py-3 rounded-xl bg-gray-900 border border-gray-700 text-sm text-white focus:outline-none focus:border-blue-500 font-mono" />
        </div>
        <div>
          <label class="block text-xs font-semibold text-gray-400 mb-1.5">管理员密码</label>
          <input v-model="loginForm.password" type="password" required placeholder="••••••••" class="w-full px-4 py-3 rounded-xl bg-gray-900 border border-gray-700 text-sm text-white focus:outline-none focus:border-blue-500" />
        </div>
        <button type="submit" :disabled="loginLoading" class="w-full py-3 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold text-sm transition shadow-lg shadow-blue-600/30 active:scale-[0.98] flex items-center justify-center gap-2">
          <span v-if="loginLoading" class="animate-spin">🔄</span>
          {{ loginLoading ? '正在验证身份...' : '验证并进入控制台' }}
        </button>
      </form>
      <div class="pt-2 text-center text-[11px] text-gray-500 border-t border-gray-800/80">
        安全策略：凭据严格校验，未授权请求将被全面拦截
      </div>
    </div>
  </div>

  <!-- 2. AUTHENTICATED APP CONSOLE (When authenticated) -->
  <div v-else class="min-h-screen bg-[#070a0f] text-gray-100 flex flex-col font-sans">
    <!-- Header -->
    <header class="border-b border-gray-800/80 bg-[#0b0f17] sticky top-0 z-40 px-6 py-4 flex items-center justify-between">
      <div class="flex items-center gap-3">
        <div class="w-9 h-9 rounded-xl bg-gradient-to-tr from-blue-600 to-emerald-500 flex items-center justify-center font-bold text-white shadow-lg shadow-blue-500/20">
          AF
        </div>
        <div>
          <h1 class="text-lg font-bold tracking-tight text-white flex items-center gap-2">
            Ashan FRP
            <span class="text-xs px-2 py-0.5 rounded-full bg-blue-500/10 text-blue-400 border border-blue-500/20 font-medium">Vue 3 Console</span>
          </h1>
          <p class="text-xs text-gray-400">高效穿透代理与多节点分布式调度控制台</p>
        </div>
      </div>

      <div class="flex items-center gap-3">
        <div class="flex items-center gap-2 mr-2">
          <span class="text-xs px-2.5 py-1 rounded-lg bg-gray-800/90 border border-gray-700 text-gray-300 font-mono flex items-center gap-1.5">
            <span class="w-2 h-2 rounded-full bg-emerald-400"></span>
            {{ currentAccount?.login_name || 'admin' }}
          </span>
          <button @click="openPasswordModal" class="px-2.5 py-1 rounded-lg bg-gray-800 hover:bg-gray-700 text-gray-300 border border-gray-700 text-xs font-medium transition">
            修改密码
          </button>
          <button @click="logout" class="px-2.5 py-1 rounded-lg bg-red-500/10 hover:bg-red-500/20 text-red-400 border border-red-500/20 text-xs font-semibold transition">
            退出登录
          </button>
        </div>

        <span class="text-xs px-3 py-1 rounded-lg bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-semibold flex items-center gap-1.5">
          <span class="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
          系统就绪
        </span>
        <button @click="fetchData" :disabled="loading" class="px-3.5 py-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 text-xs font-medium text-gray-200 border border-gray-700 transition active:scale-95 flex items-center gap-1.5">
          <span :class="{ 'animate-spin': loading }">🔄</span>
          刷新数据
        </button>
      </div>
    </header>

    <!-- Main Content Area -->
    <div class="flex-1 flex max-w-[1600px] w-full mx-auto p-6 gap-6">
      <!-- Sidebar Navigation -->
      <aside class="w-64 shrink-0 flex flex-col gap-2">
        <div class="text-[11px] font-bold text-gray-500 uppercase tracking-wider px-3 mb-1">功能导航</div>

        <!-- ⚡ 总控制台 -->
        <button
          @click="activePage = 'control'"
          :class="[
            'w-full text-left px-4 py-3 rounded-xl text-sm font-medium transition flex items-center gap-3',
            activePage === 'control'
              ? 'bg-blue-600/15 text-blue-400 border border-blue-500/30 font-semibold shadow-sm'
              : 'text-gray-400 hover:bg-gray-800/50 hover:text-gray-200'
          ]"
        >
          <span class="text-base">⚡</span>
          总控制台
        </button>

        <!-- ⚙️ FRPC 进程 -->
        <button
          @click="activePage = 'frp'"
          :class="[
            'w-full text-left px-4 py-3 rounded-xl text-sm font-medium transition flex items-center gap-3',
            activePage === 'frp'
              ? 'bg-blue-600/15 text-blue-400 border border-blue-500/30 font-semibold shadow-sm'
              : 'text-gray-400 hover:bg-gray-800/50 hover:text-gray-200'
          ]"
        >
          <span class="text-base">⚙️</span>
          FRPC 进程守护
        </button>

        <!-- 🔌 ChmlFrp 平台子菜单 -->
        <div class="mt-3 mb-1 px-3 flex items-center gap-1.5 text-[11px] font-bold text-gray-500 uppercase tracking-wider">
          <span>🔌</span>
          <span>ChmlFrp 服务商平台</span>
        </div>

        <div class="space-y-1 pl-2 border-l border-gray-800/80 ml-3">
          <!-- 🚀 穿透规则 (Tunnels) -->
          <button
            @click="activePage = 'tunnels'"
            :class="[
              'w-full text-left px-3.5 py-2.5 rounded-xl text-sm font-medium transition flex items-center justify-between',
              activePage === 'tunnels'
                ? 'bg-blue-600/15 text-blue-400 border border-blue-500/30 font-semibold shadow-sm'
                : 'text-gray-400 hover:bg-gray-800/50 hover:text-gray-200'
            ]"
          >
            <div class="flex items-center gap-2.5">
              <span class="text-sm">🚀</span>
              <span>穿透规则</span>
            </div>
            <span class="text-[10px] px-1.5 py-0.5 rounded bg-gray-800 text-gray-400 border border-gray-700">
              {{ tunnels.length }}/16
            </span>
          </button>

          <!-- 🌐 网络节点 (Nodes) -->
          <button
            @click="activePage = 'nodes'"
            :class="[
              'w-full text-left px-3.5 py-2.5 rounded-xl text-sm font-medium transition flex items-center justify-between',
              activePage === 'nodes'
                ? 'bg-blue-600/15 text-blue-400 border border-blue-500/30 font-semibold shadow-sm'
                : 'text-gray-400 hover:bg-gray-800/50 hover:text-gray-200'
            ]"
          >
            <div class="flex items-center gap-2.5">
              <span class="text-sm">🌐</span>
              <span>网络节点</span>
            </div>
            <span class="text-[10px] px-1.5 py-0.5 rounded bg-gray-800 text-gray-400 border border-gray-700">
              {{ nodes.length }}
            </span>
          </button>
        </div>

        <!-- ☁️ Cloudflare DNS -->
        <button
          @click="activePage = 'dns'"
          :class="[
            'w-full text-left px-4 py-3 rounded-xl text-sm font-medium transition flex items-center justify-between',
            activePage === 'dns'
              ? 'bg-blue-600/15 text-blue-400 border border-blue-500/30 font-semibold shadow-sm'
              : 'text-gray-400 hover:bg-gray-800/50 hover:text-gray-200'
          ]"
        >
          <div class="flex items-center gap-3">
            <span class="text-base">☁️</span>
            <span>Cloudflare DNS</span>
          </div>
          <span v-if="dnsRecords.length" class="text-[10px] px-1.5 py-0.5 rounded bg-gray-800 text-gray-400 border border-gray-700">
            {{ dnsRecords.length }}
          </span>
        </button>

        <!-- ⚙️ 设置中心 -->
        <button
          @click="activePage = 'settings'"
          :class="[
            'w-full text-left px-4 py-3 rounded-xl text-sm font-medium transition flex items-center gap-3',
            activePage === 'settings'
              ? 'bg-blue-600/15 text-blue-400 border border-blue-500/30 font-semibold shadow-sm'
              : 'text-gray-400 hover:bg-gray-800/50 hover:text-gray-200'
          ]"
        >
          <span class="text-base">⚙️</span>
          设置中心
        </button>

        <!-- 长期服务商连接状态 (侧边栏底部常驻) -->
        <div class="mt-auto pt-4 border-t border-gray-800/80 space-y-2">
          <div class="text-[10px] font-bold text-gray-500 uppercase tracking-wider px-1">服务商连通状态</div>

          <div @click="activePage = 'settings'" class="p-2.5 rounded-xl bg-gray-900/90 border border-gray-800 hover:border-blue-500/40 cursor-pointer transition flex items-center justify-between">
            <div class="flex items-center gap-2 text-xs">
              <span>🔌</span>
              <div>
                <div class="font-bold text-gray-300 text-xs">ChmlFrp</div>
                <div class="text-[10px] text-gray-400 truncate max-w-[95px]">{{ chmlfrpStatus.text }}</div>
              </div>
            </div>
            <span :class="['px-2 py-0.5 rounded text-[10px] font-bold', chmlfrpStatus.connected ? 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30' : 'bg-red-500/15 text-red-400 border border-red-500/30']">
              {{ chmlfrpStatus.label }}
            </span>
          </div>

          <div @click="activePage = 'settings'" class="p-2.5 rounded-xl bg-gray-900/90 border border-gray-800 hover:border-blue-500/40 cursor-pointer transition flex items-center justify-between">
            <div class="flex items-center gap-2 text-xs">
              <span>☁️</span>
              <div>
                <div class="font-bold text-gray-300 text-xs">Cloudflare</div>
                <div class="text-[10px] text-gray-400 truncate max-w-[95px]">{{ cloudflareStatus.text }}</div>
              </div>
            </div>
            <span :class="['px-2 py-0.5 rounded text-[10px] font-bold', cloudflareStatus.connected ? 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30' : 'bg-gray-800 text-gray-400']">
              {{ cloudflareStatus.label }}
            </span>
          </div>
        </div>
      </aside>

      <!-- Main Body View -->
      <main class="flex-1 min-w-0">
        <!-- Banner / Notice Alert -->
        <div v-if="notice" class="mb-4 p-4 rounded-xl bg-emerald-500/10 border border-emerald-500/30 text-emerald-300 text-sm flex items-center justify-between animate-fade-in">
          <span>✅ {{ notice }}</span>
          <button @click="notice = ''" class="text-emerald-400 hover:text-emerald-200 text-xs">✕</button>
        </div>
        <div v-if="error" class="mb-4 p-4 rounded-xl bg-red-500/10 border border-red-500/30 text-red-300 text-sm flex items-center justify-between animate-fade-in">
          <span>⚠️ {{ error }}</span>
          <button @click="error = ''" class="text-red-400 hover:text-red-200 text-xs">✕</button>
        </div>

        <!-- ⚡ 控制台 VIEW -->
        <section v-if="activePage === 'control'" class="space-y-6">
          <div class="flex items-center justify-between">
            <div>
              <h2 class="text-xl font-bold text-white">总控制台</h2>
              <p class="text-xs text-gray-400 mt-1">快捷域名映射 · 隧道状态矩阵 · 四级健康监控</p>
            </div>
            <button @click="openControlModal()" class="px-4 py-2 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold text-sm shadow-lg shadow-blue-600/30 transition active:scale-95 flex items-center gap-2">
              <span>➕</span> 新增映射
            </button>
          </div>

          <!-- 4 层系统健康监控状态卡片 -->
          <div class="grid grid-cols-4 gap-4">
            <div class="glass-card p-4">
              <div class="text-xs text-gray-400">Go Runtime</div>
              <div class="text-lg font-bold text-emerald-400 mt-1">正常运行</div>
              <div class="text-[11px] text-gray-500 mt-0.5">Go 1.26 单二进制</div>
            </div>
            <div class="glass-card p-4">
              <div class="text-xs text-gray-400">SQLite 存储</div>
              <div class="text-lg font-bold text-emerald-400 mt-1">WAL 已就绪</div>
              <div class="text-[11px] text-gray-500 mt-0.5">单连接防锁机制生效</div>
            </div>
            <div class="glass-card p-4">
              <div class="text-xs text-gray-400">FRPC Supervisor</div>
              <div :class="['text-lg font-bold mt-1', frpcRuntime?.status === 'running' ? 'text-emerald-400' : 'text-amber-400']">
                {{ frpcRuntime?.status === 'running' ? '守护运行中' : (frpcRuntime?.status || '就绪') }}
              </div>
              <div class="text-[11px] text-gray-500 mt-0.5">PID: {{ frpcRuntime?.pid || '—' }}</div>
            </div>
            <div class="glass-card p-4">
              <div class="text-xs text-gray-400">外部适配器</div>
              <div class="text-lg font-bold text-blue-400 mt-1">
                {{ chmlfrpStatus.connected && cloudflareStatus.connected ? '全链路就绪' : '部分已配置' }}
              </div>
              <div class="text-[11px] text-gray-500 mt-0.5">ChmlFrp & Cloudflare</div>
            </div>
          </div>

          <!-- ⚡ 节点故障自动迁移自愈与周度优选状态条 -->
          <div class="glass-card p-4 border border-blue-500/30 bg-blue-950/20 flex flex-wrap items-center justify-between gap-4">
            <div class="flex items-center gap-3">
              <div class="w-10 h-10 rounded-xl bg-blue-500/20 flex items-center justify-center text-blue-400 text-lg">
                ⚡
              </div>
              <div>
                <div class="flex items-center gap-2">
                  <span class="text-sm font-bold text-white">双轨切换中心 (Failover & Weekly Optimizer)</span>
                  <span :class="['px-2 py-0.5 rounded-full text-[11px] font-semibold border', failoverStatus.tracked_node_online ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30' : 'bg-rose-500/15 text-rose-400 border-rose-500/30']">
                    {{ failoverStatus.tracked_node_online ? '● 核心节点健康在线' : `● 节点已离线 ${failoverStatus.continuous_offline_min} 分钟` }}
                  </span>
                </div>
                <p class="text-xs text-gray-400 mt-0.5">
                  监控核心节点 <span class="text-gray-200 font-mono font-medium">{{ failoverStatus.tracked_node || '北京多线' }} ({{ failoverStatus.tracked_node_ip || '82.156.215.91' }})</span> ·
                  离线容忍阈值: <span class="text-amber-300 font-bold">{{ failoverStatus.offline_threshold_min || 10 }} 分钟</span> ·
                  周度凌晨优选: <span class="text-emerald-300 font-medium">每周一 03:30 (自动测速提速/坏了自愈回滚)</span>
                  <span v-if="failoverStatus.candidate_best_node" class="text-gray-400 ml-2">| 测速优选备用: <span class="text-emerald-400">{{ failoverStatus.candidate_best_node }} ({{ failoverStatus.candidate_best_latency }}ms, {{ failoverStatus.candidate_best_speed_mbps || 0 }}Mbps)</span></span>
                </p>
                <div v-if="failoverStatus.last_failover_log" class="text-[11px] text-gray-400 mt-1 bg-gray-900/60 px-2 py-1 rounded border border-gray-800">
                  <span class="text-amber-400 font-medium">离线自愈日志:</span> {{ failoverStatus.last_failover_log }}
                </div>
                <div v-if="failoverStatus.last_optimize_log" class="text-[11px] text-gray-400 mt-1 bg-gray-900/60 px-2 py-1 rounded border border-gray-800">
                  <span class="text-blue-400 font-medium">周度优选日志:</span> {{ failoverStatus.last_optimize_log }}
                </div>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <button @click="triggerWeeklyOptimize()" :disabled="failoverTriggering" class="px-3 py-1.5 rounded-lg bg-blue-900/40 hover:bg-blue-800/60 text-xs font-semibold text-blue-200 border border-blue-700/50 transition">
                {{ failoverTriggering ? '测速中...' : '🚀 周度优选测速' }}
              </button>
              <button @click="triggerManualFailover()" :disabled="failoverTriggering" class="px-3 py-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 text-xs font-semibold text-gray-200 border border-gray-700 transition">
                {{ failoverTriggering ? '切换中...' : '⚡ 演练一键迁移' }}
              </button>
              <button @click="toggleAutoFailover()" :class="['px-3 py-1.5 rounded-lg text-xs font-semibold transition border', failoverStatus.auto_failover_enabled ? 'bg-emerald-600 hover:bg-emerald-500 text-white border-emerald-500' : 'bg-gray-800 hover:bg-gray-700 text-gray-400 border-gray-700']">
                {{ failoverStatus.auto_failover_enabled ? '✓ 自愈已开启 (10分钟)' : '✕ 自愈已暂停' }}
              </button>
            </div>
          </div>

          <!-- Table -->
          <div class="glass-panel p-5">
            <div class="flex flex-wrap items-center justify-between gap-3 mb-4">
              <div>
                <h3 class="text-sm font-bold text-white flex items-center gap-2">
                  <span>🌐</span> 全链路状态总控矩阵
                  <span class="text-[11px] px-2 py-0.5 rounded-full bg-blue-500/10 text-blue-400 border border-blue-500/20 font-medium">1Panel · ChmlFrp · Cloudflare · FRPC</span>
                </h3>
                <p class="text-xs text-gray-400 mt-0.5">端到端链路观测：真实容器后端 ➔ 1Panel 反代与 SSL ➔ 本地 FRPC ➔ 上游穿透节点 ➔ Cloudflare DNS 解析</p>
              </div>
              <div class="flex items-center gap-2.5">
                <input 
                  v-model="matrixFilter" 
                  placeholder="🔍 搜索映射、域名、端口、节点..." 
                  class="px-3 py-1.5 rounded-lg bg-gray-900 border border-gray-700 text-xs text-white placeholder-gray-500 focus:outline-none focus:border-blue-500 w-56"
                />
                <button @click="syncMatrix" :disabled="matrixLoading" class="px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs transition active:scale-95 flex items-center gap-1.5 shadow-md shadow-emerald-600/20">
                  <span :class="{ 'animate-spin': matrixLoading }">🔄</span>
                  {{ matrixLoading ? '同步中...' : '全链路一键同步' }}
                </button>
              </div>
            </div>

            <div class="overflow-x-auto">
              <table class="w-full text-left text-sm">
                <thead>
                  <tr class="border-b border-gray-800 text-gray-400 text-xs uppercase tracking-wider">
                    <th class="py-3 px-3">映射服务 / 协议</th>
                    <th class="py-3 px-3">公网访问域名 / 端点</th>
                    <th class="py-3 px-3">1Panel 反代与目标</th>
                    <th class="py-3 px-3">SSL 证书安全</th>
                    <th class="py-3 px-3">ChmlFrp 节点 / 穿透</th>
                    <th class="py-3 px-3">Cloudflare DNS</th>
                    <th class="py-3 px-3">全链路健康</th>
                    <th class="py-3 px-3 text-right">快捷操作</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-800/50">
                  <tr v-for="m in displayMatrix" :key="m.id" class="hover:bg-gray-800/40 transition">
                    <!-- 1. 映射服务 / 协议 -->
                    <td class="py-3.5 px-3">
                      <div class="flex items-center gap-2">
                        <span class="font-bold text-white font-mono text-sm">{{ m.name }}</span>
                        <span :class="['px-1.5 py-0.5 rounded text-[10px] font-bold tracking-wider uppercase border', m.protocol === 'HTTPS' ? 'bg-indigo-500/15 text-indigo-400 border-indigo-500/30' : 'bg-gray-800 text-gray-300 border-gray-700']">
                          {{ m.protocol || 'HTTPS' }}
                        </span>
                      </div>
                      <div class="text-[11px] text-gray-400 mt-0.5 font-sans">
                        {{ getServiceBadge(m.name) }}
                      </div>
                    </td>

                    <!-- 2. 访问域名 / 端点 -->
                    <td class="py-3.5 px-3">
                      <div class="flex items-center gap-1.5">
                        <a :href="'https://' + m.full_domain" target="_blank" class="text-xs font-mono font-medium text-blue-400 hover:text-blue-300 hover:underline flex items-center gap-1">
                          {{ m.full_domain || '—' }}
                          <span class="text-[10px]">↗</span>
                        </a>
                        <button v-if="m.full_domain" @click="copyText(m.full_domain)" title="复制域名" class="text-gray-500 hover:text-gray-300 text-xs px-1">
                          📋
                        </button>
                      </div>
                      <div class="text-[11px] text-gray-500 mt-0.5 font-mono">
                        {{ m.local_ip || '192.168.8.11' }}:{{ m.local_port || m.onepanel?.listen_port }}
                      </div>
                    </td>

                    <!-- 3. 1Panel 反代与后端目标 -->
                    <td class="py-3.5 px-3">
                      <div v-if="m.onepanel?.configured || m.onepanel?.listen_port" class="space-y-0.5">
                        <div class="flex items-center gap-1.5">
                          <span class="px-1.5 py-0.5 rounded text-[11px] font-mono font-semibold bg-blue-950/60 text-blue-300 border border-blue-800/60">
                            :{{ m.onepanel?.listen_port || m.local_port }}
                          </span>
                          <span class="text-[11px] text-gray-400">➔</span>
                          <span class="text-[11px] font-mono text-emerald-300 bg-emerald-950/40 px-1.5 py-0.5 rounded border border-emerald-800/40" :title="m.onepanel?.proxy_target">
                            {{ formatTarget(m.onepanel?.proxy_target || m.local_ip + ':' + m.local_port) }}
                          </span>
                        </div>
                        <div class="text-[10px] text-gray-500">1Panel 站点 #{{ m.onepanel?.website_id || '已映射' }}</div>
                      </div>
                      <div v-else class="text-xs text-gray-500">
                        直连 :{{ m.local_port }}
                      </div>
                    </td>

                    <!-- 4. SSL 证书状态 -->
                    <td class="py-3.5 px-3">
                      <div v-if="m.onepanel?.ssl_enabled || m.protocol === 'HTTPS'" class="space-y-0.5">
                        <span :class="['px-2 py-0.5 rounded-full text-[11px] font-semibold flex items-center gap-1 w-max border', m.onepanel?.ssl_days_left > 30 || !m.onepanel?.ssl_days_left ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30' : 'bg-amber-500/15 text-amber-400 border-amber-500/30']">
                          <span>🔒</span>
                          {{ m.onepanel?.ssl_days_left ? `有效 (余 ${m.onepanel.ssl_days_left} 天)` : 'SSL 已就绪' }}
                        </span>
                        <div v-if="m.onepanel?.ssl_expire_at" class="text-[10px] text-gray-500">
                          到期: {{ formatDate(m.onepanel.ssl_expire_at) }}
                        </div>
                      </div>
                      <span v-else class="px-2 py-0.5 rounded text-[11px] bg-gray-800 text-gray-500 border border-gray-700">
                        未开启 SSL
                      </span>
                    </td>

                    <!-- 5. ChmlFrp 节点 / 穿透 -->
                    <td class="py-3.5 px-3">
                      <div class="space-y-0.5">
                        <div class="flex items-center gap-1.5 text-xs text-gray-200 font-medium">
                          <span :class="['w-2 h-2 rounded-full', m.chmlfrp?.node_online ? 'bg-emerald-400 animate-pulse' : 'bg-rose-500']"></span>
                          {{ m.chmlfrp?.node_name || m.chmlfrp_node || '未分配节点' }}
                        </div>
                        <div class="flex items-center gap-1.5">
                          <span class="text-[10px] font-mono text-gray-400">{{ m.chmlfrp?.node_ip || '—' }}</span>
                          <span :class="['text-[10px] px-1 rounded border', m.chmlfrp?.synced ? 'bg-blue-500/10 text-blue-400 border-blue-500/20' : 'bg-gray-800 text-gray-400 border-gray-700']">
                            {{ m.chmlfrp?.synced ? '已同步' : '本地生效' }}
                          </span>
                        </div>
                      </div>
                    </td>

                    <!-- 6. Cloudflare DNS -->
                    <td class="py-3.5 px-3">
                      <div v-if="m.cloudflare?.configured" class="space-y-0.5">
                        <div class="flex items-center gap-1.5">
                          <span :class="['px-1.5 py-0.5 rounded text-[10px] font-semibold border flex items-center gap-1', m.cloudflare?.is_aligned ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20' : 'bg-rose-500/15 text-rose-400 border-rose-500/30']">
                            {{ m.cloudflare?.is_aligned ? '✓ A 记录对齐' : '⚠️ IP 漂移' }}
                          </span>
                          <span class="text-[10px] text-gray-500" :title="m.cloudflare?.proxied ? 'Cloudflare CDN 代理开启' : '仅 DNS 解析'">
                            {{ m.cloudflare?.proxied ? '☁️ 代理' : '☁️ 仅DNS' }}
                          </span>
                        </div>
                        <div class="text-[10px] font-mono text-gray-400">
                          {{ m.cloudflare?.dns_ip || '—' }}
                        </div>
                      </div>
                      <div v-else class="text-xs text-gray-500">
                        未接入 DNS
                      </div>
                    </td>

                    <!-- 7. 全链路健康 -->
                    <td class="py-3.5 px-3">
                      <span :class="['px-2 py-0.5 rounded-full text-xs font-semibold flex items-center gap-1.5 w-max border', m.status === 'healthy' ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30' : 'bg-amber-500/15 text-amber-400 border-amber-500/30']">
                        <span :class="['w-1.5 h-1.5 rounded-full', m.status === 'healthy' ? 'bg-emerald-400' : 'bg-amber-400']"></span>
                        {{ m.status === 'healthy' ? '正常运行' : '部分降级' }}
                      </span>
                    </td>

                    <!-- 8. 操作 -->
                    <td class="py-3.5 px-3 text-right space-x-1.5 whitespace-nowrap">
                      <a :href="'https://' + m.full_domain" target="_blank" class="text-xs px-2 py-1 rounded bg-blue-600/20 hover:bg-blue-600/40 text-blue-400 border border-blue-500/30 transition inline-flex items-center gap-1">
                        <span>🌐</span> 访问
                      </a>
                      <button @click="openControlModal(m)" class="text-xs px-2 py-1 rounded bg-gray-800 hover:bg-gray-700 text-gray-300 transition">
                        ✏️ 编辑
                      </button>
                      <button @click="deleteTunnel(m)" class="text-xs px-2 py-1 rounded bg-red-500/10 hover:bg-red-500/20 text-red-400 border border-red-500/20 transition">
                        🗑️
                      </button>
                    </td>
                  </tr>

                  <tr v-if="!displayMatrix.length">
                    <td colspan="8" class="py-8 text-center text-gray-500">
                      {{ matrixFilter ? `未找到匹配「${matrixFilter}」的穿透映射` : '暂无穿透映射记录，点击右上角【新增映射】开始配置。' }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </section>

        <!-- 🚀 CHMLFRP 穿透规则 VIEW -->
        <section v-if="activePage === 'tunnels'" class="space-y-6">
          <div class="flex items-center justify-between">
            <div>
              <h2 class="text-xl font-bold text-white">ChmlFrp 穿透规则</h2>
              <p class="text-xs text-gray-400 mt-1">管理服务商平台穿透映射 · 全功能增删查改 · 故障容灾优选排班</p>
            </div>
            <div class="flex items-center gap-3">
              <input 
                v-model="tunnelFilter" 
                placeholder="🔍 搜索规则名称、域名或节点..." 
                class="px-3.5 py-2 rounded-xl bg-gray-800/80 border border-gray-700 text-xs text-white placeholder-gray-500 focus:outline-none focus:border-blue-500 w-64"
              />
              <button @click="syncChmlFrpTunnels" class="px-3.5 py-2 rounded-xl bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs transition active:scale-95 flex items-center gap-1.5 shadow-lg shadow-emerald-600/20">
                🔄 同步 24h 线上规则
              </button>
              <button @click="openTunnelModal()" class="px-4 py-2 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold text-sm shadow-lg shadow-blue-600/30 transition active:scale-95 flex items-center gap-2">
                <span>➕</span> 新建规则
              </button>
            </div>
          </div>

          <!-- Login warning banner if not authenticated -->
          <div v-if="!isChmlFrpLoggedIn" class="p-4 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-300 flex items-center justify-between">
            <div class="flex items-center gap-3 text-xs">
              <span class="text-lg">⚠️</span>
              <div>
                <div class="font-bold text-sm">ChmlFrp API 账号未配置或未登录</div>
                <div class="text-amber-400/80 mt-0.5">{{ loginErrorMsg || '未检测到绑定的 ChmlFrp 密钥/凭据，无法从线上同步现有隧道列表。' }}</div>
              </div>
            </div>
            <button @click="activePage = 'settings'" class="px-3 py-1.5 rounded-lg bg-amber-500 text-gray-950 font-bold text-xs hover:bg-amber-400 transition">
              ⚙️ 去设置凭据
            </button>
          </div>

          <!-- Status Bar -->
          <div class="grid grid-cols-4 gap-4">
            <div class="glass-card p-4">
              <div class="text-xs text-gray-400">隧道规则配额</div>
              <div class="text-2xl font-bold text-white mt-1">{{ tunnels.length }} / 16</div>
              <div class="text-xs text-emerald-400 mt-1">配额使用率 {{ Math.round((tunnels.length/16)*100) }}%</div>
            </div>
            <div class="glass-card p-4">
              <div class="text-xs text-gray-400">生效在线映射</div>
              <div class="text-2xl font-bold text-emerald-400 mt-1">{{ tunnels.filter(t => t.actual_state === 'running' || t.desired_state === 'enabled').length }}</div>
              <div class="text-xs text-gray-500 mt-1">运行正常</div>
            </div>
            <div class="glass-card p-4">
              <div class="text-xs text-gray-400">可用节点网络</div>
              <div class="text-2xl font-bold text-blue-400 mt-1">{{ nodes.length }}</div>
              <div class="text-xs text-gray-500 mt-1">多线路在线</div>
            </div>
            <div class="glass-card p-4">
              <div class="text-xs text-gray-400">剩余可用额度</div>
              <div class="text-2xl font-bold text-white mt-1">{{ 16 - tunnels.length }}</div>
              <div class="text-xs text-gray-500 mt-1">随时可建</div>
            </div>
          </div>

          <div class="glass-panel p-5">
            <div class="overflow-x-auto">
              <table class="w-full text-left text-sm">
                <thead>
                  <tr class="border-b border-gray-800 text-gray-400 text-xs uppercase">
                    <th class="py-3 px-4">规则名称 / 域名</th>
                    <th class="py-3 px-4">协议</th>
                    <th class="py-3 px-4">对应节点</th>
                    <th class="py-3 px-4">本地目标</th>
                    <th class="py-3 px-4">容灾排班</th>
                    <th class="py-3 px-4">上线状态</th>
                    <th class="py-3 px-4 text-right">操作</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-800/50">
                  <tr v-for="t in filteredTunnels" :key="t.id" class="hover:bg-gray-800/30 transition">
                    <td class="py-3.5 px-4">
                      <div class="font-semibold text-white">{{ t.name || t.id }}</div>
                      <div class="text-xs text-gray-400 font-mono">{{ t.full_domain || '—' }}</div>
                    </td>
                    <td class="py-3.5 px-4">
                      <span class="px-2 py-0.5 rounded text-xs font-semibold bg-gray-800 text-gray-300 border border-gray-700">
                        {{ (t.protocol || t.tunnel_type || 'TCP').toUpperCase() }}
                      </span>
                    </td>
                    <td class="py-3.5 px-4 text-xs text-gray-300">{{ t.chmlfrp_node || t.node_id || '自动分配' }}</td>
                    <td class="py-3.5 px-4 font-mono text-xs text-gray-300">{{ t.local_ip || '127.0.0.1' }}:{{ t.local_port }}</td>
                    <td class="py-3.5 px-4">
                      <span v-if="t.is_failover_pool" class="px-2 py-0.5 rounded-full text-xs font-semibold bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">
                        ⚡ 优选 #{{ t.failover_priority || 1 }}
                      </span>
                      <span v-else class="text-xs text-gray-500">⚪ 普通线路</span>
                    </td>
                    <td class="py-3.5 px-4">
                      <span :class="['px-2 py-0.5 rounded-full text-xs font-semibold', t.actual_state === 'running' || t.desired_state === 'enabled' ? 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30' : 'bg-gray-800 text-gray-400']">
                        {{ t.actual_state || t.desired_state || 'ready' }}
                      </span>
                    </td>
                    <td class="py-3.5 px-4 text-right space-x-2">
                      <button @click="provisionTunnel(t)" class="text-xs px-2.5 py-1 rounded bg-blue-600/20 hover:bg-blue-600/40 text-blue-400 border border-blue-500/30 transition">⚡ 部署</button>
                      <button @click="openTunnelModal(t)" class="text-xs px-2.5 py-1 rounded bg-gray-800 hover:bg-gray-700 text-gray-300 transition">✏️ 编辑</button>
                      <button @click="toggleFailover(t)" class="text-xs px-2.5 py-1 rounded bg-gray-800 hover:bg-gray-700 text-gray-300 transition">
                        {{ t.is_failover_pool ? '移出优选' : '加入优选' }}
                      </button>
                      <button @click="deleteTunnel(t)" class="text-xs px-2.5 py-1 rounded bg-red-500/10 hover:bg-red-500/20 text-red-400 border border-red-500/20 transition">🗑️ 删除</button>
                    </td>
                  </tr>
                  <tr v-if="!filteredTunnels.length">
                    <td colspan="7" class="py-8 text-center text-gray-500">未找到匹配的穿透规则。</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </section>

        <!-- 🌐 CHMLFRP 网络节点 VIEW -->
        <section v-if="activePage === 'nodes'" class="space-y-6">
          <div class="flex items-center justify-between">
            <div>
              <h2 class="text-xl font-bold text-white">ChmlFrp 网络节点</h2>
              <p class="text-xs text-gray-400 mt-1">服务商网络节点 · **节点 IP** 与 **物理地区与线路** 详细视图</p>
            </div>
            <div class="flex items-center gap-3">
              <button @click="toggleWebFilter" :class="['px-3.5 py-2 rounded-xl text-xs font-semibold transition', webOnlyFilter ? 'bg-emerald-600 text-white' : 'bg-gray-800 text-gray-300 hover:bg-gray-700']">
                🌐 只看支持建站节点
              </button>
              <button @click="syncNodes" class="px-3.5 py-2 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold text-xs transition active:scale-95 flex items-center gap-1.5 shadow-lg shadow-blue-600/20">
                🔄 同步节点列表
              </button>
            </div>
          </div>

          <div class="grid grid-cols-2 gap-4">
            <div v-for="n in filteredNodes" :key="n.id" class="glass-panel p-5 space-y-3 hover:border-gray-700 transition">
              <div class="flex items-start justify-between">
                <div>
                  <div class="flex items-center gap-2">
                    <span class="text-base">{{ n.web_supported ? '🌐' : '⚡' }}</span>
                    <h3 class="font-bold text-white text-base">{{ n.display_name || n.canonical_name || n.id }}</h3>
                  </div>
                  <div class="text-xs text-gray-400 mt-0.5">ID: <span class="font-mono">{{ n.id }}</span></div>
                </div>
                <div class="flex flex-col items-end gap-1.5">
                  <span :class="['px-2.5 py-0.5 rounded-full text-xs font-semibold', n.status === 'online' ? 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30' : 'bg-gray-800 text-gray-400']">
                    {{ n.status || '在线' }}
                  </span>
                  <span v-if="n.web_supported" class="px-2 py-0.5 rounded text-[10px] font-semibold bg-blue-500/15 text-blue-400 border border-blue-500/30">
                    支持建站 (Web)
                  </span>
                </div>
              </div>

              <!-- 物理地区与真实 IP 属性看板 -->
              <div class="grid grid-cols-2 gap-2 text-xs bg-gray-900/60 p-3 rounded-xl border border-gray-800/80">
                <div>
                  <div class="text-gray-500 text-[11px]">物理地区与线路 (Region)</div>
                  <div class="text-gray-200 font-semibold mt-0.5">📍 {{ n.region || n.area || '全国通用 BGP' }}</div>
                </div>
                <div>
                  <div class="text-gray-500 text-[11px]">节点 IP (Real IP)</div>
                  <div class="text-blue-400 font-mono mt-0.5">{{ n.real_ip || n.endpoint_url || '动态解析中' }}</div>
                </div>
              </div>

              <div class="text-xs text-gray-400 space-y-1">
                <div class="flex justify-between">
                  <span>开放端口范围:</span>
                  <span class="text-gray-300 font-mono">{{ n.port_range || '10000-65535' }}</span>
                </div>
                <div class="flex justify-between">
                  <span>防御/说明:</span>
                  <span class="text-gray-300 truncate max-w-[200px]">{{ n.notes || '高防网络节点' }}</span>
                </div>
              </div>

              <div class="pt-2 border-t border-gray-800/80 flex items-center justify-between text-xs">
                <span class="text-gray-500 text-[11px]">更新于: {{ formatTime(n.updated_at) }}</span>
                <div class="space-x-2">
                  <button @click="speedTestNode(n)" class="px-2.5 py-1 rounded bg-gray-800 hover:bg-gray-700 text-gray-300 transition">⚡ 测速</button>
                  <button @click="toggleNodePreferred(n)" class="px-2.5 py-1 rounded bg-blue-600/20 hover:bg-blue-600/30 text-blue-400 border border-blue-500/30 transition">
                    {{ n.is_preferred_node ? '★ 优选节点' : '☆ 设为优选' }}
                  </button>
                </div>
              </div>
            </div>
            <div v-if="!filteredNodes.length" class="col-span-2 py-12 text-center text-gray-500">
              暂无可用的网络节点。
            </div>
          </div>
        </section>

        <!-- ⚙️ FRPC 进程 VIEW -->
        <section v-if="activePage === 'frp'" class="space-y-6">
          <div class="flex items-center justify-between">
            <div>
              <h2 class="text-xl font-bold text-white">FRPC 进程守护</h2>
              <p class="text-xs text-gray-400 mt-1">托管本地 FRPC 客户端进程生命周期 · 动态配置渲染 · 实时输出日志</p>
            </div>
            <div class="flex items-center gap-3">
              <button @click="frpcAction('start')" class="px-3.5 py-1.5 rounded-xl bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs transition">启动守护</button>
              <button @click="frpcAction('restart')" class="px-3.5 py-1.5 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold text-xs transition">重启守护</button>
              <button @click="frpcAction('stop')" class="px-3.5 py-1.5 rounded-xl bg-red-600 hover:bg-red-500 text-white font-semibold text-xs transition">停止守护</button>
            </div>
          </div>

          <div class="glass-panel p-5 space-y-4">
            <h3 class="font-bold text-white">运行时状态</h3>
            <div class="grid grid-cols-4 gap-4">
              <div class="glass-card p-4">
                <div class="text-xs text-gray-400">进程状态</div>
                <div :class="['text-lg font-bold mt-1', frpcRuntime?.status === 'running' ? 'text-emerald-400' : 'text-amber-400']">
                  {{ frpcRuntime?.status === 'running' ? '守护运行中' : (frpcRuntime?.status || '已停止') }}
                </div>
              </div>
              <div class="glass-card p-4">
                <div class="text-xs text-gray-400">当前 PID</div>
                <div class="text-lg font-bold text-white font-mono mt-1">{{ frpcRuntime?.pid || '—' }}</div>
              </div>
              <div class="glass-card p-4">
                <div class="text-xs text-gray-400">客户端版本</div>
                <div class="text-sm font-semibold text-gray-300 mt-1 font-mono">{{ frpcRuntime?.version || 'frpc v0.50+' }}</div>
              </div>
              <div class="glass-card p-4">
                <div class="text-xs text-gray-400">健康诊断</div>
                <div class="text-sm font-semibold text-emerald-400 mt-1">{{ frpcRuntime?.health_reason || '就绪' }}</div>
              </div>
            </div>
          </div>

          <!-- 实时进程日志与配置面板 -->
          <div class="glass-panel p-5 space-y-4">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-3">
                <h3 class="font-bold text-white">FRPC 进程输出日志 (Stdout/Stderr)</h3>
                <span class="text-xs text-gray-500">最近 500 行</span>
              </div>
              <div class="flex items-center gap-2">
                <button @click="showConfig = !showConfig" class="px-3 py-1 rounded-lg bg-gray-800 hover:bg-gray-700 text-gray-300 text-xs font-medium border border-gray-700 transition">
                  {{ showConfig ? '隐藏配置' : '📄 查看 frpc.toml' }}
                </button>
                <button @click="fetchFrpcLogs" :disabled="logsLoading" class="px-3 py-1 rounded-lg bg-blue-600/20 hover:bg-blue-600/30 text-blue-400 border border-blue-500/30 text-xs font-semibold transition flex items-center gap-1.5">
                  <span :class="{ 'animate-spin': logsLoading }">🔄</span>
                  刷新日志
                </button>
              </div>
            </div>

            <!-- 折叠配置面板 -->
            <div v-if="showConfig" class="p-4 rounded-xl bg-gray-950 border border-gray-800 font-mono text-xs text-emerald-400 overflow-x-auto max-h-64 whitespace-pre-wrap animate-fade-in">
              <div class="text-gray-500 text-[10px] mb-2"># 动态渲染生效的 frpc.toml 配置文件内容：</div>
              {{ frpcConfig || '# 暂无生效配置' }}
            </div>

            <!-- 日志终端输出框 -->
            <div class="bg-black/90 p-4 rounded-xl border border-gray-800 font-mono text-xs text-gray-300 h-80 overflow-y-auto whitespace-pre-wrap leading-relaxed shadow-inner">
              {{ frpcLogs || '暂无进程标准输出日志。点击上方【刷新日志】或启动守护进程。' }}
            </div>
          </div>
        </section>

        <!-- ☁️ CLOUDFLARE DNS VIEW -->
        <section v-if="activePage === 'dns'" class="space-y-6">
          <div class="flex items-center justify-between">
            <div>
              <h2 class="text-xl font-bold text-white">Cloudflare DNS 解析管理</h2>
              <p class="text-xs text-gray-400 mt-1">
                当前托管主域名: <span class="text-blue-400 font-mono font-semibold">{{ settingsForm.cfZoneName || '未配置 Zone' }}</span> · 遵循资产归属防误删保护机制
              </p>
            </div>
            <div class="flex items-center gap-3">
              <button @click="fetchDNSRecords" :disabled="dnsLoading" class="px-3.5 py-2 rounded-xl bg-gray-800 hover:bg-gray-700 text-white font-semibold text-xs border border-gray-700 transition active:scale-95 flex items-center gap-1.5">
                <span :class="{ 'animate-spin': dnsLoading }">🔄</span>
                刷新解析列表
              </button>
              <button @click="dnsModalOpen = true" class="px-4 py-2 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold text-sm shadow-lg shadow-blue-600/30 transition active:scale-95 flex items-center gap-2">
                <span>➕</span> 新增解析记录
              </button>
            </div>
          </div>

          <!-- 防误删策略说明提示卡片 -->
          <div class="p-4 rounded-xl bg-blue-500/10 border border-blue-500/20 text-blue-300 text-xs flex items-center justify-between">
            <div class="flex items-center gap-2.5">
              <span class="text-base">🛡️</span>
              <div>
                <span class="font-bold">原生记录保护纪律：</span>
                原生及其他用途（如 1Panel、企业邮箱等）DNS 记录受保护锁定（只读）。只有标记为 <code class="text-emerald-400 bg-gray-900 px-1 py-0.5 rounded">ashan-frp managed</code> 的记录才允许编辑或删除；其他记录需先点击【认领】。
              </div>
            </div>
          </div>

          <!-- DNS Records Table -->
          <div class="glass-panel p-5">
            <div class="overflow-x-auto">
              <table class="w-full text-left text-sm">
                <thead>
                  <tr class="border-b border-gray-800 text-gray-400 text-xs uppercase">
                    <th class="py-3 px-4">类型</th>
                    <th class="py-3 px-4">解析名称 (Name)</th>
                    <th class="py-3 px-4">记录值 (Content)</th>
                    <th class="py-3 px-4">CDN 代理</th>
                    <th class="py-3 px-4">纳管归属</th>
                    <th class="py-3 px-4 text-right">操作</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-800/50">
                  <tr v-for="rec in dnsRecords" :key="rec.id" class="hover:bg-gray-800/30 transition">
                    <td class="py-3.5 px-4 font-mono font-bold text-xs">
                      <span class="px-2 py-0.5 rounded bg-gray-800 text-blue-400 border border-gray-700">
                        {{ rec.type }}
                      </span>
                    </td>
                    <td class="py-3.5 px-4 font-mono text-white text-xs font-semibold">
                      {{ rec.name }}
                    </td>
                    <td class="py-3.5 px-4 font-mono text-gray-300 text-xs truncate max-w-xs">
                      {{ rec.content }}
                    </td>
                    <td class="py-3.5 px-4">
                      <span :class="['px-2 py-0.5 rounded-full text-xs font-semibold', rec.proxied ? 'bg-amber-500/15 text-amber-400 border border-amber-500/30' : 'bg-gray-800 text-gray-400']">
                        {{ rec.proxied ? 'CDN 代理' : '仅 DNS' }}
                      </span>
                    </td>
                    <td class="py-3.5 px-4">
                      <span :class="['px-2.5 py-0.5 rounded-full text-xs font-semibold', getRecordOriginLabel(rec.comment).badgeClass]">
                        {{ getRecordOriginLabel(rec.comment).text }}
                      </span>
                    </td>
                    <td class="py-3.5 px-4 text-right space-x-2">
                      <button v-if="!getRecordOriginLabel(rec.comment).managed" @click="claimDNSRecord(rec)" class="text-xs px-2.5 py-1 rounded bg-emerald-600/20 hover:bg-emerald-600/30 text-emerald-400 border border-emerald-500/30 transition">
                        📌 认领纳管
                      </button>
                      <button v-if="getRecordOriginLabel(rec.comment).managed" @click="unclaimDNSRecord(rec)" class="text-xs px-2.5 py-1 rounded bg-gray-800 hover:bg-gray-700 text-gray-300 transition">
                        🔓 释放
                      </button>
                      <button v-if="getRecordOriginLabel(rec.comment).managed" @click="deleteDNSRecord(rec)" class="text-xs px-2.5 py-1 rounded bg-red-500/10 hover:bg-red-500/20 text-red-400 border border-red-500/20 transition">
                        🗑️ 删除
                      </button>
                    </td>
                  </tr>
                  <tr v-if="!dnsRecords.length">
                    <td colspan="6" class="py-8 text-center text-gray-500">
                      {{ dnsLoading ? '正在加载 Cloudflare DNS 解析记录...' : '暂无解析记录，请确保已在【设置中心】完成 Cloudflare API 绑定。' }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </section>

        <!-- ⚙️ SETTINGS VIEW -->
        <section v-if="activePage === 'settings'" class="space-y-6">
          <div class="flex items-center justify-between">
            <div>
              <h2 class="text-xl font-bold text-white">设置中心 (Settings)</h2>
              <p class="text-xs text-gray-400 mt-1">配置 ChmlFrp 隧道服务商、Cloudflare DNS 凭据与系统参数</p>
            </div>
            <button @click="saveSettings" class="px-5 py-2 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold text-sm transition shadow-lg shadow-blue-600/30">
              💾 保存设置
            </button>
          </div>

          <div class="grid grid-cols-2 gap-6">
            <!-- ChmlFrp 凭据配置 -->
            <div class="glass-panel p-5 space-y-4">
              <div class="flex items-center justify-between">
                <h3 class="font-bold text-white flex items-center gap-2">
                  <span>🔌</span> ChmlFrp 服务商凭据
                </h3>
                <span :class="['px-2 py-0.5 rounded text-xs font-bold', chmlfrpStatus.connected ? 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30' : 'bg-red-500/15 text-red-400 border border-red-500/30']">
                  {{ chmlfrpStatus.label }}
                </span>
              </div>
              <div v-if="!chmlfrpAuthLink">
                <label class="block text-xs font-semibold text-gray-400 mb-1">API Token / 用户密钥</label>
                <div class="flex gap-2">
                  <input v-model="settingsForm.chmlfrpToken" type="text" placeholder="如：wasf21479haHWON..." class="flex-1 px-3.5 py-2.5 rounded-xl bg-gray-900 border border-gray-700 text-sm text-white focus:outline-none focus:border-blue-500" />
                  <button @click="startChmlfrpAuth" type="button" class="px-4 py-2 bg-blue-600/20 text-blue-400 border border-blue-500/50 hover:bg-blue-600/40 rounded-xl text-sm font-semibold transition whitespace-nowrap">
                    🔗 自动登录 / 授权
                  </button>
                </div>
                <p class="text-[11px] text-gray-500 mt-1">可以直接填入 Token，也可以点击授权自动获取；已保存密钥会在此完整明文显示。</p>
              </div>
              <div v-else class="p-4 rounded-xl bg-blue-900/20 border border-blue-500/30 text-center space-y-3">
                <div class="text-sm text-blue-300 font-semibold">等待浏览器确认授权...</div>
                <div v-if="chmlfrpUserCode" class="p-2.5 rounded-lg bg-gray-900/90 border border-blue-500/40 inline-flex items-center gap-3">
                  <span class="text-xs text-gray-400">设备验证码:</span>
                  <span class="font-mono text-base font-bold text-amber-400 tracking-wider">{{ chmlfrpUserCode }}</span>
                  <button @click="copyText(chmlfrpUserCode)" type="button" class="text-xs px-2 py-1 bg-blue-600/30 hover:bg-blue-600/50 text-blue-300 rounded transition">复制</button>
                </div>
                <div>
                  <a :href="chmlfrpAuthLink" target="_blank" class="inline-block px-5 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-bold shadow-lg shadow-blue-500/20 transition">
                    点击前往浏览器确认授权 ↗
                  </a>
                </div>
                <div class="text-[11px] text-emerald-400/90 bg-emerald-500/10 border border-emerald-500/20 rounded-lg p-2 max-w-sm mx-auto text-left">
                  💡 <b>自动自愈开启</b>：首次授权后将永久自动刷新，无需重复登录；若登录需邮箱验证码，本地 MailFlow 邮箱系统已处于监控。
                </div>
                <div class="text-xs text-gray-400 flex items-center justify-center gap-2">
                  <svg class="animate-spin h-3 w-3 text-blue-400" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>
                  等待授权回调中，请勿刷新页面
                </div>
                <button @click="chmlfrpPolling = false; chmlfrpAuthLink = ''; chmlfrpUserCode = ''" class="text-xs text-gray-500 hover:text-gray-300 mt-2 block w-full">取消授权</button>
              </div>
            </div>

            <!-- Cloudflare API 凭据配置 -->
            <div class="glass-panel p-5 space-y-4">
              <div class="flex items-center justify-between">
                <h3 class="font-bold text-white flex items-center gap-2">
                  <span>☁️</span> Cloudflare 凭据
                </h3>
                <span :class="['px-2 py-0.5 rounded text-xs font-bold', cloudflareStatus.connected ? 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30' : 'bg-gray-800 text-gray-400']">
                  {{ cloudflareStatus.label }}
                </span>
              </div>
              <p class="text-[11px] text-amber-300 bg-amber-500/10 border border-amber-500/20 rounded-lg px-3 py-2">
                个人自托管模式：设置页默认直接显示完整密钥；禁止将密钥写入日志、审计详情或 URL。
              </p>
              <div class="space-y-3">
                <div>
                  <label class="block text-xs font-semibold text-gray-400 mb-1">API Token / Global API Key</label>
                  <input v-model="settingsForm.cfApiToken" type="text" autocomplete="off" placeholder="粘贴 Cloudflare API Token 或 Global API Key" class="w-full px-3.5 py-2.5 rounded-xl bg-gray-900 border border-gray-700 text-sm text-white focus:outline-none focus:border-blue-500" />
                </div>
                <div>
                  <label class="block text-xs font-semibold text-gray-400 mb-1">Cloudflare 登录邮箱（Global API Key 必填）</label>
                  <input v-model="settingsForm.cfAccountEmail" type="email" autocomplete="off" placeholder="如：user@example.com" class="w-full px-3.5 py-2.5 rounded-xl bg-gray-900 border border-gray-700 text-sm text-white focus:outline-none focus:border-blue-500" />
                </div>
                <div class="flex items-center justify-between pt-1">
                  <span v-if="settingsForm.cfAuthMethod" class="text-xs text-gray-400">
                    已识别：{{ settingsForm.cfAuthMethod === 'global_api_key' ? 'Global API Key' : 'Zone API Token' }}
                  </span>
                  <span v-else class="text-xs text-gray-500">自动探测凭据类型</span>
                  <button @click="configureCloudflare()" :disabled="cfVerifying" class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-xl text-xs font-semibold transition disabled:opacity-60 flex items-center gap-1.5 shadow-lg shadow-blue-600/20">
                    <span :class="{ 'animate-spin': cfVerifying }">⚡</span>
                    {{ cfVerifying ? '正在检测与保存...' : '检测并保存' }}
                  </button>
                </div>
                <div v-if="cfZones.length" class="p-3 rounded-xl bg-blue-950/40 border border-blue-500/30 space-y-2">
                  <div class="text-xs text-blue-300 font-semibold">检测到多个可用 Zone，请选择要绑定的主域名：</div>
                  <div class="flex gap-2">
                    <select v-model="settingsForm.cfSelectedZoneId" class="form-select flex-1 text-xs">
                      <option v-for="z in cfZones" :key="z.id" :value="z.id">{{ z.name }} ({{ z.id }})</option>
                    </select>
                    <button @click="configureCloudflare(settingsForm.cfSelectedZoneId)" :disabled="!settingsForm.cfSelectedZoneId || cfVerifying" class="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-semibold transition disabled:opacity-60">
                      保存所选域名
                    </button>
                  </div>
                </div>
                <div v-else-if="settingsForm.cfZoneName" class="text-xs text-gray-400 flex items-center gap-1.5 pt-1">
                  <span>已绑定主域名:</span>
                  <span class="text-white font-semibold font-mono">{{ settingsForm.cfZoneName }}</span>
                  <span class="text-gray-500 font-mono">({{ settingsForm.cfSelectedZoneId }})</span>
                </div>
              </div>
            </div>
          </div>
        </section>
      </main>
    </div>

    <!-- ZERO-FLASH VUE MODAL: 控制台映射弹窗 -->
    <div v-if="controlModalOpen" class="fixed inset-0 z-50 bg-black/85 flex items-center justify-center p-4">
      <div class="glass-panel max-w-lg w-full p-6 space-y-5 animate-fade-in border border-gray-700 shadow-2xl">
        <div class="flex items-center justify-between border-b border-gray-800 pb-3">
          <h3 class="font-bold text-white text-lg">{{ controlForm.id ? '编辑穿透映射' : '新增穿透映射' }}</h3>
          <button @click="controlModalOpen = false" class="text-gray-400 hover:text-white">✕</button>
        </div>
        <form @submit.prevent="submitControlModal" class="space-y-4">
          <div>
            <label class="block text-xs font-semibold text-gray-400 mb-1">服务名称</label>
            <input v-model="controlForm.name" placeholder="如：NAS 控制台" required class="w-full px-3.5 py-2.5 rounded-xl bg-gray-900 border border-gray-700 text-sm text-white focus:outline-none focus:border-blue-500" />
          </div>
          
          <div>
            <label class="block text-xs font-semibold text-gray-400 mb-1">穿透协议</label>
            <select v-model="controlForm.protocol" class="form-select">
              <optgroup label="Web 站点代理">
                <option value="https">🔒 HTTPS (加密 SSL 证书)</option>
                <option value="http">🌐 HTTP (自定义域名映射)</option>
              </optgroup>
              <optgroup label="端口与数据包代理">
                <option value="tcp">⚡ TCP (基础端口转发)</option>
                <option value="udp">📡 UDP (数据包高效代理)</option>
              </optgroup>
            </select>
          </div>

          <div v-if="['http', 'https'].includes(controlForm.protocol)">
            <label class="block text-xs font-semibold text-gray-400 mb-1">域名前缀 (Subdomain)</label>
            <input v-model="controlForm.subdomain" placeholder="如：nas" required class="w-full px-3.5 py-2.5 rounded-xl bg-gray-900 border border-gray-700 text-sm text-white focus:outline-none focus:border-blue-500" />
          </div>

          <!-- 动态端点预览提示 -->
          <div class="p-3 rounded-xl bg-gray-900/90 border border-gray-800 text-xs font-mono space-y-1">
            <div class="text-gray-500 text-[10px] uppercase font-sans font-bold">端点预览 (Endpoint Preview)</div>
            <div v-if="['http', 'https'].includes(controlForm.protocol)" class="text-blue-400 font-semibold">
              {{ controlForm.protocol }}://{{ controlForm.subdomain || 'prefix' }}.{{ settingsForm.cfZoneName || 'your-domain.com' }}
            </div>
            <div v-else class="text-emerald-400 font-semibold">
              [节点服务器]:{{ controlForm.remotePort || 40022 }} ➔ {{ controlForm.localIp || '127.0.0.1' }}:{{ controlForm.localPort }}
            </div>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-xs font-semibold text-gray-400 mb-1">内网 IP</label>
              <input v-model="controlForm.localIp" placeholder="127.0.0.1" required class="w-full px-3.5 py-2.5 rounded-xl bg-gray-900 border border-gray-700 text-sm text-white focus:outline-none focus:border-blue-500" />
            </div>
            <div>
              <label class="block text-xs font-semibold text-gray-400 mb-1">内网端口</label>
              <input v-model.number="controlForm.localPort" type="number" placeholder="80" required class="w-full px-3.5 py-2.5 rounded-xl bg-gray-900 border border-gray-700 text-sm text-white focus:outline-none focus:border-blue-500" />
            </div>
          </div>

          <div v-if="['tcp','udp'].includes(controlForm.protocol)">
            <label class="block text-xs font-semibold text-gray-400 mb-1">远程监听端口 (Remote Port)</label>
            <input v-model.number="controlForm.remotePort" type="number" placeholder="40022" required class="w-full px-3.5 py-2.5 rounded-xl bg-gray-900 border border-gray-700 text-sm text-white focus:outline-none focus:border-blue-500" />
          </div>

          <div>
            <label class="block text-xs font-semibold text-gray-400 mb-1">穿透节点</label>
            <select v-model="controlForm.nodeId" class="form-select">
              <option v-for="opt in nodeOptions" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </option>
            </select>
          </div>

          <div class="pt-2 flex justify-end gap-3">
            <button type="button" @click="controlModalOpen = false" class="px-4 py-2 rounded-xl bg-gray-800 text-gray-300 text-sm hover:bg-gray-700 transition">取消</button>
            <button type="submit" class="px-5 py-2 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold text-sm transition shadow-lg shadow-blue-600/30">保存并部署</button>
          </div>
        </form>
      </div>
    </div>

    <!-- ZERO-FLASH VUE MODAL: DNS 新增记录弹窗 -->
    <div v-if="dnsModalOpen" class="fixed inset-0 z-50 bg-black/85 flex items-center justify-center p-4">
      <div class="glass-panel max-w-lg w-full p-6 space-y-5 animate-fade-in border border-gray-700 shadow-2xl">
        <div class="flex items-center justify-between border-b border-gray-800 pb-3">
          <h3 class="font-bold text-white text-lg">新增 Cloudflare DNS 解析记录</h3>
          <button @click="dnsModalOpen = false" class="text-gray-400 hover:text-white">✕</button>
        </div>
        <form @submit.prevent="submitDNSModal" class="space-y-4">
          <div class="grid grid-cols-3 gap-3">
            <div>
              <label class="block text-xs font-semibold text-gray-400 mb-1">记录类型</label>
              <select v-model="dnsForm.type" class="form-select">
                <option value="A">A (IPv4)</option>
                <option value="AAAA">AAAA (IPv6)</option>
                <option value="CNAME">CNAME (别名)</option>
                <option value="TXT">TXT (文本)</option>
              </select>
            </div>
            <div class="col-span-2">
              <label class="block text-xs font-semibold text-gray-400 mb-1">解析名称 (Name)</label>
              <input v-model="dnsForm.name" placeholder="如：sub 或 sub.domain.com" required class="w-full px-3.5 py-2.5 rounded-xl bg-gray-900 border border-gray-700 text-sm text-white focus:outline-none focus:border-blue-500" />
            </div>
          </div>

          <div>
            <label class="block text-xs font-semibold text-gray-400 mb-1">记录内容 (Content / 目标地址)</label>
            <input v-model="dnsForm.content" placeholder="如：1.2.3.4 或 target.chmlfrp.cn" required class="w-full px-3.5 py-2.5 rounded-xl bg-gray-900 border border-gray-700 text-sm text-white focus:outline-none focus:border-blue-500" />
          </div>

          <div class="flex items-center justify-between pt-1">
            <label class="flex items-center gap-2 cursor-pointer text-xs text-gray-300 font-semibold">
              <input type="checkbox" v-model="dnsForm.proxied" class="rounded bg-gray-900 border-gray-700 text-blue-600 focus:ring-0" />
              开启 Cloudflare CDN 代理加速 (Proxied)
            </label>
            <span class="text-[11px] text-gray-500">TTL: 自动 (1)</span>
          </div>

          <div class="pt-2 flex justify-end gap-3">
            <button type="button" @click="dnsModalOpen = false" class="px-4 py-2 rounded-xl bg-gray-800 text-gray-300 text-sm hover:bg-gray-700 transition">取消</button>
            <button type="submit" class="px-5 py-2 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold text-sm transition shadow-lg shadow-blue-600/30">创建并纳管</button>
          </div>
        </form>
      </div>
    </div>

    <!-- 修改管理员密码弹窗 -->
    <div v-if="passwordModalOpen" class="fixed inset-0 z-50 bg-black/80 flex items-center justify-center p-4">
      <div class="glass-panel max-w-sm w-full p-6 space-y-4 border border-gray-700 shadow-2xl rounded-2xl bg-[#0b0f17]">
        <div class="flex items-center justify-between">
          <h3 class="font-bold text-white text-base flex items-center gap-2">
            <span>🔑</span> 修改管理员密码
          </h3>
          <button @click="passwordModalOpen = false" class="text-gray-400 hover:text-white text-sm">✕</button>
        </div>
        <p class="text-xs text-gray-400">密码由您完全自主设定并加密存储，无明文暴露，AI 亦无法通过命令行修改。</p>
        <div v-if="passwordError" class="p-3 rounded-lg bg-red-500/10 border border-red-500/30 text-red-400 text-xs">
          {{ passwordError }}
        </div>
        <form @submit.prevent="submitChangePassword" class="space-y-3">
          <div>
            <label class="block text-xs font-semibold text-gray-400 mb-1">当前密码</label>
            <input v-model="passwordForm.oldPassword" type="password" required placeholder="输入当前使用的密码" class="w-full px-3.5 py-2.5 rounded-xl bg-gray-900 border border-gray-700 text-sm text-white focus:outline-none focus:border-blue-500" />
          </div>
          <div>
            <label class="block text-xs font-semibold text-gray-400 mb-1">新密码</label>
            <input v-model="passwordForm.newPassword" type="password" required placeholder="输入新密码" class="w-full px-3.5 py-2.5 rounded-xl bg-gray-900 border border-gray-700 text-sm text-white focus:outline-none focus:border-blue-500" />
          </div>
          <div>
            <label class="block text-xs font-semibold text-gray-400 mb-1">确认新密码</label>
            <input v-model="passwordForm.confirmPassword" type="password" required placeholder="再次输入新密码" class="w-full px-3.5 py-2.5 rounded-xl bg-gray-900 border border-gray-700 text-sm text-white focus:outline-none focus:border-blue-500" />
          </div>
          <div class="pt-2 flex justify-end gap-2.5">
            <button type="button" @click="passwordModalOpen = false" class="px-4 py-2 rounded-xl bg-gray-800 text-gray-300 text-xs hover:bg-gray-700 transition">取消</button>
            <button type="submit" :disabled="passwordLoading" class="px-4 py-2 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold text-xs transition shadow-md shadow-blue-600/30">
              {{ passwordLoading ? '正在保存...' : '保存新密码' }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'

const API_BASE = '/api/v1'
const activePage = ref('control')
const loading = ref(false)
const notice = ref('')
const error = ref('')

const navItems = [
  { id: 'control', name: '总控制台', icon: '⚡' },
  { id: 'tunnels', name: '穿透规则 (Tunnels)', icon: '🚀' },
  { id: 'nodes', name: '网络节点 (Nodes)', icon: '🌐' },
  { id: 'frp', name: '进程守护 (Daemon)', icon: '⚙️' },
  { id: 'dns', name: 'Cloudflare DNS', icon: '☁️' }
]

const isAuthenticated = ref(false)
const currentAccount = ref(null)
const loginLoading = ref(false)
const loginError = ref('')
const loginForm = ref({ username: 'admin', password: '' })

const passwordModalOpen = ref(false)
const passwordLoading = ref(false)
const passwordError = ref('')
const passwordForm = ref({ oldPassword: '', newPassword: '', confirmPassword: '' })

const tunnels = ref([])
const nodes = ref([])
const frpcRuntime = ref(null)
const settings = ref({})
const matrix = ref([])
const matrixLoading = ref(false)
const matrixFilter = ref('')

const displayMatrix = computed(() => {
  const source = matrix.value.length ? matrix.value : tunnels.value
  if (!matrixFilter.value) return source
  const q = matrixFilter.value.toLowerCase().trim()
  return source.filter(m => 
    (m.name && m.name.toLowerCase().includes(q)) ||
    (m.full_domain && m.full_domain.toLowerCase().includes(q)) ||
    (m.chmlfrp?.node_name && m.chmlfrp.node_name.toLowerCase().includes(q)) ||
    (m.chmlfrp_node && m.chmlfrp_node.toLowerCase().includes(q)) ||
    (m.onepanel?.alias && m.onepanel.alias.toLowerCase().includes(q)) ||
    String(m.onepanel?.listen_port || m.local_port || '').includes(q)
  )
})
const filteredMatrix = displayMatrix

const getServiceBadge = (name) => {
  const map = {
    'drive': 'Nextcloud 云盘存储',
    'pw': 'Vaultwarden 密码保险库',
    'aiapi': 'New API 接口分发网关',
    'erpnext': 'ERPNext 16 企业系统',
    'unraid': 'Unraid 宿主控制台'
  }
  return map[name?.toLowerCase()] || '本地业务微服务'
}

const formatTarget = (target) => {
  if (!target) return '—'
  return target.replace('http://', '').replace('https://', '')
}

const formatDate = (dateStr) => {
  if (!dateStr) return '—'
  try {
    const d = new Date(dateStr)
    return d.toISOString().split('T')[0]
  } catch(e) {
    return dateStr
  }
}
const dnsRecords = ref([])
const dnsLoading = ref(false)
const dnsModalOpen = ref(false)
const dnsForm = ref({
  type: 'A',
  name: '',
  content: '',
  proxied: false,
  ttl: 1
})

const frpcLogs = ref('')
const frpcConfig = ref('')
const showConfig = ref(false)
const logsLoading = ref(false)

const isChmlFrpLoggedIn = ref(true)
const loginErrorMsg = ref('')

const settingsForm = ref({
  chmlfrpToken: '',
  cfApiToken: '',
  cfAccountEmail: '',
  cfAuthMethod: '',
  cfSelectedZoneId: '',
  cfZoneName: ''
})

const chmlfrpAuthLink = ref('')
const chmlfrpUserCode = ref('')
const chmlfrpPolling = ref(false)

const failoverStatus = ref({
  auto_failover_enabled: true,
  tracked_node: '北京多线',
  tracked_node_ip: '82.156.215.91',
  tracked_node_online: true,
  continuous_offline_min: 0,
  offline_threshold_min: 10,
  last_failover_log: '',
  last_optimize_log: '',
  candidate_best_node: '',
  candidate_best_latency: 0,
  candidate_best_speed_mbps: 0
})
const failoverTriggering = ref(false)
const cfZones = ref([])
const cfVerifying = ref(false)

const copyText = async (text) => {
  try {
    await navigator.clipboard.writeText(text)
    notice.value = '已复制设备码: ' + text
  } catch(e) {
    prompt('请手动复制:', text)
  }
}

const apiFetch = async (url, options = {}) => {
  const token = localStorage.getItem('ashan_frp_token')
  const headers = { ...(options.headers || {}) }
  if (token && !headers['Authorization']) {
    headers['Authorization'] = `Bearer ${token}`
  }
  const opts = {
    ...options,
    headers,
    credentials: 'include'
  }
  const resp = await fetch(url, opts)
  if (resp.status === 401 && !url.includes('/auth/login') && !url.includes('/auth/session')) {
    isAuthenticated.value = false
    currentAccount.value = null
  }
  return resp
}

const checkSession = async () => {
  try {
    const res = await apiFetch(`${API_BASE}/auth/session`).then(r => r.json())
    if (res.data?.authenticated) {
      isAuthenticated.value = true
      currentAccount.value = res.data.account
      return true
    } else {
      isAuthenticated.value = false
      currentAccount.value = null
      return false
    }
  } catch (e) {
    isAuthenticated.value = false
    return false
  }
}

const submitLogin = async () => {
  loginLoading.value = true
  loginError.value = ''
  try {
    const res = await fetch(`${API_BASE}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({
        username: loginForm.value.username,
        password: loginForm.value.password
      })
    }).then(r => r.json())
    if (res.error) throw new Error(res.error.message || '登录失败，请检查账号密码')
    
    // Store token in localStorage
    const token = res.data?.auth?.token
    if (token) {
      localStorage.setItem('ashan_frp_token', token)
    }
    
    isAuthenticated.value = true
    currentAccount.value = res.data?.account
    loginForm.value.password = ''
    notice.value = `管理员 ${currentAccount.value?.login_name || ''} 登录成功！`
    await fetchData()
  } catch (err) {
    loginError.value = err.message
  } finally {
    loginLoading.value = false
  }
}

const logout = async () => {
  try {
    await apiFetch(`${API_BASE}/auth/logout`, { method: 'POST' })
  } catch (err) {}
  localStorage.removeItem('ashan_frp_token')
  isAuthenticated.value = false
  currentAccount.value = null
  loginForm.value.password = ''
  notice.value = '已退出管理员会话'
}

const openPasswordModal = () => {
  passwordForm.value = { oldPassword: '', newPassword: '', confirmPassword: '' }
  passwordError.value = ''
  passwordModalOpen.value = true
}

const submitChangePassword = async () => {
  if (!passwordForm.value.oldPassword) {
    passwordError.value = '请输入当前密码'
    return
  }
  if (!passwordForm.value.newPassword) {
    passwordError.value = '请输入新密码'
    return
  }
  if (passwordForm.value.newPassword !== passwordForm.value.confirmPassword) {
    passwordError.value = '两次输入的新密码不一致'
    return
  }
  passwordLoading.value = true
  passwordError.value = ''
  try {
    const res = await apiFetch(`${API_BASE}/auth/password/change`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        old_password: passwordForm.value.oldPassword,
        new_password: passwordForm.value.newPassword
      })
    }).then(r => r.json())
    if (res.error) throw new Error(res.error.message || '修改密码失败')
    notice.value = '🎉 管理员密码修改成功！请牢记您的新密码。'
    passwordModalOpen.value = false
    passwordForm.value = { oldPassword: '', newPassword: '', confirmPassword: '' }
  } catch (err) {
    passwordError.value = err.message
  } finally {
    passwordLoading.value = false
  }
}

const startChmlfrpAuth = async () => {
  loading.value = true
  try {
    const res = await apiFetch(`${API_BASE}/settings/integrations/chmlfrp/oauth/start`, { method: 'POST' }).then(r => r.json())
    if (res.error) throw new Error(res.error.message)
    const data = res.data
    chmlfrpAuthLink.value = data.verification_uri_complete || data.verification_uri
    chmlfrpUserCode.value = data.user_code || ''
    chmlfrpPolling.value = true
    pollChmlfrpAuth(data.device_code)
  } catch(e) {
    error.value = '授权启动失败: ' + e.message
  } finally {
    loading.value = false
  }
}

const pollChmlfrpAuth = async (deviceCode) => {
  if (!chmlfrpPolling.value) return
  try {
    const res = await apiFetch(`${API_BASE}/settings/integrations/chmlfrp/oauth/poll`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ device_code: deviceCode })
    }).then(r => r.json())

    if (res.data?.status === 'success') {
      settingsForm.value.chmlfrpToken = res.data.token.access_token
      chmlfrpPolling.value = false
      chmlfrpAuthLink.value = ''
      chmlfrpUserCode.value = ''
      notice.value = '✅ ChmlFrp 授权成功！'
      saveSettings()
    } else if (res.data?.status === 'pending') {
      setTimeout(() => pollChmlfrpAuth(deviceCode), 3000)
    } else {
      throw new Error(res.error?.message || res.data?.error || '轮询失败')
    }
  } catch(e) {
    error.value = '授权轮询异常: ' + e.message
    chmlfrpPolling.value = false
  }
}

const configureCloudflare = async (zoneId = '') => {
  if (!settingsForm.value.cfApiToken) {
    error.value = '请先输入 Cloudflare API Token 或 Global API Key'
    return
  }
  if (!zoneId && cfZones.value.length > 1) {
    cfZones.value = []
    settingsForm.value.cfSelectedZoneId = ''
  }
  cfVerifying.value = true
  error.value = ''
  try {
    const res = await apiFetch(`${API_BASE}/settings/integrations/cloudflare/configure`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        secret: settingsForm.value.cfApiToken,
        email: settingsForm.value.cfAccountEmail,
        zone_id: zoneId || undefined
      })
    }).then(r => r.json())

    if (res.error) throw new Error(res.error.message)
    const data = res.data || {}
    settingsForm.value.cfAuthMethod = data.auth_method || ''
    settingsForm.value.cfAccountEmail = data.account_email || settingsForm.value.cfAccountEmail
    if (data.status === 'zone_selection_required') {
      cfZones.value = data.zones || []
      settingsForm.value.cfSelectedZoneId = ''
      notice.value = `检测到 ${cfZones.value.length} 个可用域名，请选择后自动验证并保存`
      return
    }
    if (data.status === 'saved') {
      settingsForm.value.cfApiToken = data.secret || settingsForm.value.cfApiToken
      settingsForm.value.cfSelectedZoneId = data.zone_id || ''
      settingsForm.value.cfZoneName = data.zone_name || ''
      cfZones.value = []
      notice.value = `Cloudflare 凭据已验证并保存：${data.zone_name}`
      await fetchData()
    }
  } catch(e) {
    error.value = 'Cloudflare 配置失败: ' + e.message
  } finally {
    cfVerifying.value = false
  }
}

const chmlfrpStatus = computed(() => {
  const integ = settings.value?.integrations?.chmlfrp
  const hasPass = integ?.has_password
  const user = integ?.username
  if (hasPass || user || isChmlFrpLoggedIn.value) {
    return { connected: true, label: '🟢 已连通', text: user ? `Token/账号: ${user}` : '凭据授权生效' }
  }
  return { connected: false, label: '🔴 未登录', text: '点击配置凭据' }
})

const cloudflareStatus = computed(() => {
  const integ = settings.value?.integrations?.cloudflare
  const hasToken = integ?.has_api_token
  const zone = integ?.zone_name
  if (hasToken || zone) {
    return { connected: true, label: '🟢 已就绪', text: zone || 'API Token 有效' }
  }
  return { connected: false, label: '⚪ 未配置', text: '点击绑定 Token' }
})

const tunnelFilter = ref('')
const webOnlyFilter = ref(false)

const controlModalOpen = ref(false)
const controlForm = ref({
  id: null,
  name: '',
  protocol: 'https',
  subdomain: '',
  localIp: '127.0.0.1',
  localPort: 80,
  remotePort: 40022,
  nodeId: ''
})

const filteredTunnels = computed(() => {
  if (!tunnelFilter.value) return tunnels.value
  const q = tunnelFilter.value.toLowerCase()
  return tunnels.value.filter(t => 
    (t.name || '').toLowerCase().includes(q) ||
    (t.full_domain || '').toLowerCase().includes(q) ||
    (t.protocol || '').toLowerCase().includes(q) ||
    (t.chmlfrp_node || '').toLowerCase().includes(q)
  )
})

const filteredNodes = computed(() => {
  if (!webOnlyFilter.value) return nodes.value
  return nodes.value.filter(n => n.web_supported)
})

const nodeOptions = computed(() => {
  if (!nodes.value.length) return [{ label: '暂无可用节点', value: '' }]
  return nodes.value.map(n => ({
    label: `${n.web_supported ? '🌐' : '⚡'} ${n.display_name || n.canonical_name || n.id} (📍 ${n.region || n.area || '通用'}) ${n.real_ip || n.endpoint_url ? `- IP: ${n.real_ip || n.endpoint_url}` : ''}`,
    value: n.id,
    webSupported: n.web_supported
  }))
})

const formatTime = (val) => {
  if (!val) return '—'
  const date = new Date(val)
  if (isNaN(date.getTime())) return String(val)
  return date.toLocaleString('zh-CN', { hour12: false })
}

const getRecordOriginLabel = (comment) => {
  if (!comment) return { text: '🔒 原生/其他保护', managed: false, badgeClass: 'bg-gray-800 text-gray-400 border border-gray-700' }
  const c = comment.toLowerCase().trim()
  if (c.startsWith('ashan-frp managed:')) {
    return { text: '⚡ ashan-frp 专属', managed: true, badgeClass: 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30' }
  }
  if (c.includes('1panel')) {
    return { text: '📦 1Panel (外部锁定)', managed: false, badgeClass: 'bg-blue-500/15 text-blue-400 border border-blue-500/30' }
  }
  return { text: '🔒 原生/其他保护', managed: false, badgeClass: 'bg-gray-800 text-gray-400 border border-gray-700' }
}

const fetchDNSRecords = async () => {
  dnsLoading.value = true
  try {
    const res = await apiFetch(`${API_BASE}/dns/records`).then(r => r.json())
    if (res.error) throw new Error(res.error.message)
    dnsRecords.value = res.data?.records || []
  } catch (e) {
    // silently allow if credentials not yet configured
  } finally {
    dnsLoading.value = false
  }
}

const claimDNSRecord = async (rec) => {
  try {
    const res = await apiFetch(`${API_BASE}/dns/records/${encodeURIComponent(rec.id)}/claim`, { method: 'POST' }).then(r => r.json())
    if (res.error) throw new Error(res.error.message)
    notice.value = `已成功将记录「${rec.name}」纳管为 ashan-frp 管理！`
    await fetchDNSRecords()
  } catch (e) {
    error.value = `认领失败: ${e.message}`
  }
}

const unclaimDNSRecord = async (rec) => {
  if (!confirm(`确认释放记录「${rec.name}」的纳管状态吗？释放后将变为原生受保护记录。`)) return
  try {
    const res = await apiFetch(`${API_BASE}/dns/records/${encodeURIComponent(rec.id)}/unclaim`, { method: 'POST' }).then(r => r.json())
    if (res.error) throw new Error(res.error.message)
    notice.value = `已释放记录「${rec.name}」的纳管状态`
    await fetchDNSRecords()
  } catch (e) {
    error.value = `释放认领失败: ${e.message}`
  }
}

const deleteDNSRecord = async (rec) => {
  if (!confirm(`确认删除 Cloudflare DNS 记录「${rec.name} (${rec.content})」吗？`)) return
  try {
    const res = await apiFetch(`${API_BASE}/dns/records/${encodeURIComponent(rec.id)}`, { method: 'DELETE' }).then(r => r.json())
    if (res.error) throw new Error(res.error.message)
    notice.value = `DNS 记录「${rec.name}」已成功删除`
    await fetchDNSRecords()
  } catch (e) {
    error.value = `删除失败: ${e.message}`
  }
}

const submitDNSModal = async () => {
  try {
    const res = await apiFetch(`${API_BASE}/dns/records`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        type: dnsForm.value.type,
        name: dnsForm.value.name,
        content: dnsForm.value.content,
        proxied: dnsForm.value.proxied,
        ttl: Number(dnsForm.value.ttl) || 1
      })
    }).then(r => r.json())
    if (res.error) throw new Error(res.error.message)
    notice.value = `DNS 记录「${dnsForm.value.name}」已成功创建并纳管！`
    dnsModalOpen.value = false
    await fetchDNSRecords()
  } catch (e) {
    error.value = `创建 DNS 记录失败: ${e.message}`
  }
}

const fetchFrpcLogs = async () => {
  logsLoading.value = true
  try {
    const res = await apiFetch(`${API_BASE}/frpc/logs`).then(r => r.json())
    if (res.data?.logs) {
      frpcLogs.value = res.data.logs
    } else {
      frpcLogs.value = '暂无进程标准输出日志。'
    }
  } catch (e) {
    frpcLogs.value = `获取日志失败: ${e.message}`
  } finally {
    logsLoading.value = false
  }
}

const syncMatrix = async () => {
  matrixLoading.value = true
  notice.value = '正在全链路同步 (1Panel + ChmlFrp + Cloudflare)...'
  try {
    const res = await apiFetch(`${API_BASE}/matrix/sync`, { method: 'POST' }).then(r => r.json())
    if (res.data?.matrix) {
      matrix.value = res.data.matrix
      notice.value = '🎉 全链路 (1Panel / ChmlFrp / Cloudflare) 状态已成功同步并对齐！'
      setTimeout(() => notice.value = '', 4000)
      await fetchData()
    } else if (res.error) {
      error.value = `同步失败: ${res.error.message}`
    }
  } catch (e) {
    error.value = `全链路同步失败: ${e.message}`
  } finally {
    matrixLoading.value = false
  }
}

const fetchFrpcConfig = async () => {
  try {
    const res = await apiFetch(`${API_BASE}/frpc/config`).then(r => r.json())
    frpcConfig.value = res.data?.config || '# 暂无生效配置'
  } catch (e) {
    frpcConfig.value = `# 获取配置失败: ${e.message}`
  }
}

const fetchData = async () => {
  loading.value = true
  error.value = ''
  try {
    const isAuthed = await checkSession()
    if (!isAuthed) {
      return
    }

    const [tRes, nRes, rRes, sRes, dRes, fRes, mRes] = await Promise.all([
      apiFetch(`${API_BASE}/tunnels`).then(r => r.json()).catch(() => ({ data: { tunnels: [] } })),
      apiFetch(`${API_BASE}/nodes`).then(r => r.json()).catch(() => ({ data: { nodes: [] } })),
      apiFetch(`${API_BASE}/frpc/runtime`).then(r => r.json()).catch(() => ({ data: {} })),
      apiFetch(`${API_BASE}/settings`).then(r => r.json()).catch(() => ({ data: {} })),
      apiFetch(`${API_BASE}/dns/records`).then(r => r.json()).catch(() => ({ data: { records: [] } })),
      apiFetch(`${API_BASE}/failover/status`).then(r => r.json()).catch(() => ({ data: {} })),
      apiFetch(`${API_BASE}/matrix`).then(r => r.json()).catch(() => ({ data: { matrix: [] } }))
    ])
    tunnels.value = tRes?.data?.tunnels || []
    nodes.value = nRes?.data?.nodes || []
    frpcRuntime.value = rRes?.data || null
    settings.value = sRes?.data || {}
    dnsRecords.value = dRes?.data?.records || []
    matrix.value = mRes?.data?.matrix || []
    if (fRes?.data?.tracked_node) {
      failoverStatus.value = fRes.data
    }

    if (settings.value?.integrations) {
      const i = settings.value.integrations
      settingsForm.value.chmlfrpToken = i.chmlfrp?.password || i.chmlfrp?.token || ''
      settingsForm.value.cfApiToken = i.cloudflare?.api_token || ''
      settingsForm.value.cfAccountEmail = i.cloudflare?.account_email || ''
      settingsForm.value.cfAuthMethod = i.cloudflare?.auth_method || ''
      settingsForm.value.cfSelectedZoneId = i.cloudflare?.zone_id || ''
      settingsForm.value.cfZoneName = i.cloudflare?.zone_name || ''
      if (i.chmlfrp?.has_password || i.chmlfrp?.username) {
        isChmlFrpLoggedIn.value = true
      }
    }
  } catch (err) {
    error.value = err.message || '获取数据失败'
  } finally {
    loading.value = false
  }
}

const saveSettings = async () => {
  loading.value = true
  error.value = ''
  try {
    const current = settings.value || {}
    const currentIntegrations = current.integrations || {}
    const payload = {
      general: current.general || {},
      sync: current.sync || {},
      queue: current.queue || {},
      frpc_runtime: current.frpc_runtime || {},
      integrations: {
        ...currentIntegrations,
        chmlfrp: {
          ...(currentIntegrations.chmlfrp || {}),
          username: currentIntegrations.chmlfrp?.username || settingsForm.value.chmlfrpToken,
          password: settingsForm.value.chmlfrpToken
        },
        cloudflare: {
          ...(currentIntegrations.cloudflare || {}),
          api_token: '',
          auth_method: settingsForm.value.cfAuthMethod,
          account_email: settingsForm.value.cfAccountEmail,
          zone_id: settingsForm.value.cfSelectedZoneId,
          zone_name: settingsForm.value.cfZoneName
        }
      }
    }
    const res = await apiFetch(`${API_BASE}/settings`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    }).then(r => r.json())

    if (res?.error) {
      error.value = res.error.message || '保存设置失败'
    } else {
      notice.value = '服务商凭据设置保存成功！'
      isChmlFrpLoggedIn.value = true
      await fetchData()
    }
  } catch (err) {
    error.value = `保存失败: ${err.message}`
  } finally {
    loading.value = false
  }
}

const openControlModal = (tunnel = null) => {
  if (tunnel) {
    const full = tunnels.value.find(t => t.id === tunnel.id) || tunnel
    controlForm.value = {
      id: full.id,
      name: full.name || full.project_name || '',
      protocol: full.protocol || 'https',
      subdomain: full.subdomain || '',
      localIp: full.local_ip || '127.0.0.1',
      localPort: full.local_port || 80,
      remotePort: full.remote_port || 40022,
      nodeId: full.node_id || (nodes.value[0]?.id || '')
    }
  } else {
    controlForm.value = {
      id: null,
      name: '',
      protocol: 'https',
      subdomain: '',
      localIp: '127.0.0.1',
      localPort: 80,
      remotePort: 40022,
      nodeId: nodes.value[0]?.id || ''
    }
  }
  controlModalOpen.value = true
}

const openTunnelModal = (tunnel = null) => openControlModal(tunnel)

const submitControlModal = async () => {
  try {
    const isEdit = Boolean(controlForm.value.id)
    const url = isEdit ? `${API_BASE}/tunnels/${encodeURIComponent(controlForm.value.id)}` : `${API_BASE}/tunnels`
    const method = isEdit ? 'PATCH' : 'POST'
    const body = JSON.stringify({
      name: controlForm.value.name,
      project_name: controlForm.value.name,
      protocol: controlForm.value.protocol,
      subdomain: controlForm.value.subdomain,
      local_ip: controlForm.value.localIp,
      local_port: controlForm.value.localPort,
      remote_port: controlForm.value.remotePort,
      node_id: controlForm.value.nodeId,
      desired_state: 'enabled'
    })

    const res = await apiFetch(url, { method, headers: { 'Content-Type': 'application/json' }, body }).then(r => r.json())
    if (res.error) throw new Error(res.error.message)

    const tunnelId = res.data?.id || controlForm.value.id
    controlModalOpen.value = false

    // 触发自动部署调度 (Auto-provision)
    if (tunnelId) {
      try {
        await apiFetch(`${API_BASE}/tunnels/${encodeURIComponent(tunnelId)}/provision`, { method: 'POST' })
        notice.value = `映射「${controlForm.value.name}」已保存并下发自动调度部署！`
      } catch (pe) {
        notice.value = `映射「${controlForm.value.name}」已保存，调度排队中`
      }
    } else {
      notice.value = `映射「${controlForm.value.name}」已成功${isEdit ? '修改' : '创建'}！`
    }
    await fetchData()
  } catch (err) {
    error.value = `保存失败：${err.message}`
  }
}

const provisionTunnel = async (t) => {
  try {
    const res = await apiFetch(`${API_BASE}/tunnels/${encodeURIComponent(t.id)}/provision`, { method: 'POST' }).then(r => r.json())
    if (res.error) throw new Error(res.error.message)
    notice.value = `已触发规则「${t.name || t.id}」的自动部署与调度对账！`
    await fetchData()
  } catch (err) {
    error.value = `部署触发失败：${err.message}`
  }
}

const deleteTunnel = async (t) => {
  if (!confirm(`确认要删除穿透规则「${t.name || t.id}」吗？`)) return
  try {
    await apiFetch(`${API_BASE}/tunnels/${encodeURIComponent(t.id)}`, { method: 'DELETE' })
    notice.value = `规则「${t.name || t.id}」已成功删除`
    await fetchData()
  } catch (err) {
    error.value = `删除失败：${err.message}`
  }
}

const toggleFailover = async (t) => {
  const next = !t.is_failover_pool
  try {
    await apiFetch(`${API_BASE}/tunnels/${encodeURIComponent(t.id)}/failover-pool`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ is_failover_pool: next, failover_priority: next ? 1 : 0 })
    })
    notice.value = next ? `已加入故障容灾优选` : `已移出优选库`
    await fetchData()
  } catch (err) {
    error.value = err.message
  }
}

const toggleAutoFailover = async () => {
  const next = !failoverStatus.value.auto_failover_enabled
  try {
    await apiFetch(`${API_BASE}/failover/settings`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        auto_failover_enabled: next,
        offline_threshold_min: failoverStatus.value.offline_threshold_min || 10,
        heartbeat_interval_sec: 30,
        weekly_optimize_enabled: true
      })
    })
    notice.value = next ? '已开启核心节点故障自愈与周度优选（10分钟离线自动选优迁移）' : '已暂停自动故障迁移'
    await fetchData()
  } catch (err) {
    error.value = err.message
  }
}

const triggerManualFailover = async () => {
  if (failoverTriggering.value) return
  failoverTriggering.value = true
  try {
    const res = await apiFetch(`${API_BASE}/failover/trigger`, { method: 'POST' }).then(r => r.json())
    if (res?.error) {
      error.value = `故障转移失败: ${res.error.message}`
    } else {
      notice.value = '已完成节点应急迁移！'
      await fetchData()
    }
  } catch (err) {
    error.value = err.message
  } finally {
    failoverTriggering.value = false
  }
}

const triggerWeeklyOptimize = async () => {
  if (failoverTriggering.value) return
  failoverTriggering.value = true
  try {
    const res = await apiFetch(`${API_BASE}/failover/optimize`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ force: false })
    }).then(r => r.json())
    if (res?.error) {
      error.value = `周度优选测速: ${res.error.message}`
    } else {
      notice.value = '周度优选测速与评估完成！'
      await fetchData()
    }
  } catch (err) {
    error.value = err.message
  } finally {
    failoverTriggering.value = false
  }
}

const toggleWebFilter = () => webOnlyFilter.value = !webOnlyFilter.value
const syncNodes = async () => {
  try {
    await apiFetch(`${API_BASE}/nodes/sync`, { method: 'POST' })
    notice.value = '节点同步已完成'
    await fetchData()
  } catch (err) {
    error.value = err.message
  }
}

const syncChmlFrpTunnels = async () => {
  loading.value = true
  error.value = ''
  try {
    const res = await apiFetch(`${API_BASE}/tunnels/sync-chmlfrp`, { method: 'POST' }).then(r => r.json())
    if (res?.error) {
      if (['CHMLFRP_NOT_LOGGED_IN', 'CREDENTIAL_NOT_CONFIGURED', 'UNAUTHORIZED', 'NOT_FOUND'].includes(res.error.code)) {
        isChmlFrpLoggedIn.value = false
        loginErrorMsg.value = res.error.message || '未配置或未登录 ChmlFrp 凭据，请在【设置中心】绑定 Token 或账号'
      } else {
        error.value = res.error.message || '同步失败'
      }
    } else {
      isChmlFrpLoggedIn.value = true
      notice.value = res?.data?.message || '已全量同步 ChmlFrp 线上所有穿透规则！'
      await fetchData()
    }
  } catch (err) {
    error.value = err.message || '同步请求失败'
  } finally {
    loading.value = false
  }
}

const speedTestNode = async (n) => {
  try {
    await apiFetch(`${API_BASE}/nodes/${encodeURIComponent(n.id)}/speedtest`, { method: 'POST' })
    notice.value = `节点 ${n.display_name} 测速完成`
    await fetchData()
  } catch (err) {
    error.value = err.message
  }
}

const toggleNodePreferred = async (n) => {
  const next = !n.is_preferred_node
  try {
    await apiFetch(`${API_BASE}/nodes/${encodeURIComponent(n.id)}/preferred-pool`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ is_preferred_node: next })
    })
    notice.value = next ? '节点已设为优选' : '节点已移出优选'
    await fetchData()
  } catch (err) {
    error.value = err.message
  }
}

const frpcAction = async (action) => {
  try {
    await apiFetch(`${API_BASE}/frpc/${action}`, { method: 'POST' })
    notice.value = `FRPC 守护进程 ${action} 指令已下发`
    await fetchData()
    if (activePage.value === 'frp') {
      setTimeout(fetchFrpcLogs, 1000)
    }
  } catch (err) {
    error.value = err.message
  }
}

watch(activePage, (page) => {
  if (page === 'dns') fetchDNSRecords()
  if (page === 'frp') {
    fetchFrpcLogs()
    fetchFrpcConfig()
  }
})

onMounted(() => {
  fetchData()
})
</script>
