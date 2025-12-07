import axios from 'axios'
import router from '@/router'

// 创建 axios 实例
const api = axios.create({
  baseURL: '/api',
  timeout: 30000
})

// 请求拦截器
api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// 响应拦截器
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token')
      router.push('/login')
    }
    return Promise.reject(error)
  }
)

// ============ 认证相关 ============
export const login = (username: string, password: string) => {
  return api.post('/auth/login', { username, password })
}

export const logout = () => {
  return api.post('/auth/logout')
}

export const changeUsername = (password: string, newUsername: string) => {
  return api.post('/auth/change-username', { password, new_username: newUsername })
}

export const changePassword = (oldPassword: string, newPassword: string, confirmPassword: string) => {
  return api.post('/auth/change-password', { 
    old_password: oldPassword, 
    new_password: newPassword,
    confirm_password: confirmPassword
  })
}

// ============ 用户管理 ============
export interface User {
  id: number
  name: string
  uuid: string
  level: number
  expiry_date: string
  enabled: boolean
  traffic_limit: number
  traffic_used: number
  created_at: string
  updated_at: string
}

export const getUsers = () => {
  return api.get('/users')
}

export const createUser = (user: Partial<User>) => {
  return api.post<User>('/users', user)
}

export const updateUser = (id: number, user: Partial<User>) => {
  return api.put<User>(`/users/${id}`, user)
}

export const deleteUser = (id: number) => {
  return api.delete(`/users/${id}`)
}

// ============ 节点管理 ============
export interface Node {
  id: number
  tag: string
  protocol: string
  listen: string
  port: number
  tls_enabled: boolean
  server_name: string
  cert_path: string
  key_path: string
  reality_enabled: boolean
  reality_server: string
  reality_pubkey: string
  reality_privkey: string
  reality_short_id: string
  transport_enabled: boolean
  transport_type: string
  ws_path: string
  transport_host: string
  grpc_service: string
  flow: string
  ss_method: string
  ss_password: string
  hy2_password: string
  hy2_up_mbps: number
  hy2_down_mbps: number
  hy2_obfs_enabled: boolean
  hy2_obfs_password: string
  notes: string
  enabled: boolean
  created_at: string
  updated_at: string
}

// API 响应类型
interface ApiResponse<T> {
  success: boolean
  data: T
  message?: string
}

interface NodesResponse {
  nodes: Node[]
  stats: {
    total: number
    enabled: number
    protocols: { protocol: string; count: number }[]
  }
}

export const getNodes = () => {
  return api.get<ApiResponse<NodesResponse>>('/nodes')
}

export const createNode = (node: Partial<Node>) => {
  return api.post<ApiResponse<Node>>('/nodes', node)
}

export const updateNode = (id: number, node: Partial<Node>) => {
  return api.put<ApiResponse<Node>>(`/nodes/${id}`, node)
}

export const deleteNode = (id: number) => {
  return api.delete(`/nodes/${id}`)
}

export const generateRealityKeys = () => {
  return api.post<{ private_key: string; public_key: string }>('/nodes/generate-reality-keys')
}

export const getNodeConfig = () => {
  return api.get<any>('/nodes/config')
}

export const getConfig = (type: string) => {
  return api.get(`/config/preview?type=${type}`)
}

// ============ 服务器管理 ============
export interface Server {
  id: number
  name: string
  host: string
  node_domain: string  // 节点域名（用于订阅 DNS 解析）
  port: number
  category: string
  core_type: string
  node_1: string
  node_2: string
  node_3: string
  dns_resolve: string
  enabled: boolean
  // 部署模式
  deploy_mode: string  // ssh / agent
  ssh_user: string
  ssh_key_path: string
  // Agent 相关
  agent_token: string
  agent_id: string
  agent_version: string
  agent_online: boolean
  last_heartbeat: string
  cpu_usage: number
  mem_usage: number
  disk_usage: number
  net_in: number
  net_out: number
  // 其他
  notes: string
  last_deploy_at: string
  created_at: string
  updated_at: string
}

export interface NodeConfig {
  id: number
  server_id: number
  node_id: number
  config_type: string
  listen: string
  created_at: string
  updated_at: string
  node?: Node
}

export const getServers = () => {
  return api.get<Server[]>('/servers')
}

// 批量获取服务器状态（用于实时刷新）
export interface ServerStatus {
  id: number
  agent_online: boolean
  cpu_usage: number
  mem_usage: number
  disk_usage: number
  net_in: number   // bytes/s
  net_out: number  // bytes/s
  monthly_in: number   // 月度下行流量 (bytes)
  monthly_out: number  // 月度上行流量 (bytes)
}

export const getServersStatus = () => {
  return api.get<{ statuses: ServerStatus[] }>('/servers/status')
}

export const createServer = (server: Partial<Server>) => {
  return api.post<Server>('/servers', server)
}

export const updateServer = (id: number, server: Partial<Server>) => {
  return api.put<Server>(`/servers/${id}`, server)
}

export const deleteServer = (id: number) => {
  return api.delete(`/servers/${id}`)
}

export const reorderServers = (ids: number[]) => {
  return api.post('/servers/reorder', { ids })
}

export const getNodeConfigs = (serverId: number) => {
  return api.get<NodeConfig[]>(`/servers/${serverId}/node-configs`)
}

export const addNodeConfig = (serverId: number, config: Partial<NodeConfig>) => {
  return api.post<NodeConfig>(`/servers/${serverId}/node-configs`, config)
}

export const deleteNodeConfig = (serverId: number, nodeId: number) => {
  return api.delete(`/servers/${serverId}/node-configs/${nodeId}`)
}

// 部署相关
export const deployServer = (serverId: number, type: string) => {
  return api.post(`/servers/${serverId}/deploy`, { type })
}

// 部署相关 - SSE
export const getDeployUrl = (serverId: number) => {
  return `/api/servers/${serverId}/deploy`
}

export const getDeployFolderUrl = (serverId: number) => {
  return `/api/servers/${serverId}/deploy-folder`
}

// Agent 相关
export const regenerateAgentToken = (serverId: number) => {
  return api.post(`/servers/${serverId}/agent/regenerate-token`)
}

export const sendAgentCommand = (serverId: number, command: string) => {
  return api.post(`/servers/${serverId}/agent/command`, { command })
}

export const getAgentStatus = (serverId: number) => {
  return api.get(`/servers/${serverId}/agent/status`)
}

// ============ 设置相关 ============
export interface Settings {
  id: number
  admin_username: string
  admin_password: string
  sublink_domain: string
  created_at: string
  updated_at: string
}

export const getSettings = () => {
  return api.get<Settings>('/settings')
}

export const updateSettings = (settings: Partial<Settings>) => {
  return api.post('/settings', settings)
}

export default api
