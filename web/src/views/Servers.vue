<template>
  <div class="container-fluid mt-4">
    <div class="d-flex justify-content-between align-items-center mb-4">
      <h2>服务器</h2>
      <div>
        <button class="btn btn-info btn-sm me-2" @click="updateAllAgents" :disabled="updatingAgents">
          <span v-if="updatingAgents" class="spinner-border spinner-border-sm me-1"></span>
          <i v-else class="bi bi-cloud-download"></i> Agent更新
        </button>
        <button class="btn btn-warning btn-sm me-2" @click="deployAll" :disabled="deploying">
          <span v-if="deploying" class="spinner-border spinner-border-sm me-1"></span>
          <i v-else class="bi bi-arrow-clockwise"></i> 部署
        </button>
        <button class="btn btn-primary btn-sm" @click="openAddModal">
          <i class="bi bi-plus-lg"></i> 添加
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
              <div v-if="server.host_ipv6" class="server-host text-info">
                <i class="bi bi-hdd-stack"></i> {{ maskIP(server.host_ipv6) }}
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
                  <div class="small mt-1">
                    <div v-if="formData.host" class="mb-1">
                      <span class="text-muted">IPv4:</span>
                      <span class="ms-2">{{ formData.host }}</span>
                    </div>
                    <div v-if="formData.host_ipv6">
                      <span class="text-muted">IPv6:</span>
                      <span class="ms-2">{{ formData.host_ipv6 }}</span>
                    </div>
                  </div>
                  <div class="mt-2" v-if="formData.agent_token">
                    <div class="d-flex align-items-center mb-1">
                      <span class="text-muted small">部署命令:</span>
                      <span class="ms-2 text-muted small">(点击复制)</span>
                    </div>
                    <div 
                      class="deploy-command-box"
                      @click="copyDeployCommand"
                      role="button"
                      tabindex="0"
                      title="点击复制部署命令"
                    >
                      <code class="deploy-command-text">{{ agentDeployCommand }}</code>
                      <i class="bi bi-clipboard copy-icon"></i>
                    </div>
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
                  </div>
                </div>
                <div class="col-md-3">
                  <div class="mb-3">
                    <label class="form-label">订阅 IP 来源</label>
                    <select class="form-select" v-model="formData.dns_resolve">
                      <option value="none">节点域名</option>
                      <option value="ipv4">节点 IPv4</option>
                      <option value="ipv6">节点 IPv6</option>
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
                <!-- Tab 导航 -->
                <ul class="nav nav-tabs mb-3" role="tablist">
                  <li class="nav-item" role="presentation">
                    <button 
                      class="nav-link" 
                      :class="{ active: serverConfigTab === 'forward' }"
                      @click="serverConfigTab = 'forward'"
                      type="button"
                    >
                      <i class="bi bi-arrow-left-right"></i> 端口转发
                    </button>
                  </li>
                  <li class="nav-item" role="presentation">
                    <button 
                      class="nav-link" 
                      :class="{ active: serverConfigTab === 'outbound' }"
                      @click="serverConfigTab = 'outbound'"
                      type="button"
                    >
                      <i class="bi bi-box-arrow-right"></i> 落地出站
                      <span v-if="serverOutbounds.length > 0" class="badge bg-primary ms-1">{{ serverOutbounds.length }}</span>
                    </button>
                  </li>
                </ul>

                <!-- 端口转发 Tab 内容 -->
                <div v-if="serverConfigTab === 'forward'">
                  <p class="text-muted small mb-3">配置入站节点的端口转发（用于订阅地址替换）</p>
                  <div class="d-flex flex-wrap gap-2">
                    <button 
                      v-for="node in availableNodes" 
                      :key="node.id" 
                      type="button"
                      class="btn btn-sm position-relative"
                      :class="isForwardEnabled(node) ? 'btn-success' : 'btn-outline-secondary'"
                      @click="openNodeConfigModal(node)"
                    >
                      {{ node.tag }}
                      <span 
                        v-if="isForwardEnabled(node)" 
                        class="position-absolute translate-middle badge rounded-circle bg-success p-1"
                        style="top: 4px; right: -2px; width: 8px; height: 8px;"
                      ></span>
                    </button>
                  </div>
                </div>

                <!-- 落地出站 Tab 内容 -->
                <div v-if="serverConfigTab === 'outbound'">
                  <div class="d-flex justify-content-between align-items-center mb-3">
                    <p class="text-muted small mb-0">配置落地服务器出站（最多10个，与用户额外UUID对应）</p>
                    <button 
                      class="btn btn-sm btn-primary" 
                      @click="openAddOutboundModal"
                      :disabled="serverOutbounds.length >= 10"
                    >
                      <i class="bi bi-plus-lg"></i> 添加出站
                    </button>
                  </div>
                  <div v-if="serverOutbounds.length === 0" class="text-center py-4 text-muted">
                    <i class="bi bi-box-arrow-right" style="font-size: 2rem;"></i>
                    <p class="mt-2 mb-0">暂无落地出站配置</p>
                  </div>
                  <div v-else class="d-flex flex-wrap gap-2">
                    <div 
                      v-for="outbound in serverOutbounds" 
                      :key="outbound.slot" 
                      class="outbound-card"
                      :class="{ 'outbound-disabled': !outbound.enabled }"
                    >
                      <div class="outbound-header">
                        <span class="outbound-slot">{{ outbound.slot }}</span>
                        <span class="outbound-name">{{ outbound.remark || '未命名' }}</span>
                        <div class="form-check form-switch form-switch-sm mb-0">
                          <input 
                            class="form-check-input" 
                            type="checkbox" 
                            :checked="outbound.enabled"
                            @change="toggleOutbound(outbound)"
                          >
                        </div>
                      </div>
                      <div class="outbound-info">
                        <small class="text-muted">{{ outbound.protocol }} | {{ outbound.host }}:{{ outbound.port }}</small>
                      </div>
                      <div class="outbound-actions">
                        <button class="btn btn-sm btn-outline-primary" @click="openEditOutboundModal(outbound)" title="编辑">
                          <i class="bi bi-pencil"></i>
                        </button>
                        <button class="btn btn-sm btn-outline-danger" @click="deleteOutbound(outbound)" title="删除">
                          <i class="bi bi-trash"></i>
                        </button>
                      </div>
                    </div>
                  </div>
                </div>
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

    <!-- 端口转发配置模态框 -->
    <div class="modal fade" id="nodeConfigModal" tabindex="-1" ref="nodeConfigModalEl">
      <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title"><i class="bi bi-arrow-left-right"></i> 端口转发配置 - {{ selectedNode?.tag }}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <form @submit.prevent="saveNodeConfig">
              <div class="mb-3">
                <div class="form-check form-switch">
                  <input class="form-check-input" type="checkbox" v-model="nodeConfigData.forward_enabled" id="forwardEnabled">
                  <label class="form-check-label" for="forwardEnabled">启用端口转发</label>
                </div>
              </div>
              <div v-if="nodeConfigData.forward_enabled">
                <small class="text-muted d-block mb-3">用于订阅地址替换（将节点地址替换为转发地址）</small>
                <div class="row">
                  <div class="col-8">
                    <div class="mb-3">
                      <label class="form-label">转发地址 (IP/域名)</label>
                      <input type="text" class="form-control" v-model="nodeConfigData.forward_host" placeholder="例: 1.2.3.4 或 forward.example.com">
                    </div>
                  </div>
                  <div class="col-4">
                    <div class="mb-3">
                      <label class="form-label">转发端口</label>
                      <input type="number" class="form-control" v-model.number="nodeConfigData.forward_port" min="1" max="65535" placeholder="端口">
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

    <!-- 落地出站配置模态框 -->
    <div class="modal fade" id="outboundModal" tabindex="-1" ref="outboundModalEl">
      <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">
              <i class="bi bi-box-arrow-right"></i> 
              {{ isEditingOutbound ? '编辑落地出站' : '添加落地出站' }}
            </h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <form @submit.prevent="saveOutbound">
              <!-- 备注 -->
              <div class="mb-3">
                <label class="form-label">备注 <span class="text-danger">*</span></label>
                <input type="text" class="form-control" v-model="outboundData.remark" placeholder="显示为按钮名称，如：美国落地">
              </div>

              <!-- 导入链接 -->
              <div class="mb-3">
                <label class="form-label small">导入节点链接 (支持 vmess/vless/trojan/ss/hysteria2/socks5)</label>
                <div class="input-group">
                  <input type="text" class="form-control" v-model="outboundImportLink" placeholder="粘贴节点链接，如 vmess://... vless://... trojan://... socks5://...">
                  <button class="btn btn-outline-primary" type="button" @click="parseOutboundImportLink">
                    <i class="bi bi-box-arrow-in-down"></i> 解析
                  </button>
                </div>
              </div>

              <div class="row">
                <div class="col-4 mb-3">
                  <label class="form-label">协议</label>
                  <select class="form-select" v-model="outboundData.protocol">
                    <option value="shadowsocks">Shadowsocks</option>
                    <option value="trojan">Trojan</option>
                    <option value="socks5">SOCKS5</option>
                    <option value="anytls">AnyTLS</option>
                    <option value="vless">VLESS</option>
                    <option value="vmess">VMess</option>
                    <option value="hysteria2">Hysteria2</option>
                  </select>
                </div>
                <div class="col-5 mb-3">
                  <label class="form-label">出站地址</label>
                  <input type="text" class="form-control" v-model="outboundData.host" placeholder="落地服务器 IP/域名">
                </div>
                <div class="col-3 mb-3">
                  <label class="form-label">端口</label>
                  <input type="number" class="form-control" v-model.number="outboundData.port" min="1" max="65535">
                </div>
              </div>

              <!-- Shadowsocks 配置 -->
              <div class="row" v-if="outboundData.protocol === 'shadowsocks'">
                <div class="col-6 mb-3">
                  <label class="form-label">加密方式</label>
                  <select class="form-select" v-model="outboundData.method">
                    <optgroup label="传统加密方式">
                      <option value="aes-128-gcm">aes-128-gcm</option>
                      <option value="aes-256-gcm">aes-256-gcm</option>
                      <option value="chacha20-ietf-poly1305">chacha20-ietf-poly1305</option>
                      <option value="xchacha20-ietf-poly1305">xchacha20-ietf-poly1305</option>
                    </optgroup>
                    <optgroup label="SS2022 加密方式">
                      <option value="2022-blake3-aes-128-gcm">2022-blake3-aes-128-gcm</option>
                      <option value="2022-blake3-aes-256-gcm">2022-blake3-aes-256-gcm</option>
                      <option value="2022-blake3-chacha20-poly1305">2022-blake3-chacha20-poly1305</option>
                    </optgroup>
                  </select>
                </div>
                <div class="col-6 mb-3">
                  <label class="form-label">密码</label>
                  <input type="text" class="form-control" v-model="outboundData.password" placeholder="密码">
                </div>
              </div>

              <!-- Trojan/AnyTLS 配置 -->
              <div class="row" v-if="outboundData.protocol === 'trojan' || outboundData.protocol === 'anytls'">
                <div class="col-6 mb-3">
                  <label class="form-label">密码</label>
                  <input type="text" class="form-control" v-model="outboundData.password" placeholder="密码">
                </div>
                <div class="col-6 mb-3">
                  <label class="form-label">SNI (可选)</label>
                  <input type="text" class="form-control" v-model="outboundData.sni" placeholder="TLS SNI">
                </div>
              </div>

              <!-- SOCKS5 配置 -->
              <div class="row" v-if="outboundData.protocol === 'socks5'">
                <div class="col-6 mb-3">
                  <label class="form-label">用户名 (可选)</label>
                  <input type="text" class="form-control" v-model="outboundData.username" placeholder="用户名">
                </div>
                <div class="col-6 mb-3">
                  <label class="form-label">密码 (可选)</label>
                  <input type="text" class="form-control" v-model="outboundData.password" placeholder="密码">
                </div>
              </div>

              <!-- VLESS 配置 -->
              <div v-if="outboundData.protocol === 'vless'">
                <div class="row">
                  <div class="col-8 mb-3">
                    <label class="form-label">UUID</label>
                    <input type="text" class="form-control" v-model="outboundData.uuid" placeholder="VLESS UUID">
                  </div>
                  <div class="col-4 mb-3">
                    <label class="form-label">Flow (可选)</label>
                    <select class="form-select" v-model="outboundData.flow">
                      <option value="">无</option>
                      <option value="xtls-rprx-vision">xtls-rprx-vision</option>
                    </select>
                  </div>
                </div>
                <div class="row">
                  <div class="col-4 mb-3">
                    <label class="form-label">安全类型</label>
                    <select class="form-select" v-model="outboundVlessTlsType">
                      <option value="none">无</option>
                      <option value="tls">TLS</option>
                      <option value="reality">Reality</option>
                    </select>
                  </div>
                  <div class="col-4 mb-3" v-if="outboundData.tls || outboundData.reality">
                    <label class="form-label">SNI</label>
                    <input type="text" class="form-control" v-model="outboundData.sni" placeholder="TLS SNI">
                  </div>
                  <div class="col-4 mb-3" v-if="outboundData.tls || outboundData.reality">
                    <label class="form-label">Fingerprint</label>
                    <select class="form-select" v-model="outboundData.fp">
                      <option value="chrome">chrome</option>
                      <option value="firefox">firefox</option>
                      <option value="safari">safari</option>
                      <option value="edge">edge</option>
                      <option value="random">random</option>
                    </select>
                  </div>
                </div>
                <div class="row" v-if="outboundData.reality">
                  <div class="col-8 mb-3">
                    <label class="form-label">Public Key</label>
                    <input type="text" class="form-control" v-model="outboundData.pub_key" placeholder="Reality Public Key">
                  </div>
                  <div class="col-4 mb-3">
                    <label class="form-label">Short ID</label>
                    <input type="text" class="form-control" v-model="outboundData.short_id" placeholder="Short ID">
                  </div>
                </div>
                <!-- 传输层配置 -->
                <div class="row">
                  <div class="col-4 mb-3">
                    <label class="form-label">传输层</label>
                    <select class="form-select" v-model="outboundData.network">
                      <option value="">tcp (默认)</option>
                      <option value="ws">WebSocket</option>
                      <option value="grpc">gRPC</option>
                      <option value="http">HTTP/2</option>
                    </select>
                  </div>
                  <div class="col-4 mb-3" v-if="outboundData.network === 'ws'">
                    <label class="form-label">WS Path</label>
                    <input type="text" class="form-control" v-model="outboundData.ws_path" placeholder="/">
                  </div>
                  <div class="col-4 mb-3" v-if="outboundData.network === 'ws'">
                    <label class="form-label">WS Host</label>
                    <input type="text" class="form-control" v-model="outboundData.ws_host" placeholder="可选">
                  </div>
                </div>
              </div>

              <!-- VMess 配置 -->
              <div v-if="outboundData.protocol === 'vmess'">
                <div class="row">
                  <div class="col-6 mb-3">
                    <label class="form-label">UUID</label>
                    <input type="text" class="form-control" v-model="outboundData.uuid" placeholder="VMess UUID">
                  </div>
                  <div class="col-3 mb-3">
                    <label class="form-label">加密方式</label>
                    <select class="form-select" v-model="outboundData.security">
                      <option value="auto">auto</option>
                      <option value="aes-128-gcm">aes-128-gcm</option>
                      <option value="chacha20-poly1305">chacha20-poly1305</option>
                      <option value="none">none</option>
                      <option value="zero">zero</option>
                    </select>
                  </div>
                  <div class="col-3 mb-3">
                    <label class="form-label">Alter ID</label>
                    <input type="number" class="form-control" v-model.number="outboundData.alter_id" min="0" placeholder="0">
                  </div>
                </div>
                <div class="row">
                  <div class="col-4 mb-3">
                    <div class="form-check mt-4">
                      <input type="checkbox" class="form-check-input" v-model="outboundData.tls" id="vmessTls">
                      <label class="form-check-label" for="vmessTls">启用 TLS</label>
                    </div>
                  </div>
                  <div class="col-8 mb-3" v-if="outboundData.tls">
                    <label class="form-label">SNI (可选)</label>
                    <input type="text" class="form-control" v-model="outboundData.sni" placeholder="TLS SNI">
                  </div>
                </div>
                <div class="row">
                  <div class="col-4 mb-3">
                    <label class="form-label">传输层</label>
                    <select class="form-select" v-model="outboundData.network">
                      <option value="">tcp (默认)</option>
                      <option value="ws">WebSocket</option>
                      <option value="grpc">gRPC</option>
                      <option value="http">HTTP/2</option>
                    </select>
                  </div>
                  <div class="col-4 mb-3" v-if="outboundData.network === 'ws'">
                    <label class="form-label">WS Path</label>
                    <input type="text" class="form-control" v-model="outboundData.ws_path" placeholder="/">
                  </div>
                  <div class="col-4 mb-3" v-if="outboundData.network === 'ws'">
                    <label class="form-label">WS Host</label>
                    <input type="text" class="form-control" v-model="outboundData.ws_host" placeholder="可选">
                  </div>
                </div>
              </div>

              <!-- Hysteria2 配置 -->
              <div v-if="outboundData.protocol === 'hysteria2'">
                <div class="row">
                  <div class="col-6 mb-3">
                    <label class="form-label">密码</label>
                    <input type="text" class="form-control" v-model="outboundData.password" placeholder="Hysteria2 密码">
                  </div>
                  <div class="col-6 mb-3">
                    <label class="form-label">SNI (可选)</label>
                    <input type="text" class="form-control" v-model="outboundData.sni" placeholder="TLS SNI">
                  </div>
                </div>
                <div class="row">
                  <div class="col-4 mb-3">
                    <label class="form-label">Obfs 类型 (可选)</label>
                    <select class="form-select" v-model="outboundData.obfs">
                      <option value="">无</option>
                      <option value="salamander">salamander</option>
                    </select>
                  </div>
                  <div class="col-8 mb-3" v-if="outboundData.obfs">
                    <label class="form-label">Obfs 密码</label>
                    <input type="text" class="form-control" v-model="outboundData.obfs_pwd" placeholder="Obfs 密码">
                  </div>
                </div>
              </div>
            </form>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">取消</button>
            <button type="button" class="btn btn-primary" @click="saveOutbound" :disabled="savingOutbound">
              <span v-if="savingOutbound" class="spinner-border spinner-border-sm me-1"></span>
              保存
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, inject, computed } from 'vue'
import { Modal } from 'bootstrap'
import { getServers, createServer, updateServer, deleteServer, getNodes, deployServer, deployAll as apiDeployAll, updateAllAgents as apiUpdateAllAgents, regenerateAgentToken, getServersStatus, reorderServers, getNodeConfigs, saveNodeConfig as saveNodeConfigApi, getServerOutbounds, createServerOutbound, updateServerOutbound, deleteServerOutbound, toggleServerOutbound, type Server, type Node, type ServerStatus, type NodeConfig, type ServerOutbound } from '@/api'

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
const outboundModalEl = ref<HTMLElement | null>(null)
let serverModal: Modal | null = null
let deployModal: Modal | null = null
let deleteModal: Modal | null = null
let nodeConfigModal: Modal | null = null
let outboundModal: Modal | null = null

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
const updatingAgents = ref(false)

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
  return `curl -fsSL ${GITHUB_SCRIPT_URL} | bash -s -- --token ${formData.value.agent_token} --panel ${panelUrl}`
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

// 当前服务器的节点配置缓存 (key 为 nodeId)
const currentServerNodeConfigs = ref<Record<string, NodeConfig>>({})

// 导入链接
const importLink = ref('')

// 服务器配置Tab
const serverConfigTab = ref<'forward' | 'outbound'>('forward')

// 落地出站管理
const serverOutbounds = ref<ServerOutbound[]>([])
const isEditingOutbound = ref(false)
const savingOutbound = ref(false)
const outboundImportLink = ref('')
const outboundData = ref(getDefaultOutboundData())

function getDefaultOutboundData(): Partial<ServerOutbound> {
  return {
    slot: 0,
    enabled: true,
    remark: '',
    protocol: 'shadowsocks',
    host: '',
    port: 443,
    password: '',
    method: '2022-blake3-aes-128-gcm',
    username: '',
    sni: '',
    uuid: '',
    flow: '',
    security: 'auto',
    alter_id: 0,
    tls: false,
    reality: false,
    pub_key: '',
    short_id: '',
    fp: 'chrome',
    obfs: '',
    obfs_pwd: '',
    network: '',
    ws_path: '',
    ws_host: ''
  }
}

// 落地出站 VLESS TLS 类型计算属性
const outboundVlessTlsType = computed({
  get() {
    if (outboundData.value.reality) return 'reality'
    if (outboundData.value.tls) return 'tls'
    return 'none'
  },
  set(value: string) {
    outboundData.value.tls = value === 'tls'
    outboundData.value.reality = value === 'reality'
  }
})

// 加载服务器的落地出站列表
async function loadServerOutbounds() {
  if (!formData.value.id) {
    serverOutbounds.value = []
    return
  }
  try {
    const res = await getServerOutbounds(formData.value.id)
    serverOutbounds.value = res.data.data || []
  } catch (error) {
    console.error('加载落地出站失败:', error)
    serverOutbounds.value = []
  }
}

// 打开添加落地出站模态框
function openAddOutboundModal() {
  if (!formData.value.id) {
    showToast('warning', '警告', '请先保存服务器后再添加落地出站')
    return
  }
  if (serverOutbounds.value.length >= 10) {
    showToast('warning', '警告', '最多只能添加 10 个落地出站')
    return
  }
  isEditingOutbound.value = false
  outboundData.value = getDefaultOutboundData()
  outboundImportLink.value = ''
  outboundModal?.show()
}

// 打开编辑落地出站模态框
function openEditOutboundModal(outbound: ServerOutbound) {
  isEditingOutbound.value = true
  outboundData.value = { ...outbound }
  outboundImportLink.value = ''
  outboundModal?.show()
}

// 保存落地出站
async function saveOutbound() {
  if (!outboundData.value.remark) {
    showToast('warning', '警告', '请填写备注名称')
    return
  }
  if (!outboundData.value.host) {
    showToast('warning', '警告', '请填写出站地址')
    return
  }
  
  savingOutbound.value = true
  try {
    if (isEditingOutbound.value && outboundData.value.slot) {
      await updateServerOutbound(formData.value.id, outboundData.value.slot, outboundData.value)
      showToast('success', '成功', '落地出站已更新')
    } else {
      await createServerOutbound(formData.value.id, outboundData.value)
      showToast('success', '成功', '落地出站已添加')
    }
    outboundModal?.hide()
    await loadServerOutbounds()
  } catch (error: any) {
    showToast('error', '错误', error.response?.data?.error || '保存失败')
  } finally {
    savingOutbound.value = false
  }
}

// 删除落地出站
async function deleteOutbound(outbound: ServerOutbound) {
  if (!confirm(`确定要删除"${outbound.remark}"吗？`)) return
  
  try {
    await deleteServerOutbound(formData.value.id, outbound.slot)
    showToast('success', '成功', '落地出站已删除')
    await loadServerOutbounds()
  } catch (error: any) {
    showToast('error', '错误', error.response?.data?.error || '删除失败')
  }
}

// 切换落地出站启用状态
async function toggleOutbound(outbound: ServerOutbound) {
  try {
    await toggleServerOutbound(formData.value.id, outbound.slot)
    outbound.enabled = !outbound.enabled
    showToast('success', '成功', `落地出站已${outbound.enabled ? '启用' : '禁用'}`)
  } catch (error: any) {
    showToast('error', '错误', error.response?.data?.error || '操作失败')
  }
}

// 解析落地出站导入链接
function parseOutboundImportLink() {
  const link = outboundImportLink.value.trim()
  if (!link) {
    showToast('warning', '提示', '请输入节点链接')
    return
  }

  try {
    // VMess 链接解析
    if (link.startsWith('vmess://')) {
      parseOutboundVmessLink(link)
    }
    // VLESS 链接解析
    else if (link.startsWith('vless://')) {
      parseOutboundVlessLink(link)
    }
    // Trojan 链接解析
    else if (link.startsWith('trojan://')) {
      parseOutboundTrojanLink(link)
    }
    // Shadowsocks 链接解析
    else if (link.startsWith('ss://')) {
      parseOutboundShadowsocksLink(link)
    }
    // Hysteria2 链接解析
    else if (link.startsWith('hysteria2://') || link.startsWith('hy2://')) {
      parseOutboundHysteria2Link(link)
    }
    // SOCKS5 链接解析
    else if (link.startsWith('socks5://') || link.startsWith('socks://')) {
      parseOutboundSocks5Link(link)
    }
    else {
      showToast('error', '错误', '不支持的链接格式')
      return
    }
    
    showToast('success', '成功', '节点信息已解析')
    outboundImportLink.value = ''
  } catch (error: any) {
    showToast('error', '解析失败', error.message || '链接格式错误')
  }
}

// 解析 VMess 链接到落地出站 (v2rayN 格式)
function parseOutboundVmessLink(link: string) {
  const base64 = link.replace('vmess://', '')
  const decoded = atob(base64)
  const config = JSON.parse(decoded)
  
  outboundData.value.protocol = 'vmess'
  outboundData.value.host = config.add || ''
  outboundData.value.port = parseInt(config.port) || 443
  outboundData.value.uuid = config.id || ''
  outboundData.value.alter_id = parseInt(config.aid) || 0
  outboundData.value.security = config.scy || 'auto'
  outboundData.value.tls = config.tls === 'tls'
  outboundData.value.sni = config.sni || ''
  outboundData.value.fp = 'chrome'
  
  // 备注自动填充
  if (!outboundData.value.remark && config.ps) {
    outboundData.value.remark = config.ps
  }
  
  // 传输层配置
  const network = config.net || 'tcp'
  outboundData.value.network = network === 'tcp' ? '' : network
  if (network === 'ws') {
    outboundData.value.ws_path = config.path || '/'
    outboundData.value.ws_host = config.host || ''
  } else if (network === 'grpc') {
    outboundData.value.ws_path = config.path || ''
  }
}

// 解析 VLESS 链接到落地出站
function parseOutboundVlessLink(link: string) {
  // vless://uuid@host:port?params#name
  const url = new URL(link)
  const params = new URLSearchParams(url.search)
  
  outboundData.value.protocol = 'vless'
  outboundData.value.uuid = url.username
  outboundData.value.host = url.hostname
  outboundData.value.port = parseInt(url.port) || 443
  outboundData.value.flow = params.get('flow') || ''
  outboundData.value.sni = params.get('sni') || params.get('serverName') || ''
  outboundData.value.fp = params.get('fp') || 'chrome'
  
  // 备注自动填充
  if (!outboundData.value.remark && url.hash) {
    outboundData.value.remark = decodeURIComponent(url.hash.substring(1))
  }
  
  // 安全类型 (security 参数，不要和 type 传输层混淆)
  const security = params.get('security') || ''
  if (security === 'reality') {
    outboundData.value.reality = true
    outboundData.value.tls = false
    outboundData.value.pub_key = params.get('pbk') || ''
    outboundData.value.short_id = params.get('sid') || ''
  } else if (security === 'tls') {
    outboundData.value.tls = true
    outboundData.value.reality = false
  } else {
    outboundData.value.tls = false
    outboundData.value.reality = false
  }
  
  // 传输层配置 (type 参数)
  const transport = params.get('type') || 'tcp'
  outboundData.value.network = transport === 'tcp' ? '' : transport
  if (transport === 'ws') {
    outboundData.value.ws_path = params.get('path') || '/'
    outboundData.value.ws_host = params.get('host') || ''
  } else if (transport === 'grpc') {
    outboundData.value.ws_path = params.get('serviceName') || ''
  }
}

// 解析 Trojan 链接到落地出站
function parseOutboundTrojanLink(link: string) {
  // trojan://password@host:port?params#name
  const url = new URL(link)
  const params = new URLSearchParams(url.search)
  
  outboundData.value.protocol = 'trojan'
  outboundData.value.password = decodeURIComponent(url.username)
  outboundData.value.host = url.hostname
  outboundData.value.port = parseInt(url.port) || 443
  outboundData.value.tls = true  // Trojan 必须 TLS
  outboundData.value.sni = params.get('sni') || params.get('peer') || ''
  outboundData.value.fp = params.get('fp') || 'chrome'
  
  // 备注自动填充
  if (!outboundData.value.remark && url.hash) {
    outboundData.value.remark = decodeURIComponent(url.hash.substring(1))
  }
  
  // 传输层配置
  const transport = params.get('type') || 'tcp'
  outboundData.value.network = transport === 'tcp' ? '' : transport
  if (transport === 'ws') {
    outboundData.value.ws_path = params.get('path') || '/'
    outboundData.value.ws_host = params.get('host') || ''
  } else if (transport === 'grpc') {
    outboundData.value.ws_path = params.get('serviceName') || ''
  }
}

// 解析 Shadowsocks 链接到落地出站
function parseOutboundShadowsocksLink(link: string) {
  // ss://base64(method:password)@host:port#name
  // 或 ss://base64(method:password@host:port)#name
  const withoutPrefix = link.replace('ss://', '')
  const hashIndex = withoutPrefix.indexOf('#')
  const mainPart = hashIndex > -1 ? withoutPrefix.substring(0, hashIndex) : withoutPrefix
  const remarkPart = hashIndex > -1 ? withoutPrefix.substring(hashIndex + 1) : ''
  
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
  
  outboundData.value.protocol = 'shadowsocks'
  outboundData.value.host = host
  outboundData.value.port = port
  outboundData.value.method = method
  outboundData.value.password = password
  
  // 备注自动填充
  if (!outboundData.value.remark && remarkPart) {
    outboundData.value.remark = decodeURIComponent(remarkPart)
  }
}

// 解析 Hysteria2 链接到落地出站
function parseOutboundHysteria2Link(link: string) {
  // hysteria2://password@host:port?params#name
  // hy2://password@host:port?params#name
  const url = new URL(link.replace('hysteria2://', 'hy2://').replace('hy2://', 'http://'))
  const params = new URLSearchParams(url.search)
  
  outboundData.value.protocol = 'hysteria2'
  outboundData.value.password = decodeURIComponent(url.username)
  outboundData.value.host = url.hostname
  outboundData.value.port = parseInt(url.port) || 443
  outboundData.value.tls = true  // Hysteria2 必须 TLS
  outboundData.value.sni = params.get('sni') || ''
  outboundData.value.obfs = params.get('obfs') || ''
  outboundData.value.obfs_pwd = params.get('obfs-password') || ''
  
  // 备注自动填充
  if (!outboundData.value.remark && url.hash) {
    outboundData.value.remark = decodeURIComponent(url.hash.substring(1))
  }
}

// 解析 SOCKS5 链接到落地出站
function parseOutboundSocks5Link(link: string) {
  // socks5://user:pass@host:port#name  或  socks://user:pass@host:port#name
  const normalized = link.replace('socks5://', 'http://').replace('socks://', 'http://')
  const url = new URL(normalized)
  
  outboundData.value.protocol = 'socks5'
  outboundData.value.host = url.hostname
  outboundData.value.port = parseInt(url.port) || 1080
  outboundData.value.username = url.username ? decodeURIComponent(url.username) : ''
  outboundData.value.password = url.password ? decodeURIComponent(url.password) : ''
  
  // 备注自动填充
  if (!outboundData.value.remark && url.hash) {
    outboundData.value.remark = decodeURIComponent(url.hash.substring(1))
  }
}

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
    // SOCKS5 链接解析
    else if (link.startsWith('socks5://') || link.startsWith('socks://')) {
      parseSocks5Link(link)
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
  nodeConfigData.value.outbound_sni = config.sni || ''
  nodeConfigData.value.outbound_fp = 'chrome'
  
  // 传输层配置
  const network = config.net || 'tcp'
  nodeConfigData.value.outbound_network = network === 'tcp' ? '' : network
  if (network === 'ws') {
    nodeConfigData.value.outbound_ws_path = config.path || '/'
    nodeConfigData.value.outbound_ws_host = config.host || ''
  } else if (network === 'grpc') {
    nodeConfigData.value.outbound_ws_path = config.path || ''
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
  
  // 安全类型 (security 参数，不要和 type 传输层混淆)
  const security = params.get('security') || ''
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
  
  // 传输层配置 (type 参数)
  const transport = params.get('type') || 'tcp'
  nodeConfigData.value.outbound_network = transport === 'tcp' ? '' : transport
  if (transport === 'ws') {
    nodeConfigData.value.outbound_ws_path = params.get('path') || '/'
    nodeConfigData.value.outbound_ws_host = params.get('host') || ''
  } else if (transport === 'grpc') {
    nodeConfigData.value.outbound_ws_path = params.get('serviceName') || ''
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
  nodeConfigData.value.outbound_tls = true  // Trojan 必须 TLS
  nodeConfigData.value.outbound_sni = params.get('sni') || params.get('peer') || ''
  nodeConfigData.value.outbound_fp = params.get('fp') || 'chrome'
  
  // 传输层配置
  const transport = params.get('type') || 'tcp'
  nodeConfigData.value.outbound_network = transport === 'tcp' ? '' : transport
  if (transport === 'ws') {
    nodeConfigData.value.outbound_ws_path = params.get('path') || '/'
    nodeConfigData.value.outbound_ws_host = params.get('host') || ''
  } else if (transport === 'grpc') {
    nodeConfigData.value.outbound_ws_path = params.get('serviceName') || ''
  }
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
  nodeConfigData.value.outbound_tls = true  // Hysteria2 必须 TLS
  nodeConfigData.value.outbound_sni = params.get('sni') || ''
  nodeConfigData.value.outbound_obfs = params.get('obfs') || ''
  nodeConfigData.value.outbound_obfs_pwd = params.get('obfs-password') || ''
}

// 解析 SOCKS5 链接
function parseSocks5Link(link: string) {
  // socks5://user:pass@host:port#name  或  socks://user:pass@host:port#name
  const normalized = link.replace('socks5://', 'http://').replace('socks://', 'http://')
  const url = new URL(normalized)
  
  nodeConfigData.value.outbound_protocol = 'socks5'
  nodeConfigData.value.outbound_host = url.hostname
  nodeConfigData.value.outbound_port = parseInt(url.port) || 1080
  nodeConfigData.value.outbound_username = url.username ? decodeURIComponent(url.username) : ''
  nodeConfigData.value.outbound_password = url.password ? decodeURIComponent(url.password) : ''
}

function getDefaultFormData() {
  return {
    id: 0,
    name: '',
    host: '',
    host_ipv6: '',
    node_domain: '',
    dns_resolve: 'ipv4',
    category: 'direct',
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
  outboundModal = new Modal(outboundModalEl.value!)
  
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
  if (bytesPerSec < 1024) return `${bytesPerSec}B/s`
  if (bytesPerSec < 1024 * 1024) return `${(bytesPerSec / 1024).toFixed(1)}KB/s`
  return `${(bytesPerSec / 1024 / 1024).toFixed(2)}MB/s`
}

// 格式化流量显示
function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes}B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)}KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(2)}MB`
  return `${(bytes / 1024 / 1024 / 1024).toFixed(2)}GB`
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
  const config = currentServerNodeConfigs.value[node.id.toString()]
  return config && (config.forward_enabled || config.outbound_enabled)
}

// 检查节点是否启用了端口转发
function isForwardEnabled(node: Node) {
  const config = currentServerNodeConfigs.value[node.id.toString()]
  return config?.forward_enabled || false
}

// 检查节点是否启用了落地出站
function isOutboundEnabled(node: Node) {
  const config = currentServerNodeConfigs.value[node.id.toString()]
  return config?.outbound_enabled || false
}

function openAddModal() {
  isEditing.value = false
  formData.value = getDefaultFormData()
  currentServerNodeConfigs.value = {} // 新建服务器时清空节点配置缓存
  serverModal?.show()
}

async function openEditModal(server: Server) {
  isEditing.value = true
  formData.value = {
    id: server.id,
    name: server.name,
    host: server.host || '',
    host_ipv6: server.host_ipv6 || '',
    node_domain: server.node_domain || '',
    dns_resolve: server.dns_resolve || 'ipv4',
    category: server.category,
    node_1: server.node_1 || '',
    node_2: server.node_2 || '',
    node_3: server.node_3 || '',
    enabled: server.enabled === 1 || server.enabled === true,
    agent_token: server.agent_token || '',
    agent_id: server.agent_id || '',
    agent_online: server.agent_online || false
  }
  
  // 重置 tab 状态
  serverConfigTab.value = 'forward'
  
  // 加载服务器的节点配置
  currentServerNodeConfigs.value = {}
  try {
    const res = await getNodeConfigs(server.id)
    currentServerNodeConfigs.value = res.data.data || {}
  } catch (error) {
    console.error('加载节点配置失败:', error)
  }
  
  // 加载落地出站
  await loadServerOutbounds()
  
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

async function updateAllAgents() {
  updatingAgents.value = true
  
  try {
    const res = await apiUpdateAllAgents()
    const data = res.data.data
    
    showToast('success', '成功', `${data.message}（共 ${data.total} 个存活 Agent）`)
  } catch (error: any) {
    showToast('error', '错误', error.response?.data?.error || 'Agent 更新失败')
  } finally {
    updatingAgents.value = false
  }
}

async function deployAll() {
  deploying.value = true
  
  try {
    const res = await apiDeployAll()
    const data = res.data.data
    
    showToast('success', '成功', `${data.message}（共 ${data.total} 个存活 Agent）`)
  } catch (error: any) {
    showToast('error', '错误', error.response?.data?.error || '部署失败')
  } finally {
    deploying.value = false
  }
}

async function openNodeConfigModal(node: Node) {
  selectedNode.value = node
  // 重置为默认值（仅端口转发）
  nodeConfigData.value = {
    listen_port: node.port,
    forward_enabled: false,
    forward_host: '',
    forward_port: 0,
    // 保留这些字段以兼容旧数据，但不再在UI中使用
    outbound_enabled: false,
    outbound_protocol: '',
    outbound_host: '',
    outbound_port: 0,
    outbound_password: '',
    outbound_method: '',
    outbound_username: '',
    outbound_sni: '',
    outbound_uuid: '',
    outbound_flow: '',
    outbound_security: 'auto',
    outbound_alter_id: 0,
    outbound_tls: false,
    outbound_reality: false,
    outbound_pub_key: '',
    outbound_short_id: '',
    outbound_fp: 'chrome',
    outbound_obfs: '',
    outbound_obfs_pwd: '',
    outbound_network: '',
    outbound_ws_path: '',
    outbound_ws_host: ''
  }
  
  // 尝试加载已有配置
  if (formData.value.id) {
    try {
      const res = await getNodeConfigs(formData.value.id)
      // 后端返回的是 map 格式，key 为 nodeId
      const existing = res.data.data?.[node.id.toString()] as NodeConfig | undefined
      if (existing) {
        nodeConfigData.value.listen_port = existing.listen_port || node.port
        nodeConfigData.value.forward_enabled = existing.forward_enabled || false
        nodeConfigData.value.forward_host = existing.forward_host || ''
        nodeConfigData.value.forward_port = existing.forward_port || 0
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
    
    // 更新本地缓存
    currentServerNodeConfigs.value[selectedNode.value.id.toString()] = {
      ...nodeConfigData.value,
      node_id: selectedNode.value.id,
      server_id: formData.value.id
    } as NodeConfig
    
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

/* 部署命令框样式 */
.deploy-command-box {
  position: relative;
  background: linear-gradient(135deg, #f8fafc 0%, #f1f5f9 100%);
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 0.75rem 2.5rem 0.75rem 0.75rem;
  cursor: pointer;
  transition: all 0.3s ease;
  overflow: hidden;
}

.deploy-command-box:hover {
  background: linear-gradient(135deg, #e0f2fe 0%, #dbeafe 100%);
  border-color: #3b82f6;
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.15);
}

.deploy-command-box:active {
  transform: scale(0.98);
}

.deploy-command-text {
  display: block;
  font-size: 0.8rem;
  color: #1e293b;
  word-break: break-all;
  white-space: pre-wrap;
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
  line-height: 1.4;
  padding-right: 1rem;
}

.copy-icon {
  position: absolute;
  right: 0.75rem;
  top: 50%;
  transform: translateY(-50%);
  color: #64748b;
  font-size: 1rem;
  transition: all 0.3s ease;
}

.deploy-command-box:hover .copy-icon {
  color: #3b82f6;
  transform: translateY(-50%) scale(1.1);
}

/* 落地出站卡片样式 */
.outbound-card {
  background: linear-gradient(135deg, #f8fafc 0%, #f1f5f9 100%);
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  padding: 1rem;
  transition: all 0.3s ease;
}

.outbound-card:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  border-color: #cbd5e1;
}

.outbound-card.outbound-disabled {
  opacity: 0.6;
  background: linear-gradient(135deg, #f1f5f9 0%, #e2e8f0 100%);
}

.outbound-card .card-title {
  font-size: 1rem;
  font-weight: 600;
  color: #1e293b;
  margin-bottom: 0.25rem;
}

.outbound-card .card-subtitle {
  font-size: 0.8rem;
  color: #64748b;
}

.outbound-card .btn-group .btn {
  padding: 0.25rem 0.5rem;
  font-size: 0.8rem;
}
</style>
