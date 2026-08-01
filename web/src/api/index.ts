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
export interface ApiResponse<T> {
  success: boolean
  data: T
  message?: string
}

interface OAuthProvidersResponse {
  success: boolean
  data: OAuthProviderConfig[]
  disable_password_login: boolean
}

export const login = (username: string, password: string) => {
  return api.post<ApiResponse<{ token: string; username: string }>>('/auth/login', { username, password })
}

export const logout = () => {
  return api.post('/auth/logout')
}

// OAuth 相关
export const getOAuthProviders = () => {
  return api.get<OAuthProvidersResponse>('/auth/oauth/providers')
}

export const getGitHubLoginUrl = (authorizeMode: boolean = false) => {
  return api.get<ApiResponse<{ url: string }>>('/auth/github/login', {
    params: { authorize: authorizeMode }
  })
}

// OAuth 管理 API
export interface OAuthProviderConfig {
  name: string
  label: string
  enabled: boolean
  config?: {
    client_id: string
    has_secret: boolean
    allowed_users: string[]
  }
}

export const getOAuthProvidersAdmin = () => {
	return api.get<OAuthProvidersResponse>('/oauth/providers')
}

export const getOAuthProvider = (name: string) => {
  return api.get<{ success: boolean; data: OAuthProviderConfig }>(`/oauth/providers/${name}`)
}

export const saveOAuthProvider = (name: string, data: {
  enabled: boolean
  client_id: string
  client_secret?: string
  allowed_users: string[]
}) => {
  return api.post(`/oauth/providers/${name}`, data)
}

export const saveOAuthSettings = (data: {
  disable_password_login: boolean
}) => {
  return api.post('/oauth/settings', data)
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
  enabled: number
  traffic_limit: number
  traffic_used: number
  dns_resolve: string
  notes: string
  created_at: string
  updated_at: string
}

interface UsersResponse {
  users: User[]
  grouped: {
    level_1: User[]
    level_2: User[]
    level_3: User[]
  }
  stats: {
    total: number
    enabled: number
  }
}

export const getUsers = () => {
	return api.get<ApiResponse<UsersResponse>>('/users')
}

export const createUser = (user: Partial<User>) => {
	return api.post<ApiResponse<User>>('/users', user)
}

export const updateUser = (id: number, user: Partial<User>) => {
	return api.put<ApiResponse<User>>(`/users/${id}`, user)
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
  ss_obfs_mode: string
  ss_obfs_host: string
  hy2_password: string
  hy2_up_mbps: number
  hy2_down_mbps: number
  hy2_obfs: string
  hy2_obfs_password: string
  notes: string
  enabled: boolean
  created_at: string
  updated_at: string
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
  return api.post<ApiResponse<{
    private_key: string
    public_key: string
    short_id: string
  }>>('/nodes/generate-reality-keys')
}

export const getNodeConfig = () => {
	return api.get<ApiResponse<{ type: string; config: string }>>('/nodes/config')
}

export const getConfig = (type: string) => {
	return api.get<ApiResponse<{ type: string; server?: string; config: string }>>(`/config/preview?type=${type}`)
}

// ============ 服务器管理 ============
export interface Server {
  id: number
  name: string
  host: string
  host_ipv6: string
  node_domain: string  // 节点域名（用于订阅 DNS 解析）
  port: number
  category: string
  node_1: string
  node_2: string
  node_3: string
  dns_resolve: string
  enabled: number
  sort_order: number
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
  // 流量统计
  monthly_in: number
  monthly_out: number
  traffic_reset: string
  // 其他
  notes: string
  last_deploy_at: string
  created_at: string
  updated_at: string
}

interface ServersResponse {
  servers: Server[]
  grouped: {
    direct: Server[]
    relay: Server[]
    home: Server[]
  }
  stats: {
    total: number
    enabled: number
  }
}

export interface NodeConfig {
  id: number
  server_id: number
  node_id: number
  listen_port: number
  forward_enabled: boolean
  forward_host: string
  forward_port: number
  outbound_enabled: boolean
  outbound_protocol: string
  outbound_host: string
  outbound_port: number
  outbound_password: string
  outbound_method: string
  outbound_username: string
  outbound_sni: string
  // VLESS/VMess 配置
  outbound_uuid: string
  outbound_flow: string
  outbound_security: string
  outbound_alter_id: number
  outbound_tls: boolean
  outbound_reality: boolean
  outbound_pub_key: string
  outbound_short_id: string
  outbound_fp: string
  // Hysteria2 配置
  outbound_obfs: string
  outbound_obfs_pwd: string
  // 传输层配置 (VMess/VLESS)
  outbound_network: string
  outbound_ws_path: string
  outbound_ws_host: string
  created_at: string
  updated_at: string
  node?: Node
}

export const getServers = () => {
	return api.get<ApiResponse<ServersResponse>>('/servers')
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
	return api.get<ApiResponse<{ statuses: ServerStatus[] }>>('/servers/status')
}

export const createServer = (server: Partial<Server>) => {
	return api.post<ApiResponse<Server>>('/servers', server)
}

export const updateServer = (id: number, server: Partial<Server>) => {
	return api.put<ApiResponse<Server>>(`/servers/${id}`, server)
}

export const deleteServer = (id: number) => {
  return api.delete(`/servers/${id}`)
}

export const reorderServers = (ids: number[]) => {
  return api.post('/servers/reorder', { ids })
}

export const getNodeConfigs = (serverId: number) => {
	return api.get<ApiResponse<Record<string, NodeConfig>>>(`/servers/${serverId}/node-configs`)
}

export const saveNodeConfig = (serverId: number, nodeId: number, config: Partial<NodeConfig>) => {
	return api.post<ApiResponse<unknown>>(`/servers/${serverId}/node-configs/${nodeId}`, config)
}

export const deleteNodeConfig = (serverId: number, nodeId: number) => {
  return api.delete(`/servers/${serverId}/node-configs/${nodeId}`)
}

// 落地出站相关
export interface ServerOutbound {
  id: number
  server_id: number
  slot: number
  enabled: boolean
  remark: string
  protocol: string
  host: string
  port: number
  password: string
  method: string
  username: string
  sni: string
  // VLESS/VMess 配置
  uuid: string
  flow: string
  security: string
  alter_id: number
  tls: boolean
  reality: boolean
  pub_key: string
  short_id: string
  fp: string
  // Hysteria2 配置
  obfs: string
  obfs_pwd: string
  // 传输层配置
  network: string
  ws_path: string
  ws_host: string
  created_at: string
  updated_at: string
}

export const getServerOutbounds = (serverId: number) => {
	return api.get<ApiResponse<ServerOutbound[]>>(`/servers/${serverId}/outbounds`)
}

export const createServerOutbound = (serverId: number, outbound: Partial<ServerOutbound>) => {
	return api.post<ApiResponse<ServerOutbound>>(`/servers/${serverId}/outbounds`, outbound)
}

export const updateServerOutbound = (serverId: number, slot: number, outbound: Partial<ServerOutbound>) => {
	return api.put<ApiResponse<ServerOutbound>>(`/servers/${serverId}/outbounds/${slot}`, outbound)
}

export const deleteServerOutbound = (serverId: number, slot: number) => {
  return api.delete(`/servers/${serverId}/outbounds/${slot}`)
}

export const toggleServerOutbound = (serverId: number, slot: number) => {
	return api.post<ApiResponse<ServerOutbound>>(`/servers/${serverId}/outbounds/${slot}/toggle`)
}

// 部署相关
export const deployServer = (serverId: number, type: string) => {
	return api.post<ApiResponse<{ output?: string; data?: unknown }>>(`/servers/${serverId}/deploy`, { type })
}

// 全部部署（向所有存活 Agent 发送部署核心指令）
export const deployAll = () => {
	return api.post<ApiResponse<{ message: string; total: number }>>('/servers/deploy-all')
}

// Agent 更新（向所有存活 Agent 发送自我更新指令）
export const updateAllAgents = () => {
	return api.post<ApiResponse<{ message: string; total: number }>>('/servers/update-agents')
}

// Agent 相关
export const regenerateAgentToken = (serverId: number) => {
	return api.post<ApiResponse<{ agent_token: string }>>(`/servers/${serverId}/agent/regenerate-token`)
}

export const sendAgentCommand = (serverId: number, command: string) => {
  return api.post(`/servers/${serverId}/agent/command`, { command })
}

export const getAgentStatus = (serverId: number) => {
	return api.get<ApiResponse<{ online: boolean; status: unknown }>>(`/servers/${serverId}/agent/status`)
}

// ============ 订阅配置相关 ============
export interface SubscriptionConfigs {
  mihomoConfigs: MihomoConfig[]
  singboxConfigs: SingBoxConfig[]
}

export interface MihomoConfig {
  id: number
  name: string
  description: string
  enabled: boolean
  modules: string[]
  config: any
}

export interface SingBoxConfig {
  id: number
  name: string
  description: string
  enabled: boolean
  modules: string[]
  config: any
}

export const getSubscriptionConfigs = () => {
  return api.get<SubscriptionConfigs>('/subscription/configs')
}

export const saveSubscriptionConfigs = (data: SubscriptionConfigs) => {
  return api.post('/subscription/configs', data)
}

// ============ 外部节点管理 ============
export interface ExternalNode {
  id: number
  name: string
  protocol: string
  host: string
  port: number
  uuid: string
  tls_enabled: boolean
  server_name: string
  alpn: string
  reality_enabled: boolean
  reality_server: string
  reality_pubkey: string
  reality_short_id: string
  transport_enabled: boolean
  transport_type: string
  ws_path: string
  grpc_service: string
  transport_host: string
  flow: string
  ss_method: string
  ss_password: string
  ss_obfs_mode: string
  ss_obfs_host: string
  hy2_password: string
  hy2_up_mbps: number
  hy2_down_mbps: number
  hy2_obfs: string
  hy2_obfs_password: string
  level: number
  enabled: boolean
  sort_order: number
  country: string
  notes: string
  created_at: string
  updated_at: string
}

export const getExternalNodes = () => api.get<ApiResponse<ExternalNode[]>>('/external-nodes')
export const createExternalNode = (data: Partial<ExternalNode>) => api.post<ApiResponse<ExternalNode>>('/external-nodes', data)
export const updateExternalNode = (id: number, data: Partial<ExternalNode>) => api.put<ApiResponse<ExternalNode>>(`/external-nodes/${id}`, data)
export const deleteExternalNode = (id: number) => api.delete(`/external-nodes/${id}`)

export default api
