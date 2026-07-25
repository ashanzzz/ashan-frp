<template>
  <div class="min-h-screen bg-[#070a0f] text-gray-100 flex flex-col font-sans">
    <!-- Header -->
    <header class="border-b border-gray-800/80 bg-[#0b0f17]/80 backdrop-blur-md sticky top-0 z-40 px-6 py-4 flex items-center justify-between">
      <div class="flex items-center gap-3">
        <div class="w-9 h-9 rounded-xl bg-gradient-to-tr from-blue-600 to-emerald-500 flex items-center justify-center font-bold text-white shadow-lg shadow-blue-500/20">
          AF
        </div>
        <div>
          <h1 class="text-lg font-bold tracking-tight text-white flex items-center gap-2">
            Ashan FRP
            <span class="text-xs px-2 py-0.5 rounded-full bg-blue-500/10 text-blue-400 border border-blue-500/20 font-medium">Vue 3 Console</span>
          </h1>
          <p class="text-xs text-gray-4 shadow-sm">高效穿透代理与多节点分布式调度控制台</p>
        </div>
      </div>

      <div class="flex items-center gap-3">
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
        <button 
          v-for="item in navItems" 
          :key="item.id"
          @click="activePage = item.id"
          :class="[
            'w-full text-left px-4 py-3 rounded-xl text-sm font-medium transition flex items-center gap-3',
            activePage === item.id 
              ? 'bg-blue-600/15 text-blue-400 border border-blue-500/30 font-semibold shadow-sm' 
              : 'text-gray-400 hover:bg-gray-800/50 hover:text-gray-200'
          ]"
        >
          <span class="text-base">{{ item.icon }}</span>
          {{ item.name }}
        </button>
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

          <!-- Table -->
          <div class="glass-panel p-5">
            <div class="overflow-x-auto">
              <table class="w-full text-left text-sm">
                <thead>
                  <tr class="border-b border-gray-800 text-gray-400 text-xs uppercase">
                    <th class="py-3 px-4">映射名称</th>
                    <th class="py-3 px-4">协议</th>
                    <th class="py-3 px-4">访问域名</th>
                    <th class="py-3 px-4">本地端口</th>
                    <th class="py-3 px-4">节点</th>
                    <th class="py-3 px-4">状态</th>
                    <th class="py-3 px-4 text-right">操作</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-800/50">
                  <tr v-for="t in tunnels" :key="t.id" class="hover:bg-gray-800/30 transition">
                    <td class="py-3.5 px-4 font-semibold text-white">{{ t.name || t.project_name }}</td>
                    <td class="py-3.5 px-4">
                      <span class="px-2 py-0.5 rounded text-xs font-semibold bg-gray-800 text-gray-300 border border-gray-700">
                        {{ (t.protocol || t.tunnel_type || 'TCP').toUpperCase() }}
                      </span>
                    </td>
                    <td class="py-3.5 px-4 text-blue-400 font-mono text-xs">{{ t.full_domain || '—' }}</td>
                    <td class="py-3.5 px-4 text-gray-300 font-mono text-xs">{{ t.local_ip || '127.0.0.1' }}:{{ t.local_port }}</td>
                    <td class="py-3.5 px-4 text-gray-300 text-xs">{{ t.chmlfrp_node || t.node_id || '自动分配' }}</td>
                    <td class="py-3.5 px-4">
                      <span :class="['px-2 py-0.5 rounded-full text-xs font-semibold', t.actual_state === 'running' || t.desired_state === 'enabled' ? 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30' : 'bg-gray-800 text-gray-400']">
                        {{ t.actual_state || t.desired_state || 'ready' }}
                      </span>
                    </td>
                    <td class="py-3.5 px-4 text-right space-x-2">
                      <button @click="openControlModal(t)" class="text-xs px-2.5 py-1 rounded bg-gray-800 hover:bg-gray-700 text-gray-300 transition">✏️ 编辑</button>
                      <button @click="deleteTunnel(t)" class="text-xs px-2.5 py-1 rounded bg-red-500/10 hover:bg-red-500/20 text-red-400 border border-red-500/20 transition">🗑️ 删除</button>
                    </td>
                  </tr>
                  <tr v-if="!tunnels.length">
                    <td colspan="7" class="py-8 text-center text-gray-500">暂无穿透映射记录，点击右上角【新增映射】开始配置。</td>
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
              <button @click="openTunnelModal()" class="px-4 py-2 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold text-sm shadow-lg shadow-blue-600/30 transition active:scale-95 flex items-center gap-2">
                <span>➕</span> 新建规则
              </button>
            </div>
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
              <button @click="syncNodes" class="px-4 py-2 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold text-sm transition">
                🔄 同步最新节点
              </button>
            </div>
          </div>

          <div class="glass-panel p-5">
            <div class="overflow-x-auto">
              <table class="w-full text-left text-sm">
                <thead>
                  <tr class="border-b border-gray-800 text-gray-400 text-xs uppercase">
                    <th class="py-3 px-4">节点名称 / 标识</th>
                    <th class="py-3 px-4">物理地区与线路 (Region)</th>
                    <th class="py-3 px-4">节点 IP 地址</th>
                    <th class="py-3 px-4">健康状态</th>
                    <th class="py-3 px-4">建站能力</th>
                    <th class="py-3 px-4">更新时间</th>
                    <th class="py-3 px-4 text-right">操作</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-800/50">
                  <tr v-for="n in filteredNodes" :key="n.id" class="hover:bg-gray-800/30 transition">
                    <td class="py-3.5 px-4 font-semibold text-white flex items-center gap-2">
                      {{ n.display_name || n.canonical_name || n.id }}
                      <span v-if="n.fangyu" class="px-1.5 py-0.5 rounded text-[10px] bg-amber-500/10 text-amber-400 border border-amber-500/20">🛡️ {{ n.fangyu }}</span>
                    </td>
                    <td class="py-3.5 px-4">
                      <span class="px-2.5 py-1 rounded-lg text-xs font-semibold bg-amber-500/10 text-amber-400 border border-amber-500/20 flex items-center gap-1 w-fit">
                        📍 {{ n.region || n.area || '通用网络' }}
                      </span>
                    </td>
                    <td class="py-3.5 px-4">
                      <span class="px-2.5 py-1 rounded-lg text-xs font-mono font-semibold bg-blue-500/10 text-blue-400 border border-blue-500/20 flex items-center gap-1 w-fit">
                        🌐 {{ n.real_ip || n.endpoint_url || '未获取 IP' }}
                      </span>
                    </td>
                    <td class="py-3.5 px-4">
                      <span :class="['px-2 py-0.5 rounded-full text-xs font-semibold', n.health_status === 'healthy' || n.status === 'active' ? 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30' : 'bg-gray-800 text-gray-400']">
                        {{ n.health_status || n.status || 'online' }}
                      </span>
                    </td>
                    <td class="py-3.5 px-4">
                      <span v-if="n.web_supported" class="px-2 py-0.5 rounded text-xs font-bold bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">🌐 支持建站</span>
                      <span v-else class="px-2 py-0.5 rounded text-xs font-bold bg-gray-800 text-gray-400">🚫 仅 TCP/UDP</span>
                    </td>
                    <td class="py-3.5 px-4 text-xs font-mono text-gray-400">
                      {{ formatTime(n.updated_at || n.created_at) }}
                    </td>
                    <td class="py-3.5 px-4 text-right space-x-2">
                      <button @click="speedTestNode(n)" class="text-xs px-2.5 py-1 rounded bg-gray-800 hover:bg-gray-700 text-gray-300 transition">⚡ 测速</button>
                      <button @click="toggleNodePreferred(n)" class="text-xs px-2.5 py-1 rounded bg-gray-800 hover:bg-gray-700 text-gray-300 transition">
                        {{ n.is_preferred_node ? '移出优选' : '加入优选' }}
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </section>

        <!-- ⚙️ FRPC 守护进程 VIEW -->
        <section v-if="activePage === 'frp'" class="space-y-6">
          <div class="flex items-center justify-between">
            <div>
              <h2 class="text-xl font-bold text-white">FRPC 守护进程</h2>
              <p class="text-xs text-gray-400 mt-1">守护本地 FRPC 客户端进程运行 · 配置生成与实时日志</p>
            </div>
            <div class="flex items-center gap-3">
              <button @click="frpcAction('start')" class="px-3.5 py-1.5 rounded-xl bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs transition">启动守护</button>
              <button @click="frpcAction('restart')" class="px-3.5 py-1.5 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold text-xs transition">重启守护</button>
              <button @click="frpcAction('stop')" class="px-3.5 py-1.5 rounded-xl bg-red-600 hover:bg-red-500 text-white font-semibold text-xs transition">停止守护</button>
            </div>
          </div>

          <div class="glass-panel p-5 space-y-4">
            <h3 class="font-bold text-white">运行时状态</h3>
            <div class="grid grid-cols-3 gap-4">
              <div class="glass-card p-4">
                <div class="text-xs text-gray-400">进程状态</div>
                <div class="text-lg font-bold text-emerald-400 mt-1">{{ frpcRuntime?.status || '守护中' }}</div>
              </div>
              <div class="glass-card p-4">
                <div class="text-xs text-gray-400">当前 PID</div>
                <div class="text-lg font-bold text-white font-mono mt-1">{{ frpcRuntime?.pid || '—' }}</div>
              </div>
              <div class="glass-card p-4">
                <div class="text-xs text-gray-400">最近更新时间</div>
                <div class="text-sm font-semibold text-gray-300 mt-1">{{ frpcRuntime?.updated_at || '就绪' }}</div>
              </div>
            </div>
          </div>
        </section>

        <!-- ☁️ CLOUDFLARE DNS VIEW -->
        <section v-if="activePage === 'dns'" class="space-y-6">
          <div>
            <h2 class="text-xl font-bold text-white">Cloudflare DNS</h2>
            <p class="text-xs text-gray-400 mt-1">按主域名分组查看 Cloudflare 解析与穿透关联状态</p>
          </div>
          <div class="glass-panel p-5">
            <p class="text-sm text-gray-400">数据来自隧道配置关联，无冗余代理编辑。</p>
          </div>
        </section>
      </main>
    </div>

    <!-- ZERO-FLASH VUE MODAL: 控制台映射弹窗 -->
    <div v-if="controlModalOpen" class="fixed inset-0 z-50 bg-black/75 backdrop-blur-md flex items-center justify-center p-4">
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

          <div>
            <label class="block text-xs font-semibold text-gray-400 mb-1">域名前缀</label>
            <input v-model="controlForm.subdomain" placeholder="如：nas" required class="w-full px-3.5 py-2.5 rounded-xl bg-gray-900 border border-gray-700 text-sm text-white focus:outline-none focus:border-blue-500" />
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-xs font-semibold text-gray-400 mb-1">内网 IP</label>
              <input v-model="controlForm.localIp" placeholder="192.168.1.1" required class="w-full px-3.5 py-2.5 rounded-xl bg-gray-900 border border-gray-700 text-sm text-white focus:outline-none focus:border-blue-500" />
            </div>
            <div>
              <label class="block text-xs font-semibold text-gray-400 mb-1">内网端口</label>
              <input v-model.number="controlForm.localPort" type="number" placeholder="80" required class="w-full px-3.5 py-2.5 rounded-xl bg-gray-900 border border-gray-700 text-sm text-white focus:outline-none focus:border-blue-500" />
            </div>
          </div>

          <div v-if="['tcp','udp'].includes(controlForm.protocol)">
            <label class="block text-xs font-semibold text-gray-400 mb-1">远程端口</label>
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
            <button type="submit" class="px-5 py-2 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold text-sm transition">保存并部署</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'

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

const tunnels = ref([])
const nodes = ref([])
const frpcRuntime = ref(null)

const tunnelFilter = ref('')
const webOnlyFilter = ref(false)

const controlModalOpen = ref(false)
const controlForm = ref({
  id: null,
  name: '',
  protocol: 'https',
  subdomain: '',
  localIp: '192.168.1.1',
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

const fetchData = async () => {
  loading.value = true
  error.value = ''
  try {
    const [tRes, nRes, rRes] = await Promise.all([
      fetch(`${API_BASE}/tunnels`).then(r => r.json()).catch(() => ({ data: { tunnels: [] } })),
      fetch(`${API_BASE}/nodes`).then(r => r.json()).catch(() => ({ data: { nodes: [] } })),
      fetch(`${API_BASE}/frpc/runtime`).then(r => r.json()).catch(() => ({ data: {} }))
    ])
    tunnels.value = tRes?.data?.tunnels || []
    nodes.value = nRes?.data?.nodes || []
    frpcRuntime.value = rRes?.data || null
  } catch (err) {
    error.value = err.message || '获取数据失败'
  } finally {
    loading.value = false
  }
}

const openControlModal = (tunnel = null) => {
  if (tunnel) {
    controlForm.value = {
      id: tunnel.id,
      name: tunnel.name || tunnel.project_name || '',
      protocol: tunnel.protocol || 'https',
      subdomain: tunnel.subdomain || '',
      localIp: tunnel.local_ip || '192.168.1.1',
      localPort: tunnel.local_port || 80,
      remotePort: tunnel.remote_port || 40022,
      nodeId: tunnel.node_id || (nodes.value[0]?.id || '')
    }
  } else {
    controlForm.value = {
      id: null,
      name: '',
      protocol: 'https',
      subdomain: '',
      localIp: '192.168.1.1',
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

    const res = await fetch(url, { method, headers: { 'Content-Type': 'application/json' }, body }).then(r => r.json())
    notice.value = `映射「${controlForm.value.name}」已成功${isEdit ? '修改' : '创建'}！`
    controlModalOpen.value = false
    await fetchData()
  } catch (err) {
    error.value = `保存失败：${err.message}`
  }
}

const deleteTunnel = async (t) => {
  if (!confirm(`确认要删除穿透规则「${t.name || t.id}」吗？`)) return
  try {
    await fetch(`${API_BASE}/tunnels/${encodeURIComponent(t.id)}`, { method: 'DELETE' })
    notice.value = `规则「${t.name || t.id}」已成功删除`
    await fetchData()
  } catch (err) {
    error.value = `删除失败：${err.message}`
  }
}

const toggleFailover = async (t) => {
  const next = !t.is_failover_pool
  try {
    await fetch(`${API_BASE}/tunnels/${encodeURIComponent(t.id)}/failover-pool`, {
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

const toggleWebFilter = () => webOnlyFilter.value = !webOnlyFilter.value
const syncNodes = async () => {
  try {
    await fetch(`${API_BASE}/nodes/sync`, { method: 'POST' })
    notice.value = '节点同步已提交队列'
    await fetchData()
  } catch (err) {
    error.value = err.message
  }
}

const speedTestNode = async (n) => {
  try {
    await fetch(`${API_BASE}/nodes/${encodeURIComponent(n.id)}/speedtest`, { method: 'POST' })
    notice.value = `节点 ${n.display_name} 测速完成`
    await fetchData()
  } catch (err) {
    error.value = err.message
  }
}

const toggleNodePreferred = async (n) => {
  const next = !n.is_preferred_node
  try {
    await fetch(`${API_BASE}/nodes/${encodeURIComponent(n.id)}/preferred-pool`, {
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
    await fetch(`${API_BASE}/frpc/${action}`, { method: 'POST' })
    notice.value = `FRPC 守护进程 ${action} 指令已下发`
    await fetchData()
  } catch (err) {
    error.value = err.message
  }
}

onMounted(() => {
  fetchData()
})
</script>
