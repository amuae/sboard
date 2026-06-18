<template>
  <div class="container-fluid mt-4">
    <div class="d-flex justify-content-between mb-4">
      <h2>外部节点</h2>
      <button class="btn btn-primary" @click="openAddModal">
        <i class="bi bi-plus-lg"></i> 添加外部节点
      </button>
    </div>

    <div class="row">
      <div v-for="node in nodes" :key="node.id" class="col-xl-3 col-lg-4 col-md-6">
        <div class="card node-card mb-3">
          <div class="card-body">
            <div class="d-flex justify-content-between align-items-start mb-2">
              <h6 class="card-title mb-0">{{ node.name }}</h6>
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
                <span class="text-muted">地址:</span>
                <span>{{ node.host }}:{{ node.port }}</span>
              </div>
              <div class="d-flex justify-content-between" v-if="node.tls_enabled">
                <span class="text-muted">TLS:</span>
                <span class="text-success">
                  <i class="bi bi-shield-check"></i>
                  {{ node.reality_enabled ? 'Reality' : '启用' }}
                </span>
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

      <!-- 空状态 -->
      <div v-if="nodes.length === 0 && !loading" class="col-12 text-center py-5 text-muted">
        <i class="bi bi-inbox" style="font-size: 3rem;"></i>
        <p class="mt-2">暂无外部节点</p>
        <p class="small">添加未被 SBoard Agent 管理的第三方节点</p>
      </div>
    </div>

    <!-- 加载中 -->
    <div v-if="loading" class="text-center py-5">
      <div class="spinner-border" role="status"></div>
    </div>

    <!-- 添加/编辑模态框 -->
    <div class="modal fade" id="extNodeModal" tabindex="-1" ref="modalEl">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ editingId ? '编辑外部节点' : '添加外部节点' }}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <!-- 基本信息 -->
            <div class="row mb-3">
              <div class="col-md-8">
                <label class="form-label">节点名称</label>
                <input type="text" class="form-control" v-model="form.name" placeholder="RackNerd-US">
              </div>
              <div class="col-md-4">
                <label class="form-label">协议</label>
                <select class="form-select" v-model="form.protocol">
                  <option value="">请选择</option>
                  <option value="vmess">VMess</option>
                  <option value="vless">VLESS</option>
                  <option value="trojan">Trojan</option>
                  <option value="shadowsocks">Shadowsocks</option>
                  <option value="hysteria2">Hysteria2</option>
                  <option value="anytls">AnyTLS</option>
                </select>
              </div>
            </div>

            <div class="row mb-3">
              <div class="col-md-6">
                <label class="form-label">主机地址</label>
                <input type="text" class="form-control" v-model="form.host" placeholder="1.2.3.4 或 example.com">
              </div>
              <div class="col-md-3">
                <label class="form-label">端口</label>
                <input type="number" class="form-control" v-model.number="form.port">
              </div>
              <div class="col-md-3">
                <label class="form-label">级别</label>
                <input type="number" class="form-control" v-model.number="form.level" min="1" max="5">
              </div>
            </div>

            <!-- 认证信息 -->
            <div class="mb-3" v-if="form.protocol">
              <label class="form-label">{{ protocolAuthLabel }}</label>
              <input type="text" class="form-control" v-model="form.uuid" :placeholder="protocolAuthPlaceholder">
            </div>

            <!-- VMess/VLESS/Trojan 特有 -->
            <div v-if="['vmess','vless','trojan','anytls'].includes(form.protocol)" class="card mb-3">
              <div class="card-header">TLS 设置</div>
              <div class="card-body">
                <div class="form-check form-switch mb-2">
                  <input class="form-check-input" type="checkbox" v-model="form.tls_enabled" id="extTlsEnabled">
                  <label class="form-check-label" for="extTlsEnabled">启用 TLS</label>
                </div>
                <template v-if="form.tls_enabled">
                  <div class="row mb-2">
                    <div class="col-md-6">
                      <label class="form-label">Server Name (SNI)</label>
                      <input type="text" class="form-control" v-model="form.server_name">
                    </div>
                    <div class="col-md-6">
                      <label class="form-label">ALPN</label>
                      <input type="text" class="form-control" v-model="form.alpn" placeholder="h2,http/1.1">
                    </div>
                  </div>
                  <div class="form-check form-switch mb-2" v-if="form.protocol === 'vless'">
                    <input class="form-check-input" type="checkbox" v-model="form.reality_enabled" id="extReality">
                    <label class="form-check-label" for="extReality">REALITY</label>
                  </div>
                  <template v-if="form.reality_enabled">
                    <div class="row mb-2">
                      <div class="col-md-4">
                        <label class="form-label">PubKey</label>
                        <input type="text" class="form-control" v-model="form.reality_pubkey">
                      </div>
                      <div class="col-md-4">
                        <label class="form-label">Short ID</label>
                        <input type="text" class="form-control" v-model="form.reality_short_id">
                      </div>
                      <div class="col-md-4">
                        <label class="form-label">Flow</label>
                        <input type="text" class="form-control" v-model="form.flow">
                      </div>
                    </div>
                  </template>
                </template>
              </div>
            </div>

            <!-- Shadowsocks 特有 -->
            <div v-if="form.protocol === 'shadowsocks'" class="card mb-3">
              <div class="card-header">Shadowsocks 设置</div>
              <div class="card-body">
                <div class="row mb-2">
                  <div class="col-md-6">
                    <label class="form-label">加密方式</label>
                    <select class="form-select" v-model="form.ss_method">
                      <option value="">请选择</option>
                      <option value="2022-blake3-aes-256-gcm">2022-blake3-aes-256-gcm</option>
                      <option value="2022-blake3-aes-128-gcm">2022-blake3-aes-128-gcm</option>
                      <option value="2022-blake3-chacha20-poly1305">2022-blake3-chacha20-poly1305</option>
                      <option value="aes-256-gcm">aes-256-gcm</option>
                      <option value="aes-128-gcm">aes-128-gcm</option>
                      <option value="chacha20-ietf-poly1305">chacha20-ietf-poly1305</option>
                    </select>
                  </div>
                  <div class="col-md-6">
                    <label class="form-label">密码</label>
                    <input type="text" class="form-control" v-model="form.ss_password">
                  </div>
                </div>
                <div class="row mb-2">
                  <div class="col-md-6">
                    <label class="form-label">混淆模式 (reF1nd)</label>
                    <select class="form-select" v-model="form.ss_obfs_mode">
                      <option value="">不混淆</option>
                      <option value="tls">TLS (simple-obfs)</option>
                      <option value="http">HTTP (simple-obfs)</option>
                    </select>
                  </div>
                  <div class="col-md-6" v-if="form.ss_obfs_mode">
                    <label class="form-label">伪装域名</label>
                    <input type="text" class="form-control" v-model="form.ss_obfs_host" placeholder="gw.alicdn.com">
                  </div>
                </div>
              </div>
            </div>

            <!-- Hysteria2 特有 -->
            <div v-if="form.protocol === 'hysteria2'" class="card mb-3">
              <div class="card-header">Hysteria2 设置</div>
              <div class="card-body">
                <div class="row mb-2">
                  <div class="col-md-6">
                    <label class="form-label">密码</label>
                    <input type="text" class="form-control" v-model="form.hy2_password">
                  </div>
                </div>
                <div class="row mb-2">
                  <div class="col-md-6">
                    <label class="form-label">上行速度 (Mbps)</label>
                    <input type="number" class="form-control" v-model.number="form.hy2_up_mbps" min="1">
                  </div>
                  <div class="col-md-6">
                    <label class="form-label">下行速度 (Mbps)</label>
                    <input type="number" class="form-control" v-model.number="form.hy2_down_mbps" min="1">
                  </div>
                </div>
                <div class="row mb-2">
                  <div class="col-md-6">
                    <label class="form-label">混淆类型</label>
                    <select class="form-select" v-model="form.hy2_obfs">
                      <option value="">无</option>
                      <option value="salamander">salamander</option>
                    </select>
                  </div>
                  <div class="col-md-6" v-if="form.hy2_obfs === 'salamander'">
                    <label class="form-label">混淆密码</label>
                    <input type="text" class="form-control" v-model="form.hy2_obfs_password">
                  </div>
                </div>
              </div>
            </div>

            <!-- 传输层设置 -->
            <div v-if="['vmess','vless','trojan'].includes(form.protocol)" class="card mb-3">
              <div class="card-header">传输层设置</div>
              <div class="card-body">
                <div class="form-check form-switch mb-2">
                  <input class="form-check-input" type="checkbox" v-model="form.transport_enabled" id="extTransport">
                  <label class="form-check-label" for="extTransport">启用传输层</label>
                </div>
                <template v-if="form.transport_enabled">
                  <div class="row mb-2">
                    <div class="col-md-6">
                      <label class="form-label">传输类型</label>
                      <select class="form-select" v-model="form.transport_type">
                        <option value="ws">WebSocket</option>
                        <option value="grpc">gRPC</option>
                        <option value="httpupgrade">HTTPUpgrade</option>
                      </select>
                    </div>
                    <div class="col-md-6">
                      <label class="form-label">Host (transport)</label>
                      <input type="text" class="form-control" v-model="form.transport_host">
                    </div>
                  </div>
                  <div class="row mb-2">
                    <div class="col-md-6">
                      <label class="form-label">Path</label>
                      <input type="text" class="form-control" v-model="form.ws_path" placeholder="/ws">
                    </div>
                    <div class="col-md-6" v-if="form.transport_type === 'grpc'">
                      <label class="form-label">Service Name</label>
                      <input type="text" class="form-control" v-model="form.grpc_service">
                    </div>
                  </div>
                </template>
              </div>
            </div>

            <!-- 其他 -->
            <div class="row mb-3">
              <div class="col-md-4">
                <label class="form-label">国家代码</label>
                <input type="text" class="form-control" v-model="form.country" placeholder="US" maxlength="2">
              </div>
              <div class="col-md-4">
                <label class="form-label">排序</label>
                <input type="number" class="form-control" v-model.number="form.sort_order">
              </div>
              <div class="col-md-4">
                <label class="form-label">状态</label>
                <div class="form-check form-switch mt-2">
                  <input class="form-check-input" type="checkbox" v-model="form.enabled" id="extEnabled">
                  <label class="form-check-label" for="extEnabled">启用</label>
                </div>
              </div>
            </div>
            <div class="mb-3">
              <label class="form-label">备注</label>
              <input type="text" class="form-control" v-model="form.notes" placeholder="可选备注">
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">取消</button>
            <button type="button" class="btn btn-primary" @click="saveNode" :disabled="saving">
              <span v-if="saving" class="spinner-border spinner-border-sm me-1"></span>
              <i v-else class="bi bi-check-lg"></i>
              保存
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- 删除确认 -->
    <div class="modal fade" id="deleteModal" tabindex="-1" ref="deleteModalEl">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">确认删除</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <p>确定要删除外部节点 <strong>{{ deleteTarget?.name }}</strong> 吗？此操作不可撤销。</p>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">取消</button>
            <button type="button" class="btn btn-danger" @click="doDelete" :disabled="saving">
              <span v-if="saving" class="spinner-border spinner-border-sm me-1"></span>
              删除
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, inject } from 'vue'
import { Modal } from 'bootstrap'
import { getExternalNodes, createExternalNode, updateExternalNode, deleteExternalNode, type ExternalNode } from '../api'

const showToast = inject<any>('showToast')

const nodes = ref<ExternalNode[]>([])
const loading = ref(true)
const saving = ref(false)
const editingId = ref<number | null>(null)
const deleteTarget = ref<ExternalNode | null>(null)

let modal: Modal | null = null
let deleteModal: Modal | null = null
const modalEl = ref<HTMLElement | null>(null)
const deleteModalEl = ref<HTMLElement | null>(null)

const defaultForm = (): Partial<ExternalNode> => ({
  name: '',
  protocol: '',
  host: '',
  port: 443,
  uuid: '',
  tls_enabled: false,
  server_name: '',
  alpn: '',
  reality_enabled: false,
  reality_pubkey: '',
  reality_short_id: '',
  transport_enabled: false,
  transport_type: 'ws',
  ws_path: '',
  grpc_service: '',
  transport_host: '',
  flow: '',
  ss_method: '',
  ss_password: '',
  ss_obfs_mode: '',
  ss_obfs_host: '',
  hy2_password: '',
  hy2_up_mbps: 100,
  hy2_down_mbps: 100,
  hy2_obfs: '',
  hy2_obfs_password: '',
  level: 1,
  enabled: true,
  sort_order: 0,
  country: '',
  notes: ''
})

const form = ref<Partial<ExternalNode>>(defaultForm())

const protocolAuthLabel = computed(() => {
  const m: Record<string, string> = {
    vmess: 'UUID',
    vless: 'UUID',
    trojan: '密码',
    shadowsocks: 'UUID (用户标识)',
    hysteria2: 'UUID (用户标识)',
    anytls: '密码'
  }
  return m[form.value.protocol || ''] || '认证信息'
})

const protocolAuthPlaceholder = computed(() => {
  const m: Record<string, string> = {
    vmess: '00000000-0000-0000-0000-000000000000',
    vless: '00000000-0000-0000-0000-000000000000',
    trojan: 'your-password',
    anytls: 'your-password',
    shadowsocks: '00000000-0000-...',
    hysteria2: '00000000-0000-...'
  }
  return m[form.value.protocol || ''] || ''
})

const loadNodes = async () => {
  loading.value = true
  try {
    const res = await getExternalNodes()
    nodes.value = res.data.data || []
  } catch (e: any) {
    showToast?.('error', '错误', '加载外部节点失败')
  } finally {
    loading.value = false
  }
}

const openAddModal = () => {
  editingId.value = null
  form.value = defaultForm()
  if (modal) modal.show()
}

const openEditModal = (node: ExternalNode) => {
  editingId.value = node.id
  form.value = { ...node }
  if (modal) modal.show()
}

const saveNode = async () => {
  if (!form.value.name || !form.value.protocol || !form.value.host || !form.value.port) {
    showToast?.('error', '错误', '请填写名称、协议、主机和端口')
    return
  }
  saving.value = true
  try {
    if (editingId.value) {
      await updateExternalNode(editingId.value, form.value)
      showToast?.('success', '成功', '外部节点已更新')
    } else {
      await createExternalNode(form.value)
      showToast?.('success', '成功', '外部节点已添加')
    }
    modal?.hide()
    await loadNodes()
  } catch (e: any) {
    showToast?.('error', '错误', e.response?.data?.error || '保存失败')
  } finally {
    saving.value = false
  }
}

const confirmDelete = (node: ExternalNode) => {
  deleteTarget.value = node
  if (deleteModal) deleteModal.show()
}

const doDelete = async () => {
  if (!deleteTarget.value) return
  saving.value = true
  try {
    await deleteExternalNode(deleteTarget.value.id)
    showToast?.('success', '成功', '外部节点已删除')
    deleteModal?.hide()
    await loadNodes()
  } catch (e: any) {
    showToast?.('error', '错误', e.response?.data?.error || '删除失败')
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  loadNodes()
  if (modalEl.value) modal = new Modal(modalEl.value)
  if (deleteModalEl.value) deleteModal = new Modal(deleteModalEl.value)
})
</script>
