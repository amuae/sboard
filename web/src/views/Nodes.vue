<template>
  <div class="container-fluid mt-4">
    <div class="d-flex justify-content-between mb-4">
      <h2>节点管理</h2>
      <div>
        <button class="btn btn-outline-info me-2" @click="showConfigModal">
          <i class="bi bi-file-code"></i> 查看配置
        </button>
        <button class="btn btn-primary" @click="openAddModal">
          <i class="bi bi-plus-lg"></i> 添加节点
        </button>
      </div>
    </div>

    <div id="nodesList" class="row">
      <div 
        v-for="node in nodes" 
        :key="node.id" 
        class="col-xl-3 col-lg-4 col-md-6"
      >
        <div class="card node-card">
          <div class="card-body">
            <div class="d-flex justify-content-between align-items-start mb-2">
              <h6 class="card-title mb-0">{{ node.tag }}</h6>
              <span :class="['badge', node.enabled ? 'bg-success' : 'bg-secondary']">
                {{ node.enabled ? '启用' : '禁用' }}
              </span>
            </div>
            <div class="small mb-2">
              <div class="d-flex justify-content-between">
                <span class="text-muted">协议:</span>
                <span class="text-primary fw-bold">{{ node.protocol.toUpperCase() }}</span>
              </div>
              <div class="d-flex justify-content-between">
                <span class="text-muted">端口:</span>
                <span>{{ node.port }}</span>
              </div>
              <div class="d-flex justify-content-between" v-if="node.tls_enabled">
                <span class="text-muted">TLS:</span>
                <span class="text-success">
                  <i class="bi bi-shield-check"></i>
                  {{ node.reality_enabled ? 'Reality' : '启用' }}
                </span>
              </div>
              <div class="d-flex justify-content-between" v-if="node.transport_enabled">
                <span class="text-muted">传输层:</span>
                <span>{{ node.transport_type?.toUpperCase() }}</span>
              </div>
            </div>
            <div class="btn-group btn-group-sm w-100">
              <button class="btn btn-outline-primary" @click="openEditModal(node)">
                <i class="bi bi-pencil"></i> 编辑
              </button>
              <button class="btn btn-outline-danger" @click="confirmDelete(node)">
                <i class="bi bi-trash"></i> 删除
              </button>
            </div>
          </div>
        </div>
      </div>
      
      <div v-if="nodes.length === 0" class="col-12 text-center py-5">
        <p class="text-muted">暂无节点</p>
      </div>
    </div>

    <!-- 节点模态框 -->
    <div class="modal fade" id="nodeModal" tabindex="-1" ref="nodeModalEl">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 id="modalTitle">{{ isEditing ? '编辑节点' : '添加节点' }}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <form @submit.prevent="saveNode">
            <div class="modal-body">
              <div class="row">
                <div class="col-md-6 mb-3">
                  <label>标签 *</label>
                  <input type="text" class="form-control" v-model="formData.tag" required>
                </div>
                <div class="col-md-6 mb-3">
                  <label>协议</label>
                  <select class="form-select" v-model="formData.protocol" @change="onProtocolChange">
                    <option value="trojan">Trojan</option>
                    <option value="vless">VLESS</option>
                    <option value="vmess">VMess</option>
                    <option value="anytls">AnyTLS</option>
                    <option value="shadowsocks">Shadowsocks</option>
                    <option value="hysteria2">Hysteria2</option>
                  </select>
                </div>
                <div class="col-md-6 mb-3">
                  <label>监听地址</label>
                  <input type="text" class="form-control" v-model="formData.listen">
                </div>
                <div class="col-md-6 mb-3">
                  <label>端口 *</label>
                  <input type="number" class="form-control" v-model.number="formData.port" min="1" max="65535" required>
                </div>
              </div>

              <!-- TLS 配置 -->
              <div class="form-check mb-2" v-if="formData.protocol !== 'shadowsocks'">
                <input class="form-check-input" type="checkbox" v-model="formData.tls_enabled" id="tlsEnabled">
                <label class="form-check-label" for="tlsEnabled">启用 TLS</label>
              </div>
              
              <div v-if="formData.tls_enabled && formData.protocol !== 'shadowsocks'" id="tlsFields">
                <div class="mb-2">
                  <label>Server Name</label>
                  <input type="text" class="form-control" v-model="formData.server_name">
                </div>
                
                <!-- 证书字段（Reality 启用时隐藏） -->
                <div id="certFields" v-if="!formData.reality_enabled">
                  <div class="row">
                    <div class="col-md-6 mb-2">
                      <label>证书路径</label>
                      <input type="text" class="form-control" v-model="formData.cert_path">
                    </div>
                    <div class="col-md-6 mb-2">
                      <label>密钥路径</label>
                      <input type="text" class="form-control" v-model="formData.key_path">
                    </div>
                  </div>
                </div>
                
                <!-- Reality 配置 -->
                <div class="form-check mb-2">
                  <input class="form-check-input" type="checkbox" v-model="formData.reality_enabled" id="realityEnabled">
                  <label class="form-check-label" for="realityEnabled">启用 Reality</label>
                </div>
                <div v-if="formData.reality_enabled" id="realityFields">
                  <div class="mb-2">
                    <label>Reality Server (握手服务器)</label>
                    <input type="text" class="form-control" v-model="formData.reality_server" placeholder="www.apple.com">
                    <small class="text-muted">用于伪装的真实网站，如: www.apple.com</small>
                  </div>
                  <div class="mb-2">
                    <label>Public Key 
                      <button type="button" class="btn btn-sm btn-outline-primary" @click="generateRealityKeys">
                        <i class="bi bi-key"></i> 生成密钥对
                      </button>
                    </label>
                    <input type="text" class="form-control" v-model="formData.reality_pubkey" placeholder="点击生成按钮自动生成">
                  </div>
                  <div class="mb-2">
                    <label>Private Key (部署时使用)</label>
                    <input type="text" class="form-control" v-model="formData.reality_privkey" placeholder="点击生成按钮自动生成" readonly>
                    <small class="text-muted">此密钥将在部署时写入服务器配置</small>
                  </div>
                  <div class="mb-2">
                    <label>Short ID</label>
                    <input type="text" class="form-control" v-model="formData.reality_short_id" placeholder="保存时自动生成" readonly>
                    <small class="text-muted">保存节点时自动生成的随机标识</small>
                  </div>
                </div>
              </div>

              <!-- 传输层配置 -->
              <div class="form-check mb-2 mt-3" v-if="formData.protocol !== 'shadowsocks' && formData.protocol !== 'hysteria2'">
                <input class="form-check-input" type="checkbox" v-model="formData.transport_enabled" id="transportEnabled">
                <label class="form-check-label" for="transportEnabled">启用传输层</label>
              </div>
              <div v-if="formData.transport_enabled && formData.protocol !== 'shadowsocks' && formData.protocol !== 'hysteria2'" id="transportFields">
                <div class="mb-2">
                  <label>传输层类型</label>
                  <select class="form-select" v-model="formData.transport_type">
                    <option value="ws">WebSocket (ws)</option>
                    <option value="grpc">gRPC</option>
                    <option value="http">HTTP/2</option>
                    <option value="httpupgrade">HTTPUpgrade (仅sing-box)</option>
                  </select>
                </div>
                <!-- WebSocket / HTTPUpgrade -->
                <div v-if="formData.transport_type === 'ws' || formData.transport_type === 'httpupgrade'" id="wsFields">
                  <div class="mb-2">
                    <label>Path</label>
                    <input type="text" class="form-control" v-model="formData.ws_path" placeholder="/path">
                  </div>
                  <div class="mb-2">
                    <label>Host (伪装域名)</label>
                    <input type="text" class="form-control" v-model="formData.transport_host" placeholder="cdn.example.com">
                  </div>
                </div>
                <!-- gRPC -->
                <div v-if="formData.transport_type === 'grpc'" id="grpcFields">
                  <div class="mb-2">
                    <label>gRPC Service Name</label>
                    <input type="text" class="form-control" v-model="formData.grpc_service">
                  </div>
                </div>
              </div>

              <!-- Flow 配置 (仅VLESS) -->
              <div v-if="formData.protocol === 'vless' && formData.tls_enabled && !formData.transport_enabled" id="flowFields" class="mt-3">
                <div class="mb-2">
                  <label>Flow (VLESS XTLS)</label>
                  <select class="form-select" v-model="formData.flow">
                    <option value="">无</option>
                    <option value="xtls-rprx-vision">xtls-rprx-vision (推荐)</option>
                    <option value="xtls-rprx-direct">xtls-rprx-direct (已弃用)</option>
                  </select>
                  <small class="text-muted">仅在 TLS 且不使用传输层时可用</small>
                </div>
              </div>

              <!-- Shadowsocks 配置 -->
              <div v-if="formData.protocol === 'shadowsocks'" id="shadowsocksFields" class="mt-3">
                <div class="alert alert-info">
                  <i class="bi bi-info-circle"></i> <strong>Shadowsocks 配置说明：</strong>
                  <ul class="mb-0 mt-1">
                    <li>每个节点使用一个随机生成的 32 位密码</li>
                    <li>所有用户共享该节点的同一密码</li>
                    <li>UDP 支持已默认启用（游戏、DNS）</li>
                    <li>只提供最推荐的 3 种加密方式</li>
                    <li>不支持 TLS（使用原生协议）</li>
                  </ul>
                </div>
                <div class="mb-2">
                  <label>加密方法 <span class="text-danger">*</span></label>
                  <select class="form-select" v-model="formData.ss_method">
                    <option value="aes-256-gcm">aes-256-gcm (推荐 - 高安全性)</option>
                    <option value="chacha20-ietf-poly1305">chacha20-ietf-poly1305 (推荐 - ARM 设备优化)</option>
                    <option value="2022-blake3-aes-256-gcm">2022-blake3-aes-256-gcm (最新 - SS2022 最高安全)</option>
                  </select>
                </div>
              </div>

              <!-- Hysteria2 配置 -->
              <div v-if="formData.protocol === 'hysteria2'" id="hysteria2Fields" class="mt-3">
                <div class="alert alert-info">
                  <i class="bi bi-info-circle"></i> <strong>Hysteria2 配置说明：</strong>
                  <ul class="mb-0 mt-1">
                    <li>基于 QUIC 协议（UDP），必须启用 TLS</li>
                    <li>支持 Salamander 混淆对抗 DPI 检测</li>
                    <li>速率控制使用 Brutal 拥塞控制算法</li>
                  </ul>
                </div>
                
                <h6 class="mb-2"><i class="bi bi-shield-lock"></i> 认证配置</h6>
                <div class="mb-3">
                  <label>认证密码</label>
                  <input type="text" class="form-control" v-model="formData.hy2_password" placeholder="留空则使用用户UUID">
                  <small class="text-muted">留空时将使用用户UUID作为密码（推荐使用强随机密码）</small>
                </div>
                
                <h6 class="mb-2"><i class="bi bi-speedometer2"></i> Brutal 速率控制</h6>
                <div class="row mb-3">
                  <div class="col-md-6 mb-2">
                    <label>上行带宽 (Mbps)</label>
                    <input type="number" class="form-control" v-model.number="formData.hy2_up_mbps" min="1" max="10000">
                    <small class="text-muted">设置为实际带宽的 80-90%</small>
                  </div>
                  <div class="col-md-6 mb-2">
                    <label>下行带宽 (Mbps)</label>
                    <input type="number" class="form-control" v-model.number="formData.hy2_down_mbps" min="1" max="10000">
                    <small class="text-muted">设置为实际带宽的 80-90%</small>
                  </div>
                </div>
                
                <h6 class="mb-2"><i class="bi bi-eyeglasses"></i> 混淆设置</h6>
                <div class="mb-2">
                  <label>混淆类型</label>
                  <select class="form-select" v-model="formData.hy2_obfs">
                    <option value="">不启用混淆</option>
                    <option value="salamander">Salamander（推荐）</option>
                  </select>
                </div>
                <div v-if="formData.hy2_obfs === 'salamander'" id="hy2ObfsFields">
                  <div class="mb-2">
                    <label>混淆密码 <span class="text-danger">*</span></label>
                    <input type="text" class="form-control" v-model="formData.hy2_obfs_password" placeholder="混淆密码（与认证密码不同）">
                    <small class="text-muted">用于混淆 QUIC 流量，防止被 DPI 识别</small>
                  </div>
                </div>
              </div>

              <div class="mb-3 mt-3">
                <label>备注</label>
                <textarea class="form-control" v-model="formData.notes" rows="2" placeholder="可选，输入节点的备注信息"></textarea>
              </div>

              <div class="form-check mt-3">
                <input class="form-check-input" type="checkbox" v-model="formData.enabled" id="nodeEnabled">
                <label class="form-check-label" for="nodeEnabled">启用节点</label>
              </div>
            </div>
            <div class="modal-footer">
              <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">取消</button>
              <button type="submit" class="btn btn-primary" :disabled="saving">
                <span v-if="saving" class="spinner-border spinner-border-sm me-1"></span>
                保存
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>

    <!-- 配置文件查看模态框 -->
    <div class="modal fade" id="configModal" tabindex="-1" ref="configModalEl">
      <div class="modal-dialog modal-dialog-scrollable modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title"><i class="bi bi-file-code"></i> 服务器配置文件</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <!-- 配置类型选择 -->
            <div class="btn-group w-100 mb-3" role="group">
              <input type="radio" class="btn-check" name="configType" id="configSingbox" value="sing-box" v-model="configType">
              <label class="btn btn-outline-success" for="configSingbox">
                <i class="bi bi-box-seam"></i> Sing-Box
              </label>
            </div>
            
            <!-- 配置内容显示 -->
            <div v-if="configLoading" class="text-center py-4">
              <div class="spinner-border text-primary" role="status">
                <span class="visually-hidden">Loading...</span>
              </div>
              <p class="mt-2 text-muted">加载配置中...</p>
            </div>
            <pre id="configContent" v-else style="white-space: pre-wrap; word-break: break-word; background: #1e1e1e; color: #d4d4d4; padding: 1rem; border-radius: 8px; max-height: 450px; overflow-y: auto; font-family: 'Consolas', 'Monaco', monospace; font-size: 0.85rem;">{{ configContent }}</pre>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-primary" @click="copyConfig">
              <i class="bi bi-clipboard"></i> 复制配置
            </button>
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
              确定要删除节点 <strong class="text-danger">"{{ deleteTarget?.tag }}"</strong> 吗？
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
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, inject, watch } from 'vue'
import { Modal } from 'bootstrap'
import { getNodes, createNode, updateNode, deleteNode, getConfig, type Node } from '@/api'

const showToast = inject<(type: 'success' | 'error' | 'warning' | 'info', title: string, message: string) => void>('showToast')!

const nodes = ref<Node[]>([])
const loading = ref(false)

// 模态框
const nodeModalEl = ref<HTMLElement | null>(null)
const configModalEl = ref<HTMLElement | null>(null)
const deleteModalEl = ref<HTMLElement | null>(null)
let nodeModal: Modal | null = null
let configModal: Modal | null = null
let deleteModal: Modal | null = null

// 表单
const isEditing = ref(false)
const saving = ref(false)
const formData = ref(getDefaultFormData())

// 删除
const deleteTarget = ref<Node | null>(null)
const deleting = ref(false)

// 配置查看
const configType = ref('sing-box')
const configContent = ref('')
const configLoading = ref(false)

function getDefaultFormData() {
  return {
    id: 0,
    tag: '',
    protocol: 'trojan',
    listen: '::',
    port: 443,
    tls_enabled: false,
    server_name: 'down.dingtalk.com',
    cert_path: 'server.crt',
    key_path: 'server.key',
    reality_enabled: false,
    reality_server: '',
    reality_pubkey: '',
    reality_privkey: '',
    reality_short_id: '',
    transport_enabled: false,
    transport_type: 'ws',
    ws_path: '/',
    transport_host: '',
    grpc_service: 'GunService',
    flow: '',
    ss_method: 'aes-256-gcm',
    ss_password: '',
    hy2_password: '',
    hy2_up_mbps: 100,
    hy2_down_mbps: 100,
    hy2_obfs: '',
    hy2_obfs_password: '',
    notes: '',
    enabled: true
  }
}

onMounted(async () => {
  await loadNodes()
  nodeModal = new Modal(nodeModalEl.value!)
  configModal = new Modal(configModalEl.value!)
  deleteModal = new Modal(deleteModalEl.value!)
})

// 监听配置类型变化
watch(configType, () => {
  if (configModal) {
    loadConfig()
  }
})

async function loadNodes() {
  loading.value = true
  try {
    const res = await getNodes()
    nodes.value = res.data.data.nodes || []
  } catch (error) {
    showToast('error', '错误', '加载节点列表失败')
  } finally {
    loading.value = false
  }
}

function onProtocolChange() {
  // Shadowsocks 不支持 TLS
  if (formData.value.protocol === 'shadowsocks') {
    formData.value.tls_enabled = false
    formData.value.transport_enabled = false
  }
  // Hysteria2 必须启用 TLS，不支持传输层
  if (formData.value.protocol === 'hysteria2') {
    formData.value.tls_enabled = true
    formData.value.transport_enabled = false
  }
}

function generateRealityKeys() {
  // 生成模拟的 Reality 密钥对（实际应该调用后端API）
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'
  let pubkey = ''
  let privkey = ''
  for (let i = 0; i < 43; i++) {
    pubkey += chars.charAt(Math.floor(Math.random() * chars.length))
    privkey += chars.charAt(Math.floor(Math.random() * chars.length))
  }
  formData.value.reality_pubkey = pubkey
  formData.value.reality_privkey = privkey
  showToast('success', '成功', '已生成 Reality 密钥对')
}

function openAddModal() {
  isEditing.value = false
  formData.value = getDefaultFormData()
  nodeModal?.show()
}

function openEditModal(node: Node) {
  isEditing.value = true
  formData.value = {
    id: node.id,
    tag: node.tag,
    protocol: node.protocol,
    listen: node.listen || '::',
    port: node.port,
    tls_enabled: node.tls_enabled,
    server_name: node.server_name || '',
    cert_path: node.cert_path || '',
    key_path: node.key_path || '',
    reality_enabled: node.reality_enabled,
    reality_server: node.reality_server || '',
    reality_pubkey: node.reality_pubkey || '',
    reality_privkey: node.reality_privkey || '',
    reality_short_id: node.reality_short_id || '',
    transport_enabled: node.transport_enabled,
    transport_type: node.transport_type || 'ws',
    ws_path: node.ws_path || '/',
    transport_host: node.transport_host || '',
    grpc_service: node.grpc_service || 'GunService',
    flow: node.flow || '',
    ss_method: node.ss_method || 'aes-256-gcm',
    ss_password: node.ss_password || '',
    hy2_password: node.hy2_password || '',
    hy2_up_mbps: node.hy2_up_mbps || 100,
    hy2_down_mbps: node.hy2_down_mbps || 100,
    hy2_obfs: node.hy2_obfs || '',
    hy2_obfs_password: node.hy2_obfs_password || '',
    notes: node.notes || '',
    enabled: node.enabled
  }
  nodeModal?.show()
}

async function saveNode() {
  if (!formData.value.tag) {
    showToast('warning', '警告', '请输入节点标签')
    return
  }
  if (!formData.value.port) {
    showToast('warning', '警告', '请输入端口')
    return
  }

  saving.value = true
  try {
    const data: any = {
      tag: formData.value.tag,
      protocol: formData.value.protocol,
      listen: formData.value.listen,
      port: formData.value.port,
      tls_enabled: formData.value.tls_enabled,
      server_name: formData.value.server_name,
      cert_path: formData.value.cert_path,
      key_path: formData.value.key_path,
      reality_enabled: formData.value.reality_enabled,
      reality_server: formData.value.reality_server,
      reality_pubkey: formData.value.reality_pubkey,
      reality_privkey: formData.value.reality_privkey,
      reality_short_id: formData.value.reality_short_id,
      transport_enabled: formData.value.transport_enabled,
      transport_type: formData.value.transport_type,
      ws_path: formData.value.ws_path,
      transport_host: formData.value.transport_host,
      grpc_service: formData.value.grpc_service,
      flow: formData.value.flow,
      ss_method: formData.value.ss_method,
      ss_password: formData.value.ss_password,
      hy2_password: formData.value.hy2_password,
      hy2_up_mbps: formData.value.hy2_up_mbps,
      hy2_down_mbps: formData.value.hy2_down_mbps,
      hy2_obfs: formData.value.hy2_obfs,
      hy2_obfs_password: formData.value.hy2_obfs_password,
      notes: formData.value.notes,
      enabled: formData.value.enabled
    }

    if (isEditing.value) {
      await updateNode(formData.value.id, data)
      showToast('success', '成功', '节点已更新')
    } else {
      await createNode(data)
      showToast('success', '成功', '节点已添加')
    }
    
    nodeModal?.hide()
    await loadNodes()
  } catch (error: any) {
    showToast('error', '错误', error.response?.data?.error || '保存失败')
  } finally {
    saving.value = false
  }
}

function confirmDelete(node: Node) {
  deleteTarget.value = node
  deleteModal?.show()
}

async function doDelete() {
  if (!deleteTarget.value) return
  
  deleting.value = true
  try {
    await deleteNode(deleteTarget.value.id)
    showToast('success', '成功', '节点已删除')
    deleteModal?.hide()
    await loadNodes()
  } catch (error: any) {
    showToast('error', '错误', error.response?.data?.error || '删除失败')
  } finally {
    deleting.value = false
  }
}

async function showConfigModal() {
  configModal?.show()
  await loadConfig()
}

async function loadConfig() {
  configLoading.value = true
  try {
    const res = await getConfig(configType.value)
    configContent.value = res.data.data?.config || ''
  } catch (error) {
    configContent.value = '加载配置失败'
    showToast('error', '错误', '加载配置失败')
  } finally {
    configLoading.value = false
  }
}

function copyConfig() {
  navigator.clipboard.writeText(configContent.value).then(() => {
    showToast('success', '成功', '配置已复制到剪贴板')
  }).catch(() => {
    showToast('error', '错误', '复制失败')
  })
}
</script>

<style scoped>
.node-card {
  background: white;
  border: none;
  border-radius: 16px;
  box-shadow: 0 4px 15px rgba(0, 0, 0, 0.08);
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
  overflow: hidden;
  animation: fadeInUp 0.5s ease-out;
  margin-bottom: 1rem;
}

.node-card::before {
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

.node-card:hover {
  transform: translateY(-5px);
  box-shadow: 0 12px 30px rgba(0, 0, 0, 0.12);
}

.node-card:hover::before {
  transform: scaleX(1);
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
