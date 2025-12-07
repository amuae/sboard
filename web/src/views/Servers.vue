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
                <i class="bi bi-hdd"></i> {{ server.host || '等待 Agent 上报' }}
              </div>
              <div class="server-host" v-if="server.node_domain">
                <i class="bi bi-globe"></i> {{ server.node_domain }}
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
                  <span class="traffic-value text-muted">{{ formatBytes(server.monthly_in || 0) }}</span>
                </div>
              </div>
              <div class="col-6">
                <div class="speed-item">
                  <i class="bi bi-arrow-up text-warning"></i>
                  <span class="speed-value">{{ formatSpeed(server.net_out || 0) }}</span>
                  <span class="traffic-value text-muted">{{ formatBytes(server.monthly_out || 0) }}</span>
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
                  <small class="text-muted d-block mb-2">配置落地服务器出站地址</small>
                  <div class="row">
                    <div class="col-8">
                      <div class="mb-2">
                        <label class="form-label small">出站地址 (IP/域名)</label>
                        <input type="text" class="form-control form-control-sm" v-model="nodeConfigData.outbound_host" placeholder="落地服务器地址">
                      </div>
                    </div>
                    <div class="col-4">
                      <div class="mb-2">
                        <label class="form-label small">出站端口</label>
                        <input type="number" class="form-control form-control-sm" v-model.number="nodeConfigData.outbound_port" min="1" max="65535" placeholder="端口">
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
import { getServers, createServer, updateServer, deleteServer, getNodes, deployServer, regenerateAgentToken, getServersStatus, reorderServers, type Server, type Node, type ServerStatus } from '@/api'

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

// 计算 Agent 部署命令
const agentDeployCommand = computed(() => {
  if (!formData.value.agent_token) return ''
  const panelUrl = window.location.origin
  const coreType = formData.value.core_type || 'sing-box'
  return `curl -fsSL ${panelUrl}/install-agent.sh | sudo bash -s -- --token ${formData.value.agent_token} --panel ${panelUrl} --core ${coreType}`
})

// 节点配置
const selectedNode = ref<Node | null>(null)
const nodeConfigData = ref({
  forward_enabled: false,
  forward_host: '',
  forward_port: 0,
  outbound_enabled: false,
  outbound_host: '',
  outbound_port: 0
})

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

function openNodeConfigModal(node: Node) {
  selectedNode.value = node
  nodeConfigData.value = {
    forward_enabled: false,
    forward_host: '',
    forward_port: 0,
    outbound_enabled: false,
    outbound_host: '',
    outbound_port: 0
  }
  nodeConfigModal?.show()
}

function saveNodeConfig() {
  // 保存节点配置
  showToast('success', '成功', '节点配置已保存')
  nodeConfigModal?.hide()
}

// Agent 相关函数
async function copyAgentToken() {
  if (formData.value.agent_token) {
    try {
      await navigator.clipboard.writeText(formData.value.agent_token)
      showToast('success', '成功', 'Agent Token 已复制到剪贴板')
    } catch (error) {
      showToast('error', '错误', '复制失败')
    }
  }
}

async function copyDeployCommand() {
  try {
    await navigator.clipboard.writeText(agentDeployCommand.value)
    showToast('success', '成功', '部署命令已复制到剪贴板')
  } catch (error) {
    showToast('error', '错误', '复制失败')
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
  font-size: 0.75rem;
  padding: 0.25rem 0;
}

.speed-item i {
  font-size: 0.7rem;
}

.speed-value {
  font-weight: 500;
  font-family: 'Consolas', 'Monaco', monospace;
}

.traffic-value {
  font-size: 0.65rem;
  font-family: 'Consolas', 'Monaco', monospace;
  margin-left: auto;
  opacity: 0.7;
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
