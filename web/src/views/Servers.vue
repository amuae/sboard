<template>
  <div class="container-fluid mt-4">
    <div class="d-flex justify-content-between align-items-center mb-4">
      <h2><i class="bi bi-hdd-network"></i> 服务器管理</h2>
      <div>
        <button class="btn btn-warning me-2" @click="deployAll" :disabled="deploying">
          <span v-if="deploying" class="spinner-border spinner-border-sm me-1"></span>
          <i v-else class="bi bi-arrow-clockwise"></i> 全部部署
        </button>
        <button class="btn btn-primary" @click="openAddModal">
          <i class="bi bi-plus-lg"></i> 添加服务器
        </button>
      </div>
    </div>

    <div id="serversList" class="row g-3">
      <div 
        v-for="(server, index) in servers" 
        :key="server.id" 
        class="col-xl-3 col-lg-4 col-md-6"
        :draggable="isDragging"
        @dragstart="handleDragStart($event, index)"
        @dragover.prevent="handleDragOver($event, index)"
        @dragend="handleDragEnd"
        @drop="handleDrop($event, index)"
      >
        <div class="server-card" :class="{ 'dragging': dragIndex === index, 'drag-over': dragOverIndex === index }">
          <div 
            class="drag-handle"
            @mousedown="startDrag"
            @mouseup="endDrag"
            @mouseleave="endDrag"
            @touchstart="startDrag"
            @touchend="endDrag"
          >
            <i class="bi bi-grip-horizontal"></i>
          </div>
          <div class="server-header">
            <div>
              <div class="server-title">{{ server.name }}</div>
              <div class="server-host">
                <i class="bi bi-hdd"></i> {{ maskIP(server.host) || '等待 Agent 上报' }}
              </div>
            </div>
            <div class="d-flex flex-column align-items-end gap-1">
              <span :class="['badge', server.enabled ? 'bg-success' : 'bg-secondary']">
                {{ server.enabled ? '启用' : '禁用' }}
              </span>
              <!-- Agent 在线状态 -->
              <span :class="['badge', server.agent_online ? 'bg-primary' : 'bg-warning']"
                    :title="server.agent_online ? `在线 - ${server.agent_id}` : '等待连接'">
                <i :class="['bi', server.agent_online ? 'bi-wifi' : 'bi-wifi-off']"></i>
                {{ server.agent_online ? 'Agent 在线' : '未连接' }}
              </span>
            </div>
          </div>
          
          <!-- 实时监控数据 -->
          <div v-if="server.agent_online" class="monitor-panel mb-2">
            <!-- CPU 和内存 -->
            <div class="row g-1 mb-1">
              <div class="col-6">
                <div class="monitor-item">
                  <div class="d-flex justify-content-between align-items-center">
                    <span class="monitor-label">CPU</span>
                    <span class="monitor-value text-primary">{{ (server.cpu_usage || 0).toFixed(1) }}%</span>
                  </div>
                  <div class="progress" style="height: 3px;">
                    <div class="progress-bar bg-primary" :style="{ width: (server.cpu_usage || 0) + '%' }"></div>
                  </div>
                </div>
              </div>
              <div class="col-6">
                <div class="monitor-item">
                  <div class="d-flex justify-content-between align-items-center">
                    <span class="monitor-label">内存</span>
                    <span class="monitor-value text-success">{{ (server.mem_usage || 0).toFixed(1) }}%</span>
                  </div>
                  <div class="progress" style="height: 3px;">
                    <div class="progress-bar bg-success" :style="{ width: (server.mem_usage || 0) + '%' }"></div>
                  </div>
                </div>
              </div>
            </div>
            <!-- 网速 -->
            <div class="row g-1">
              <div class="col-6">
                <div class="speed-item">
                  <i class="bi bi-arrow-down text-info"></i>
                  <span class="speed-value">{{ formatSpeed(server.net_in || 0) }}</span>
                  <span class="traffic-value">{{ formatBytes(server.monthly_out || 0) }}</span>
                </div>
              </div>
              <div class="col-6">
                <div class="speed-item">
                  <i class="bi bi-arrow-up text-warning"></i>
                  <span class="speed-value">{{ formatSpeed(server.net_out || 0) }}</span>
                  <span class="traffic-value">{{ formatBytes(server.monthly_in || 0) }}</span>
                </div>
              </div>
            </div>
          </div>
          
          <!-- 离线时显示基本信息 -->
          <div v-else class="offline-info small mb-2">
            <div class="d-flex justify-content-between">
              <span class="text-muted">分类:</span>
              <span :class="getCategoryClass(server.category)">{{ getCategoryName(server.category) }}</span>
            </div>
            <div class="d-flex justify-content-between">
              <span class="text-muted">核心:</span>
              <span>{{ server.core_type }}</span>
            </div>
          </div>
          
          <div class="card-actions">
            <button class="btn btn-outline-primary" @click="openEditModal(server)" title="编辑">
              <i class="bi bi-pencil"></i>
            </button>
            <button class="btn btn-outline-success" @click="deployFolder(server)" :disabled="server.deployingFolder" title="部署">
              <span v-if="server.deployingFolder" class="spinner-border spinner-border-sm"></span>
              <i v-else class="bi bi-cloud-upload"></i>
            </button>
            <button class="btn btn-outline-danger" @click="confirmDelete(server)" title="删除">
              <i class="bi bi-trash"></i>
            </button>
          </div>
        </div>
      </div>
      
      <div v-if="servers.length === 0" class="col-12 text-center py-5">
        <p class="text-muted">暂无服务器</p>
      </div>
    </div>

    <!-- 添加/编辑服务器模态框 -->
    <div class="modal fade" id="serverModal" tabindex="-1" ref="serverModalEl">
      <div class="modal-dialog modal-dialog-centered modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ isEditing ? '编辑服务器' : '添加服务器' }}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <form @submit.prevent="saveServer">
              <!-- Agent 状态信息 (编辑时显示，放在最上面) -->
              <div class="card bg-light mb-3" v-if="isEditing">
                <div class="card-body py-2">
                  <div class="row small">
                    <div class="col-6">
                      <span class="text-muted">Agent 状态:</span>
                      <span :class="formData.agent_online ? 'text-success' : 'text-danger'" class="ms-2">
                        <i :class="formData.agent_online ? 'bi-wifi' : 'bi-wifi-off'"></i>
                        {{ formData.agent_online ? '在线' : '离线' }}
                      </span>
                    </div>
                    <div class="col-6" v-if="formData.agent_id">
                      <span class="text-muted">Agent ID:</span>
                      <span class="ms-2 font-monospace">{{ formData.agent_id }}</span>
                    </div>
                  </div>
                  <div class="mt-2 d-flex align-items-center gap-2" v-if="formData.agent_token">
                    <span class="text-muted small">部署命令:</span>
                    <code class="small text-break flex-grow-1" style="max-width: 300px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">{{ agentDeployCommand }}</code>
                    <button type="button" class="btn btn-sm btn-outline-primary" @click="copyDeployCommand" title="复制部署命令">
                      <i class="bi bi-clipboard"></i> 复制
                    </button>
                    <button type="button" class="btn btn-sm btn-outline-warning" @click="regenerateToken" :disabled="regeneratingToken" title="重新生成 Token">
                      <span v-if="regeneratingToken" class="spinner-border spinner-border-sm"></span>
                      <i v-else class="bi bi-arrow-clockwise"></i>
                    </button>
                  </div>
                </div>
              </div>

              <div class="row">
                <div class="col-md-4">
                  <div class="mb-3">
                    <label class="form-label">备注名称 <span class="text-danger">*</span></label>
                    <input type="text" class="form-control" v-model="formData.name" required placeholder="如: 香港HKT-01">
                  </div>
                </div>
                <div class="col-md-5">
                  <div class="mb-3">
                    <label class="form-label">节点域名 <small class="text-muted">(可选)</small></label>
                    <input type="text" class="form-control" v-model="formData.node_domain" placeholder="如: hk01.example.com">
                    <small class="text-muted">订阅使用此域名，留空则使用 IP</small>
                  </div>
                </div>
                <div class="col-md-3">
                  <div class="mb-3">
                    <label class="form-label">DNS 解析策略</label>
                    <select class="form-select" v-model="formData.dns_resolve">
                      <option value="none">不解析</option>
                      <option value="ipv4">仅 IPv4</option>
                      <option value="ipv6">仅 IPv6</option>
                    </select>
                  </div>
                </div>
              </div>
              <div class="row">
                <div class="col-md-4">
                  <div class="mb-3">
                    <label class="form-label">服务器分类 <span class="text-danger">*</span></label>
                    <select class="form-select" v-model="formData.category">
                      <option value="direct">线路</option>
                      <option value="relay">落地</option>
                      <option value="home">自用</option>
                    </select>
                  </div>
                </div>
                <div class="col-md-4">
                  <div class="mb-3">
                    <label class="form-label">核心类型 <span class="text-danger">*</span></label>
                    <select class="form-select" v-model="formData.core_type">
                      <option value="sing-box">sing-box</option>
                      <option value="mihomo">mihomo</option>
                    </select>
                  </div>
                </div>
                <div class="col-md-4">
                  <div class="mb-3">
                    <label class="form-label">节点名一</label>
                    <input type="text" class="form-control" v-model="formData.node_1" placeholder="节点标识">
                  </div>
                </div>
              </div>
              <div class="row">
                <div class="col-md-4">
                  <div class="mb-3">
                    <label class="form-label">节点名二</label>
                    <input type="text" class="form-control" v-model="formData.node_2" placeholder="节点标识">
                  </div>
                </div>
                <div class="col-md-4">
                  <div class="mb-3">
                    <label class="form-label">节点名三</label>
                    <input type="text" class="form-control" v-model="formData.node_3" placeholder="节点标识">
                  </div>
                </div>
                <div class="col-md-4">
                  <div class="mb-3">
                    <label class="form-label">状态</label>
                    <div class="form-check form-switch mt-2">
                      <input class="form-check-input" type="checkbox" v-model="formData.enabled" id="serverEnabled">
                      <label class="form-check-label" for="serverEnabled">{{ formData.enabled ? '启用' : '禁用' }}</label>
                    </div>
                  </div>
                </div>
              </div>
              
              <!-- 节点配置区域（仅编辑时显示） -->
              <div v-if="isEditing" class="mt-4 border-top pt-3">
                <h6 class="mb-3"><i class="bi bi-diagram-3"></i> 入站节点配置</h6>
                <div class="d-flex flex-wrap gap-2">
                  <button 
                    v-for="node in availableNodes" 
                    :key="node.id" 
                    type="button"
                    class="btn btn-sm"
                    :class="isNodeConfigured(node) ? 'btn-primary' : 'btn-outline-secondary'"
                    @click="openNodeConfigModal(node)"
                  >
                    {{ node.tag }}
                  </button>
                </div>
                <small class="text-muted d-block mt-2">点击节点按钮配置端口转发或落地出站</small>
              </div>
            </form>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">取消</button>
            <button type="button" class="btn btn-primary" @click="saveServer" :disabled="saving">
              <span v-if="saving" class="spinner-border spinner-border-sm me-1"></span>
              保存
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- 部署结果模态框 -->
    <div class="modal fade" id="deployModal" tabindex="-1" ref="deployModalEl">
      <div class="modal-dialog modal-dialog-scrollable modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">部署日志</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <pre id="deployOutput" style="white-space: pre-wrap; word-break: break-word; background: #f8f9fa; padding: 1rem; border-radius: 5px; max-height: 500px; overflow-y: auto;">{{ deployOutput }}</pre>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">关闭</button>
          </div>
        </div>
      </div>
    </div>

    <!-- 删除确认模态框 -->
    <div class="modal fade" id="deleteModal" tabindex="-1" ref="deleteModalEl">
      <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content border-0 shadow-lg">
          <div class="modal-header bg-danger text-white border-0">
            <h5 class="modal-title">
              <i class="bi bi-exclamation-triangle-fill me-2"></i>
              确认删除
            </h5>
            <button type="button" class="btn-close btn-close-white" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body py-4">
            <div class="text-center mb-3">
              <i class="bi bi-trash3 text-danger" style="font-size: 3rem;"></i>
            </div>
            <p class="text-center mb-2 fs-5">
              确定要删除服务器 <strong class="text-danger">"{{ deleteTarget?.name }}"</strong> 吗？
            </p>
            <p class="text-center text-muted small mb-0">
              <i class="bi bi-info-circle me-1"></i>此操作不可撤销！
            </p>
          </div>
          <div class="modal-footer border-0 justify-content-center pb-4">
            <button type="button" class="btn btn-secondary px-4" data-bs-dismiss="modal">
              <i class="bi bi-x-lg me-1"></i>取消
            </button>
            <button type="button" class="btn btn-danger px-4" @click="doDelete" :disabled="deleting">
              <span v-if="deleting" class="spinner-border spinner-border-sm me-1"></span>
              <i v-else class="bi bi-trash3 me-1"></i>确认删除
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- 节点配置模态框 -->
    <div class="modal fade" id="nodeConfigModal" tabindex="-1" ref="nodeConfigModalEl">
      <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title"><i class="bi bi-diagram-3"></i> 节点配置 - {{ selectedNode?.tag }}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <form @submit.prevent="saveNodeConfig">
              <!-- 端口转发配置 -->
              <div class="card mb-3">
                <div class="card-header d-flex justify-content-between align-items-center py-2">
                  <span><i class="bi bi-arrow-left-right"></i> 端口转发</span>
                  <div class="form-check form-switch mb-0">
                    <input class="form-check-input" type="checkbox" v-model="nodeConfigData.forward_enabled">
                  </div>
                </div>
                <div class="card-body" v-if="nodeConfigData.forward_enabled">
                  <small class="text-muted d-block mb-2">用于订阅地址替换（将节点地址替换为转发地址）</small>
                  <div class="row">
                    <div class="col-8">
                      <div class="mb-2">
                        <label class="form-label small">转发地址 (IP/域名)</label>
                        <input type="text" class="form-control form-control-sm" v-model="nodeConfigData.forward_host" placeholder="例: 1.2.3.4 或 forward.example.com">
                      </div>
                    </div>
                    <div class="col-4">
                      <div class="mb-2">
                        <label class="form-label small">转发端口</label>
                        <input type="number" class="form-control form-control-sm" v-model.number="nodeConfigData.forward_port" min="1" max="65535" placeholder="端口">
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- 落地出站配置 -->
              <div class="card">
                <div class="card-header d-flex justify-content-between align-items-center py-2">
                  <span><i class="bi bi-box-arrow-right"></i> 落地出站</span>
                  <div class="form-check form-switch mb-0">
                    <input class="form-check-input" type="checkbox" v-model="nodeConfigData.outbound_enabled">
                  </div>
                </div>
                <div class="card-body" v-if="nodeConfigData.outbound_enabled">
                  <small class="text-muted d-block mb-2">配置落地服务器出站（流量从此服务器转发到落地机）</small>
                  
                  <!-- 导入链接 -->
                  <div class="row mb-3">
                    <div class="col-12">
                      <label class="form-label small">导入节点链接 (支持 vmess/vless/trojan/ss/hysteria2)</label>
                      <div class="input-group input-group-sm">
                        <input type="text" class="form-control" v-model="importLink" placeholder="粘贴节点链接，如 vmess://... vless://... trojan://...">
                        <button class="btn btn-outline-primary" type="button" @click="parseImportLink">
                          <i class="bi bi-box-arrow-in-down"></i> 解析
                        </button>
                      </div>
                    </div>
                  </div>
                  
                  <div class="row">
                    <div class="col-4 mb-2">
                      <label class="form-label small">协议</label>
                      <select class="form-select form-select-sm" v-model="nodeConfigData.outbound_protocol">
                        <option value="shadowsocks">Shadowsocks</option>
                        <option value="trojan">Trojan</option>
                        <option value="socks5">SOCKS5</option>
                        <option value="anytls">AnyTLS</option>
                        <option value="vless">VLESS</option>
                        <option value="vmess">VMess</option>
                        <option value="hysteria2">Hysteria2</option>
                      </select>
                    </div>
                    <div class="col-5 mb-2">
                      <label class="form-label small">出站地址</label>
                      <input type="text" class="form-control form-control-sm" v-model="nodeConfigData.outbound_host" placeholder="落地服务器 IP/域名">
                    </div>
                    <div class="col-3 mb-2">
                      <label class="form-label small">端口</label>
                      <input type="number" class="form-control form-control-sm" v-model.number="nodeConfigData.outbound_port" min="1" max="65535">
                    </div>
                  </div>
                  <!-- Shadowsocks 配置 -->
                  <div class="row" v-if="nodeConfigData.outbound_protocol === 'shadowsocks'">
                    <div class="col-6 mb-2">
                      <label class="form-label small">加密方式</label>
                      <select class="form-select form-select-sm" v-model="nodeConfigData.outbound_method">
                        <option value="2022-blake3-aes-128-gcm">2022-blake3-aes-128-gcm</option>
                        <option value="2022-blake3-aes-256-gcm">2022-blake3-aes-256-gcm</option>
                        <option value="2022-blake3-chacha20-poly1305">2022-blake3-chacha20-poly1305</option>
                        <option value="aes-128-gcm">aes-128-gcm</option>
                        <option value="aes-256-gcm">aes-256-gcm</option>
                        <option value="chacha20-ietf-poly1305">chacha20-ietf-poly1305</option>
                      </select>
                    </div>
                    <div class="col-6 mb-2">
                      <label class="form-label small">密码</label>
                      <input type="text" class="form-control form-control-sm" v-model="nodeConfigData.outbound_password" placeholder="密码">
                    </div>
                  </div>
                  <!-- Trojan/AnyTLS 配置 -->
                  <div class="row" v-if="nodeConfigData.outbound_protocol === 'trojan' || nodeConfigData.outbound_protocol === 'anytls'">
                    <div class="col-6 mb-2">
                      <label class="form-label small">密码</label>
                      <input type="text" class="form-control form-control-sm" v-model="nodeConfigData.outbound_password" placeholder="密码">
                    </div>
                    <div class="col-6 mb-2">
                      <label class="form-label small">SNI (可选)</label>
                      <input type="text" class="form-control form-control-sm" v-model="nodeConfigData.outbound_sni" placeholder="TLS SNI">
                    </div>
                  </div>
                  <!-- SOCKS5 配置 -->
                  <div class="row" v-if="nodeConfigData.outbound_protocol === 'socks5'">
                    <div class="col-6 mb-2">
                      <label class="form-label small">用户名 (可选)</label>
                      <input type="text" class="form-control form-control-sm" v-model="nodeConfigData.outbound_username" placeholder="用户名">
                    </div>
                    <div class="col-6 mb-2">
                      <label class="form-label small">密码 (可选)</label>
                      <input type="text" class="form-control form-control-sm" v-model="nodeConfigData.outbound_password" placeholder="密码">
                    </div>
                  </div>
                  <!-- VLESS 配置 -->
                  <div v-if="nodeConfigData.outbound_protocol === 'vless'">
                    <div class="row">
                      <div class="col-8 mb-2">
                        <label class="form-label small">UUID</label>
                        <input type="text" class="form-control form-control-sm" v-model="nodeConfigData.outbound_uuid" placeholder="VLESS UUID">
                      </div>
                      <div class="col-4 mb-2">
                        <label class="form-label small">Flow (可选)</label>
                        <select class="form-select form-select-sm" v-model="nodeConfigData.outbound_flow">
                          <option value="">无</option>
                          <option value="xtls-rprx-vision">xtls-rprx-vision</option>
                        </select>
                      </div>
                    </div>
                    <div class="row">
                      <div class="col-4 mb-2">
                        <label class="form-label small">安全类型</label>
                        <select class="form-select form-select-sm" v-model="vlessTlsType">
                          <option value="none">无</option>
                          <option value="tls">TLS</option>
                          <option value="reality">Reality</option>
                        </select>
                      </div>
                      <div class="col-4 mb-2" v-if="nodeConfigData.outbound_tls || nodeConfigData.outbound_reality">
                        <label class="form-label small">SNI</label>
                        <input type="text" class="form-control form-control-sm" v-model="nodeConfigData.outbound_sni" placeholder="TLS SNI">
                      </div>
                      <div class="col-4 mb-2" v-if="nodeConfigData.outbound_tls || nodeConfigData.outbound_reality">
                        <label class="form-label small">Fingerprint</label>
                        <select class="form-select form-select-sm" v-model="nodeConfigData.outbound_fp">
                          <option value="chrome">chrome</option>
                          <option value="firefox">firefox</option>
                          <option value="safari">safari</option>
                          <option value="edge">edge</option>
                          <option value="random">random</option>
                        </select>
                      </div>
                    </div>
                    <!-- Reality 配置 -->
                    <div class="row" v-if="nodeConfigData.outbound_reality">
                      <div class="col-8 mb-2">
                        <label class="form-label small">Public Key</label>
                        <input type="text" class="form-control form-control-sm" v-model="nodeConfigData.outbound_pub_key" placeholder="Reality Public Key">
                      </div>
                      <div class="col-4 mb-2">
                        <label class="form-label small">Short ID</label>
                        <input type="text" class="form-control form-control-sm" v-model="nodeConfigData.outbound_short_id" placeholder="Short ID">
                      </div>
                    </div>
                  </div>
                  <!-- VMess 配置 -->
                  <div v-if="nodeConfigData.outbound_protocol === 'vmess'">
                    <div class="row">
                      <div class="col-6 mb-2">
                        <label class="form-label small">UUID</label>
                        <input type="text" class="form-control form-control-sm" v-model="nodeConfigData.outbound_uuid" placeholder="VMess UUID">
                      </div>
                      <div class="col-3 mb-2">
                        <label class="form-label small">加密方式</label>
                        <select class="form-select form-select-sm" v-model="nodeConfigData.outbound_security">
                          <option value="auto">auto</option>
                          <option value="aes-128-gcm">aes-128-gcm</option>
                          <option value="chacha20-poly1305">chacha20-poly1305</option>
                          <option value="none">none</option>
                          <option value="zero">zero</option>
                        </select>
                      </div>
                      <div class="col-3 mb-2">
                        <label class="form-label small">Alter ID</label>
                        <input type="number" class="form-control form-control-sm" v-model.number="nodeConfigData.outbound_alter_id" min="0" placeholder="0">
                      </div>
                    </div>
                    <div class="row">
                      <div class="col-4 mb-2">
                        <label class="form-label small d-flex align-items-center gap-2">
                          <input type="checkbox" class="form-check-input" v-model="nodeConfigData.outbound_tls"> 启用 TLS
                        </label>
                      </div>
                      <div class="col-8 mb-2" v-if="nodeConfigData.outbound_tls">
                        <label class="form-label small">SNI (可选)</label>
                        <input type="text" class="form-control form-control-sm" v-model="nodeConfigData.outbound_sni" placeholder="TLS SNI">
                      </div>
                    </div>
                    <!-- 传输层配置 -->
                    <div class="row">
                      <div class="col-4 mb-2">
                        <label class="form-label small">传输层</label>
                        <select class="form-select form-select-sm" v-model="nodeConfigData.outbound_network">
                          <option value="">tcp (默认)</option>
                          <option value="ws">WebSocket</option>
                          <option value="grpc">gRPC</option>
                          <option value="http">HTTP/2</option>
                        </select>
                      </div>
                      <div class="col-4 mb-2" v-if="nodeConfigData.outbound_network === 'ws'">
                        <label class="form-label small">WS Path</label>
                        <input type="text" class="form-control form-control-sm" v-model="nodeConfigData.outbound_ws_path" placeholder="/">
                      </div>
                      <div class="col-4 mb-2" v-if="nodeConfigData.outbound_network === 'ws'">
                        <label class="form-label small">WS Host</label>
                        <input type="text" class="form-control form-control-sm" v-model="nodeConfigData.outbound_ws_host" placeholder="可选">
                      </div>
                    </div>
                  </div>
                  <!-- Hysteria2 配置 -->
                  <div v-if="nodeConfigData.outbound_protocol === 'hysteria2'">
                    <div class="row">
                      <div class="col-6 mb-2">
                        <label class="form-label small">密码</label>
                        <input type="text" class="form-control form-control-sm" v-model="nodeConfigData.outbound_password" placeholder="Hysteria2 密码">
                      </div>
                      <div class="col-6 mb-2">
                        <label class="form-label small">SNI (可选)</label>
                        <input type="text" class="form-control form-control-sm" v-model="nodeConfigData.outbound_sni" placeholder="TLS SNI">
                      </div>
                    </div>
                    <div class="row">
                      <div class="col-4 mb-2">
                        <label class="form-label small">Obfs 类型 (可选)</label>
                        <select class="form-select form-select-sm" v-model="nodeConfigData.outbound_obfs">
                          <option value="">无</option>
                          <option value="salamander">salamander</option>
                        </select>
                      </div>
                      <div class="col-8 mb-2" v-if="nodeConfigData.outbound_obfs">
                        <label class="form-label small">Obfs 密码</label>
                        <input type="text" class="form-control form-control-sm" v-model="nodeConfigData.outbound_obfs_pwd" placeholder="Obfs 密码">
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </form>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">取消</button>
            <button type="button" class="btn btn-primary" @click="saveNodeConfig">保存</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, inject, computed } from 'vue'
import { Modal } from 'bootstrap'
import { getServers, createServer, updateServer, deleteServer, getNodes, deployServer, regenerateAgentToken, getServersStatus, reorderServers, getNodeConfigs, saveNodeConfig as saveNodeConfigApi, type Server, type Node, type ServerStatus, type NodeConfig } from '@/api'

const showToast = inject<(type: 'success' | 'error' | 'warning' | 'info', title: string, message: string) => void>('showToast')!

interface ServerWithState extends Server {
  deploying?: boolean
  deployingFolder?: boolean
}

const servers = ref<ServerWithState[]>([])
const availableNodes = ref<Node[]>([])
const loading = ref(false)

// 模态框
const serverModalEl = ref<HTMLElement | null>(null)
const deployModalEl = ref<HTMLElement | null>(null)
const deleteModalEl = ref<HTMLElement | null>(null)
const nodeConfigModalEl = ref<HTMLElement | null>(null)
let serverModal: Modal | null = null
let deployModal: Modal | null = null
let deleteModal: Modal | null = null
let nodeConfigModal: Modal | null = null

// 表单
const isEditing = ref(false)
const saving = ref(false)
const formData = ref(getDefaultFormData())

// 删除
const deleteTarget = ref<Server | null>(null)
const deleting = ref(false)

// 部署
const deploying = ref(false)
const deployOutput = ref('')

// Agent 相关
const regeneratingToken = ref(false)

// 拖拽排序相关
const isDragging = ref(false)
const dragIndex = ref<number | null>(null)
const dragOverIndex = ref<number | null>(null)
let dragTimer: number | null = null

function startDrag() {
  // 长按 200ms 后启用拖拽
  dragTimer = window.setTimeout(() => {
    isDragging.value = true
  }, 200)
}

function endDrag() {
  if (dragTimer) {
    clearTimeout(dragTimer)
    dragTimer = null
  }
}

function handleDragStart(e: DragEvent, index: number) {
  if (!isDragging.value) {
    e.preventDefault()
    return
  }
  dragIndex.value = index
  e.dataTransfer!.effectAllowed = 'move'
}

function handleDragOver(e: DragEvent, index: number) {
  if (!isDragging.value || dragIndex.value === null) return
  dragOverIndex.value = index
}

function handleDrop(e: DragEvent, index: number) {
  if (!isDragging.value || dragIndex.value === null || dragIndex.value === index) return
  
  // 交换位置
  const draggedServer = servers.value[dragIndex.value]
  servers.value.splice(dragIndex.value, 1)
  servers.value.splice(index, 0, draggedServer)
  
  // 保存新顺序到后端
  saveServerOrder()
}

function handleDragEnd() {
  isDragging.value = false
  dragIndex.value = null
  dragOverIndex.value = null
}

async function saveServerOrder() {
  try {
    const ids = servers.value.map(s => s.id)
    await reorderServers(ids)
  } catch (error) {
    showToast('error', '错误', '保存排序失败')
  }
}

// GitHub 脚本地址 (使用加速域名，万能入口自动检测系统)
const GITHUB_SCRIPT_URL = 'https://ghfast.top/https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-agent-auto.sh'

// 计算 Agent 部署命令
const agentDeployCommand = computed(() => {
  if (!formData.value.agent_token) return ''
  const panelUrl = window.location.origin
  const coreType = formData.value.core_type || 'sing-box'
  return `curl -fsSL ${GITHUB_SCRIPT_URL} | bash -s -- --token ${formData.value.agent_token} --panel ${panelUrl} --core ${coreType}`
})

// 节点配置
const selectedNode = ref<Node | null>(null)
const nodeConfigData = ref({
  listen_port: 0,
  forward_enabled: false,
  forward_host: '',
  forward_port: 0,
  outbound_enabled: false,
  outbound_protocol: '',
  outbound_host: '',
  outbound_port: 0,
  outbound_password: '',
  outbound_method: '',
  outbound_username: '',
  outbound_sni: '',
  // VLESS/VMess 配置
  outbound_uuid: '',
  outbound_flow: '',
  outbound_security: 'auto',
  outbound_alter_id: 0,
  outbound_tls: false,
  outbound_reality: false,
  outbound_pub_key: '',
  outbound_short_id: '',
  outbound_fp: 'chrome',
  // Hysteria2 配置
  outbound_obfs: '',
  outbound_obfs_pwd: '',
  // 传输层配置 (VMess/VLESS)
  outbound_network: '',
  outbound_ws_path: '',
  outbound_ws_host: ''
})

// 导入链接
const importLink = ref('')

// VLESS TLS 类型计算属性
const vlessTlsType = computed({
  get() {
    if (nodeConfigData.value.outbound_reality) return 'reality'
    if (nodeConfigData.value.outbound_tls) return 'tls'
    return 'none'
  },
  set(value: string) {
    nodeConfigData.value.outbound_tls = value === 'tls'
    nodeConfigData.value.outbound_reality = value === 'reality'
  }
})

// 解析导入链接
function parseImportLink() {
  const link = importLink.value.trim()
  if (!link) {
    showToast('warning', '提示', '请输入节点链接')
    return
  }

  try {
    // VMess 链接解析
    if (link.startsWith('vmess://')) {
      parseVmessLink(link)
    }
    // VLESS 链接解析
    else if (link.startsWith('vless://')) {
      parseVlessLink(link)
    }
    // Trojan 链接解析
    else if (link.startsWith('trojan://')) {
      parseTrojanLink(link)
    }
    // Shadowsocks 链接解析
    else if (link.startsWith('ss://')) {
      parseShadowsocksLink(link)
    }
    // Hysteria2 链接解析
    else if (link.startsWith('hysteria2://') || link.startsWith('hy2://')) {
      parseHysteria2Link(link)
    }
    else {
      showToast('error', '错误', '不支持的链接格式')
      return
    }
    
    showToast('success', '成功', '节点信息已解析')
    importLink.value = ''
  } catch (error: any) {
    showToast('error', '解析失败', error.message || '链接格式错误')
  }
}

// 解析 VMess 链接 (v2rayN 格式)
function parseVmessLink(link: string) {
  const base64 = link.replace('vmess://', '')
  const decoded = atob(base64)
  const config = JSON.parse(decoded)
  
  nodeConfigData.value.outbound_protocol = 'vmess'
  nodeConfigData.value.outbound_host = config.add || ''
  nodeConfigData.value.outbound_port = parseInt(config.port) || 443
  nodeConfigData.value.outbound_uuid = config.id || ''
  nodeConfigData.value.outbound_alter_id = parseInt(config.aid) || 0
  nodeConfigData.value.outbound_security = config.scy || 'auto'
  nodeConfigData.value.outbound_tls = config.tls === 'tls'
  nodeConfigData.value.outbound_sni = config.sni || config.host || ''
  
  // 传输层配置
  const network = config.net || 'tcp'
  nodeConfigData.value.outbound_network = network
  if (network === 'ws') {
    nodeConfigData.value.outbound_ws_path = config.path || '/'
    nodeConfigData.value.outbound_ws_host = config.host || ''
  }
}

// 解析 VLESS 链接
function parseVlessLink(link: string) {
  // vless://uuid@host:port?params#name
  const url = new URL(link)
  const params = new URLSearchParams(url.search)
  
  nodeConfigData.value.outbound_protocol = 'vless'
  nodeConfigData.value.outbound_uuid = url.username
  nodeConfigData.value.outbound_host = url.hostname
  nodeConfigData.value.outbound_port = parseInt(url.port) || 443
  nodeConfigData.value.outbound_flow = params.get('flow') || ''
  nodeConfigData.value.outbound_sni = params.get('sni') || params.get('serverName') || ''
  nodeConfigData.value.outbound_fp = params.get('fp') || 'chrome'
  
  const security = params.get('security') || params.get('type') || ''
  if (security === 'reality') {
    nodeConfigData.value.outbound_reality = true
    nodeConfigData.value.outbound_tls = false
    nodeConfigData.value.outbound_pub_key = params.get('pbk') || ''
    nodeConfigData.value.outbound_short_id = params.get('sid') || ''
  } else if (security === 'tls') {
    nodeConfigData.value.outbound_tls = true
    nodeConfigData.value.outbound_reality = false
  } else {
    nodeConfigData.value.outbound_tls = false
    nodeConfigData.value.outbound_reality = false
  }
}

// 解析 Trojan 链接
function parseTrojanLink(link: string) {
  // trojan://password@host:port?params#name
  const url = new URL(link)
  const params = new URLSearchParams(url.search)
  
  nodeConfigData.value.outbound_protocol = 'trojan'
  nodeConfigData.value.outbound_password = decodeURIComponent(url.username)
  nodeConfigData.value.outbound_host = url.hostname
  nodeConfigData.value.outbound_port = parseInt(url.port) || 443
  nodeConfigData.value.outbound_sni = params.get('sni') || params.get('peer') || ''
}

// 解析 Shadowsocks 链接
function parseShadowsocksLink(link: string) {
  // ss://base64(method:password)@host:port#name
  // 或 ss://base64(method:password@host:port)#name
  const withoutPrefix = link.replace('ss://', '')
  const hashIndex = withoutPrefix.indexOf('#')
  const mainPart = hashIndex > -1 ? withoutPrefix.substring(0, hashIndex) : withoutPrefix
  
  let method = '', password = '', host = '', port = 0
  
  if (mainPart.includes('@')) {
    // 新格式: base64(method:password)@host:port
    const [encoded, serverPart] = mainPart.split('@')
    const decoded = atob(encoded)
    const colonIndex = decoded.indexOf(':')
    method = decoded.substring(0, colonIndex)
    password = decoded.substring(colonIndex + 1)
    
    const [h, p] = serverPart.split(':')
    host = h
    port = parseInt(p) || 0
  } else {
    // 旧格式: base64(method:password@host:port)
    const decoded = atob(mainPart)
    const atIndex = decoded.lastIndexOf('@')
    const userInfo = decoded.substring(0, atIndex)
    const serverPart = decoded.substring(atIndex + 1)
    
    const colonIndex = userInfo.indexOf(':')
    method = userInfo.substring(0, colonIndex)
    password = userInfo.substring(colonIndex + 1)
    
    const [h, p] = serverPart.split(':')
    host = h
    port = parseInt(p) || 0
  }
  
  nodeConfigData.value.outbound_protocol = 'shadowsocks'
  nodeConfigData.value.outbound_host = host
  nodeConfigData.value.outbound_port = port
  nodeConfigData.value.outbound_method = method
  nodeConfigData.value.outbound_password = password
}

// 解析 Hysteria2 链接
function parseHysteria2Link(link: string) {
  // hysteria2://password@host:port?params#name
  // hy2://password@host:port?params#name
  const url = new URL(link.replace('hysteria2://', 'hy2://').replace('hy2://', 'http://'))
  const params = new URLSearchParams(url.search)
  
  nodeConfigData.value.outbound_protocol = 'hysteria2'
  nodeConfigData.value.outbound_password = decodeURIComponent(url.username)
  nodeConfigData.value.outbound_host = url.hostname
  nodeConfigData.value.outbound_port = parseInt(url.port) || 443
  nodeConfigData.value.outbound_sni = params.get('sni') || ''
  nodeConfigData.value.outbound_obfs = params.get('obfs') || ''
  nodeConfigData.value.outbound_obfs_pwd = params.get('obfs-password') || ''
}

function getDefaultFormData() {
  return {
    id: 0,
    name: '',
    host: '',
    node_domain: '',
    dns_resolve: 'none',
    category: 'direct',
    core_type: 'sing-box',
    node_1: '',
    node_2: '',
    node_3: '',
    enabled: true,
    agent_token: '',
    agent_id: '',
    agent_online: false
  }
}

// 定时刷新状态
let statusRefreshTimer: number | null = null

onMounted(async () => {
  await loadData()
  serverModal = new Modal(serverModalEl.value!)
  deployModal = new Modal(deployModalEl.value!)
  deleteModal = new Modal(deleteModalEl.value!)
  nodeConfigModal = new Modal(nodeConfigModalEl.value!)
  
  // 启动定时刷新状态（每2秒）
  startStatusRefresh()
})

onUnmounted(() => {
  // 清理定时器
  stopStatusRefresh()
})

function startStatusRefresh() {
  if (statusRefreshTimer) return
  statusRefreshTimer = window.setInterval(refreshStatus, 2000)
}

function stopStatusRefresh() {
  if (statusRefreshTimer) {
    clearInterval(statusRefreshTimer)
    statusRefreshTimer = null
  }
}

// 刷新服务器状态（不影响其他交互）
async function refreshStatus() {
  try {
    const res = await getServersStatus()
    const statuses = res.data.data.statuses || []
    
    // 只更新状态数据，不重新渲染整个列表
    statuses.forEach((status: ServerStatus) => {
      const server = servers.value.find(s => s.id === status.id)
      if (server) {
        server.agent_online = status.agent_online
        server.cpu_usage = status.cpu_usage
        server.mem_usage = status.mem_usage
        server.disk_usage = status.disk_usage
        server.net_in = status.net_in
        server.net_out = status.net_out
        server.monthly_in = status.monthly_in
        server.monthly_out = status.monthly_out
      }
    })
  } catch {
    // 静默失败，不影响用户操作
  }
}

// 隐藏 IP 后三段，只显示第一段
function maskIP(ip: string | undefined): string {
  if (!ip) return ''
  // 处理 IPv4
  if (ip.includes('.')) {
    const parts = ip.split('.')
    if (parts.length === 4) {
      return `${parts[0]}.*.*.*`
    }
  }
  // 处理 IPv6
  if (ip.includes(':')) {
    const parts = ip.split(':')
    if (parts.length >= 2) {
      return `${parts[0]}:${parts[1]}:****`
    }
  }
  return ip
}

// 格式化网速显示
function formatSpeed(bytesPerSec: number): string {
  if (bytesPerSec < 1024) return `${bytesPerSec} B/s`
  if (bytesPerSec < 1024 * 1024) return `${(bytesPerSec / 1024).toFixed(1)} KB/s`
  return `${(bytesPerSec / 1024 / 1024).toFixed(2)} MB/s`
}

// 格式化流量显示
function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(2)} MB`
  return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`
}

async function loadData() {
  loading.value = true
  try {
    const [serversRes, nodesRes] = await Promise.all([getServers(), getNodes()])
    servers.value = (serversRes.data.data.servers || []).map((s: Server) => ({
      ...s,
      deploying: false,
      deployingFolder: false
    }))
    availableNodes.value = nodesRes.data.data.nodes || []
  } catch (error) {
    showToast('error', '错误', '加载数据失败')
  } finally {
    loading.value = false
  }
}

function getCategoryName(category: string) {
  const names: Record<string, string> = {
    direct: '线路',
    relay: '落地',
    home: '自用'
  }
  return names[category] || category
}

function getCategoryClass(category: string) {
  const classes: Record<string, string> = {
    direct: 'text-primary',
    relay: 'text-success',
    home: 'text-warning'
  }
  return classes[category] || ''
}

function isNodeConfigured(node: Node) {
  // 检查节点是否已配置（这里需要根据实际数据结构判断）
  return false
}

function openAddModal() {
  isEditing.value = false
  formData.value = getDefaultFormData()
  serverModal?.show()
}

function openEditModal(server: Server) {
  isEditing.value = true
  formData.value = {
    id: server.id,
    name: server.name,
    host: server.host || '',
    node_domain: server.node_domain || '',
    dns_resolve: server.dns_resolve || 'none',
    category: server.category,
    core_type: server.core_type,
    node_1: server.node_1 || '',
    node_2: server.node_2 || '',
    node_3: server.node_3 || '',
    enabled: server.enabled === 1 || server.enabled === true,
    agent_token: server.agent_token || '',
    agent_id: server.agent_id || '',
    agent_online: server.agent_online || false
  }
  serverModal?.show()
}

async function saveServer() {
  if (!formData.value.name) {
    showToast('warning', '警告', '请填写备注名称')
    return
  }

  saving.value = true
  try {
    const data: any = {
      name: formData.value.name,
      node_domain: formData.value.node_domain,
      dns_resolve: formData.value.dns_resolve,
      category: formData.value.category,
      core_type: formData.value.core_type,
      node_1: formData.value.node_1,
      node_2: formData.value.node_2,
      node_3: formData.value.node_3,
      enabled: formData.value.enabled ? 1 : 0
    }

    if (isEditing.value) {
      await updateServer(formData.value.id, data)
      showToast('success', '成功', '服务器已更新')
      serverModal?.hide()
    } else {
      const res = await createServer(data)
      // 创建成功后，切换到编辑模式，显示部署命令
      const newServer = res.data.data
      formData.value.id = newServer.id
      formData.value.agent_token = newServer.agent_token
      isEditing.value = true
      showToast('success', '成功', '服务器已添加，请复制下方的部署命令')
    }
    
    await loadData()
  } catch (error: any) {
    showToast('error', '错误', error.response?.data?.error || '保存失败')
  } finally {
    saving.value = false
  }
}

function confirmDelete(server: Server) {
  deleteTarget.value = server
  deleteModal?.show()
}

async function doDelete() {
  if (!deleteTarget.value) return
  
  deleting.value = true
  try {
    await deleteServer(deleteTarget.value.id)
    showToast('success', '成功', '服务器已删除')
    deleteModal?.hide()
    await loadData()
  } catch (error: any) {
    showToast('error', '错误', error.response?.data?.error || '删除失败')
  } finally {
    deleting.value = false
  }
}

async function deployFolder(server: ServerWithState) {
  server.deployingFolder = true
  try {
    const res = await deployServer(server.id, 'folder')
    deployOutput.value = res.data.data?.output || '目录部署完成'
    deployModal?.show()
    showToast('success', '成功', `服务器 ${server.name} 目录部署完成`)
  } catch (error: any) {
    deployOutput.value = error.response?.data?.error || '目录部署失败'
    deployModal?.show()
    showToast('error', '错误', `服务器 ${server.name} 目录部署失败`)
  } finally {
    server.deployingFolder = false
  }
}

async function deployAll() {
  deploying.value = true
  deployOutput.value = '开始全部部署...\n'
  deployModal?.show()
  
  for (const server of servers.value) {
    if (!server.enabled) continue
    deployOutput.value += `\n正在部署: ${server.name}...\n`
    try {
      const res = await deployServer(server.id, 'folder')
      deployOutput.value += res.data.data?.output || '部署完成\n'
    } catch (error: any) {
      deployOutput.value += `部署失败: ${error.response?.data?.error || '未知错误'}\n`
    }
  }
  
  deployOutput.value += '\n全部部署完成！'
  deploying.value = false
  showToast('success', '成功', '全部部署完成')
}

async function openNodeConfigModal(node: Node) {
  selectedNode.value = node
  // 重置为默认值
  nodeConfigData.value = {
    listen_port: node.port,
    forward_enabled: false,
    forward_host: '',
    forward_port: 0,
    outbound_enabled: false,
    outbound_protocol: '',
    outbound_host: '',
    outbound_port: 0,
    outbound_password: '',
    outbound_method: '',
    outbound_username: '',
    outbound_sni: '',
    // VLESS/VMess 配置
    outbound_uuid: '',
    outbound_flow: '',
    outbound_security: 'auto',
    outbound_alter_id: 0,
    outbound_tls: false,
    outbound_reality: false,
    outbound_pub_key: '',
    outbound_short_id: '',
    outbound_fp: 'chrome',
    // Hysteria2 配置
    outbound_obfs: '',
    outbound_obfs_pwd: '',
    // 传输层配置 (VMess/VLESS)
    outbound_network: '',
    outbound_ws_path: '',
    outbound_ws_host: ''
  }
  
  // 尝试加载已有配置
  if (formData.value.id) {
    try {
      const res = await getNodeConfigs(formData.value.id)
      const existing = res.data.data?.find((c: NodeConfig) => c.node_id === node.id)
      if (existing) {
        nodeConfigData.value = {
          listen_port: existing.listen_port || node.port,
          forward_enabled: existing.forward_enabled || false,
          forward_host: existing.forward_host || '',
          forward_port: existing.forward_port || 0,
          outbound_enabled: existing.outbound_enabled || false,
          outbound_protocol: existing.outbound_protocol || '',
          outbound_host: existing.outbound_host || '',
          outbound_port: existing.outbound_port || 0,
          outbound_password: existing.outbound_password || '',
          outbound_method: existing.outbound_method || '',
          outbound_username: existing.outbound_username || '',
          outbound_sni: existing.outbound_sni || '',
          // VLESS/VMess 配置
          outbound_uuid: existing.outbound_uuid || '',
          outbound_flow: existing.outbound_flow || '',
          outbound_security: existing.outbound_security || 'auto',
          outbound_alter_id: existing.outbound_alter_id || 0,
          outbound_tls: existing.outbound_tls || false,
          outbound_reality: existing.outbound_reality || false,
          outbound_pub_key: existing.outbound_pub_key || '',
          outbound_short_id: existing.outbound_short_id || '',
          outbound_fp: existing.outbound_fp || 'chrome',
          // Hysteria2 配置
          outbound_obfs: existing.outbound_obfs || '',
          outbound_obfs_pwd: existing.outbound_obfs_pwd || '',
          // 传输层配置 (VMess/VLESS)
          outbound_network: existing.outbound_network || '',
          outbound_ws_path: existing.outbound_ws_path || '',
          outbound_ws_host: existing.outbound_ws_host || ''
        }
      }
    } catch (error) {
      console.error('加载节点配置失败:', error)
    }
  }
  
  nodeConfigModal?.show()
}

async function saveNodeConfig() {
  if (!formData.value.id || !selectedNode.value) {
    showToast('error', '错误', '请先保存服务器')
    return
  }
  
  try {
    await saveNodeConfigApi(formData.value.id, selectedNode.value.id, nodeConfigData.value)
    showToast('success', '成功', '节点配置已保存')
    nodeConfigModal?.hide()
  } catch (error: any) {
    console.error('保存节点配置失败:', error)
    showToast('error', '错误', error.response?.data?.message || '保存失败')
  }
}

// Agent 相关函数
async function copyAgentToken() {
  if (formData.value.agent_token) {
    try {
      await copyToClipboard(formData.value.agent_token)
      showToast('success', '成功', 'Agent Token 已复制到剪贴板')
    } catch (error) {
      showToast('error', '错误', '复制失败')
    }
  }
}

// 通用复制函数
async function copyToClipboard(text: string) {
  if (navigator.clipboard && window.isSecureContext) {
    await navigator.clipboard.writeText(text)
  } else {
    const textarea = document.createElement('textarea')
    textarea.value = text
    textarea.style.position = 'fixed'
    textarea.style.left = '-9999px'
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    document.body.removeChild(textarea)
  }
}

async function copyDeployCommand() {
  const command = agentDeployCommand.value
  if (!command) {
    showToast('warning', '警告', '部署命令为空，请确保服务器已保存')
    return
  }
  try {
    await copyToClipboard(command)
    showToast('success', '成功', '部署命令已复制到剪贴板')
  } catch (error) {
    showToast('error', '错误', '复制失败，请手动复制')
    console.error('Copy failed:', error)
  }
}

async function regenerateToken() {
  if (!formData.value.id) return
  
  regeneratingToken.value = true
  try {
    const res = await regenerateAgentToken(formData.value.id)
    formData.value.agent_token = res.data.data.agent_token
    showToast('success', '成功', 'Agent Token 已重新生成')
    await loadData() // 刷新列表
  } catch (error: any) {
    showToast('error', '错误', error.response?.data?.error || '重新生成失败')
  } finally {
    regeneratingToken.value = false
  }
}
</script>

<style scoped>
.server-card {
  background: white;
  border: none;
  border-radius: 16px;
  padding: 1.5rem;
  padding-top: 2rem;
  box-shadow: 0 4px 15px rgba(0, 0, 0, 0.08);
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
  overflow: hidden;
  animation: fadeInUp 0.5s ease-out;
}

.server-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 4px;
  background: linear-gradient(90deg, #667eea, #764ba2);
  transform: scaleX(0);
  transition: transform 0.3s ease;
}

.server-card:hover {
  transform: translateY(-5px);
  box-shadow: 0 12px 30px rgba(0, 0, 0, 0.12);
}

.server-card:hover::before {
  transform: scaleX(1);
}

.server-card.dragging {
  opacity: 0.5;
  transform: scale(0.95) !important;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.server-card.drag-over {
  border: 2px dashed #667eea;
  background: linear-gradient(135deg, #f8f9ff 0%, #f0f4ff 100%);
}

.drag-handle {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 24px;
  display: flex;
  justify-content: center;
  align-items: center;
  cursor: grab;
  color: #d1d5db;
  border-radius: 16px 16px 0 0;
  transition: all 0.2s ease;
  user-select: none;
}

.drag-handle:hover {
  background: linear-gradient(135deg, #f8f9ff 0%, #f0f4ff 100%);
  color: #667eea;
}

.drag-handle:active {
  cursor: grabbing;
  color: #764ba2;
}

.drag-handle i {
  font-size: 1rem;
}

.server-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 1rem;
}

.server-title {
  font-size: 1.25rem;
  font-weight: 600;
  color: #1f2937;
  margin-bottom: 0.5rem;
}

.server-host {
  color: #6b7280;
  font-size: 0.875rem;
  display: flex;
  align-items: center;
  gap: 0.25rem;
}

.card-actions {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 0.5rem;
  margin-top: 1rem;
}

.card-actions .btn {
  font-size: 0.875rem;
  padding: 0.5rem 0.75rem;
  border-radius: 8px;
  transition: all 0.3s ease;
}

.card-actions .btn:hover {
  transform: translateY(-2px);
}

/* 监控面板样式 */
.monitor-panel {
  background: linear-gradient(135deg, #f8fafc 0%, #f1f5f9 100%);
  border-radius: 8px;
  padding: 0.5rem;
}

.monitor-item {
  padding: 0.25rem 0;
}

.monitor-label {
  font-size: 0.7rem;
  color: #6b7280;
}

.monitor-value {
  font-size: 0.75rem;
  font-weight: 600;
}

.monitor-panel .progress {
  background-color: #e2e8f0;
  margin-top: 2px;
}

.speed-item {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  font-size: 0.875rem;
  padding: 0.25rem 0;
}

.speed-item i {
  font-size: 0.85rem;
}

.speed-value {
  font-weight: 500;
  font-family: 'Consolas', 'Monaco', monospace;
}

.traffic-value {
  font-size: 0.875rem;
  font-family: 'Consolas', 'Monaco', monospace;
  font-weight: 500;
  margin-left: auto;
}

.offline-info {
  background: #f8fafc;
  border-radius: 8px;
  padding: 0.5rem;
}

@keyframes fadeInUp {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
