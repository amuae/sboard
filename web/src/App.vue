<template>
  <div id="app">
    <!-- 导航栏 - 仅登录后显示 -->
    <nav v-if="isLoggedIn" class="navbar navbar-expand-lg navbar-dark bg-dark">
      <div class="container-fluid">
        <a class="navbar-brand" href="#">SBoard</a>
        <button class="navbar-toggler" type="button" @click="navCollapsed = !navCollapsed">
          <span class="navbar-toggler-icon"></span>
        </button>
        <div class="navbar-collapse" :class="{ collapse: navCollapsed, show: !navCollapsed }" id="navbarNav">
          <ul class="navbar-nav me-auto">
            <li class="nav-item">
              <router-link class="nav-link" :class="{ active: $route.path === '/users' }" to="/users" @click="closeNav">
                <i class="bi bi-people"></i> 用户管理
              </router-link>
            </li>
            <li class="nav-item">
              <router-link class="nav-link" :class="{ active: $route.path === '/nodes' }" to="/nodes" @click="closeNav">
                <i class="bi bi-diagram-3"></i> 节点管理
              </router-link>
            </li>
            <li class="nav-item">
              <router-link class="nav-link" :class="{ active: $route.path === '/servers' }" to="/servers" @click="closeNav">
                <i class="bi bi-hdd-stack"></i> 服务器管理
              </router-link>
            </li>
          </ul>
          <ul class="navbar-nav">
            <li class="nav-item dropdown">
              <a class="nav-link dropdown-toggle" href="#" role="button" @click.prevent="dropdownOpen = !dropdownOpen">
                <i class="bi bi-person-circle"></i> 管理员
              </a>
              <ul class="dropdown-menu dropdown-menu-end" :class="{ show: dropdownOpen }">
                <li><a class="dropdown-item" href="#" @click.prevent="openAccountSettings">
                  <i class="bi bi-gear"></i> 账户设置
                </a></li>
                <li><hr class="dropdown-divider"></li>
                <li><a class="dropdown-item" href="#" @click.prevent="handleLogout">
                  <i class="bi bi-box-arrow-right"></i> 退出登录
                </a></li>
              </ul>
            </li>
          </ul>
        </div>
      </div>
    </nav>

    <!-- 主内容区 -->
    <router-view @login-success="onLoginSuccess"></router-view>

    <!-- Toast 通知 -->
    <div class="toast-container position-fixed top-0 end-0 p-3">
      <div 
        ref="toastEl" 
        class="toast" 
        :class="toastClass" 
        role="alert"
      >
        <div class="toast-header">
          <strong class="me-auto">{{ toastTitle }}</strong>
          <button type="button" class="btn-close" data-bs-dismiss="toast"></button>
        </div>
        <div class="toast-body">{{ toastMessage }}</div>
      </div>
    </div>

    <!-- 账户设置模态框 -->
    <div class="modal fade" id="accountModal" tabindex="-1" ref="accountModalEl">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title"><i class="bi bi-gear"></i> 账户设置</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <ul class="nav nav-tabs mb-3" role="tablist">
              <li class="nav-item" role="presentation">
                <button 
                  class="nav-link" 
                  :class="{ active: accountTab === 'username' }"
                  @click="accountTab = 'username'"
                >修改用户名</button>
              </li>
              <li class="nav-item" role="presentation">
                <button 
                  class="nav-link" 
                  :class="{ active: accountTab === 'password' }"
                  @click="accountTab = 'password'"
                >修改密码</button>
              </li>
              <li class="nav-item" role="presentation">
                <button 
                  class="nav-link" 
                  :class="{ active: accountTab === 'oauth' }"
                  @click="accountTab = 'oauth'; loadOAuthSettings()"
                >OAuth 设置</button>
              </li>
            </ul>
            
            <!-- 修改用户名 -->
            <div v-if="accountTab === 'username'">
              <div class="mb-3">
                <label class="form-label">当前密码</label>
                <input type="password" class="form-control" v-model="currentPassword" placeholder="请输入当前密码">
              </div>
              <div class="mb-3">
                <label class="form-label">新用户名</label>
                <input type="text" class="form-control" v-model="newUsername" placeholder="请输入新用户名">
              </div>
            </div>
            
            <!-- 修改密码 -->
            <div v-if="accountTab === 'password'">
              <div class="mb-3">
                <label class="form-label">当前密码</label>
                <input type="password" class="form-control" v-model="currentPassword" placeholder="请输入当前密码">
              </div>
              <div class="mb-3">
                <label class="form-label">新密码</label>
                <input type="password" class="form-control" v-model="newPassword" placeholder="请输入新密码">
              </div>
              <div class="mb-3">
                <label class="form-label">确认新密码</label>
                <input type="password" class="form-control" v-model="confirmPassword" placeholder="请再次输入新密码">
              </div>
            </div>
            
            <!-- OAuth 设置 -->
            <div v-if="accountTab === 'oauth'">
              <div v-if="oauthLoading" class="text-center py-4">
                <div class="spinner-border spinner-border-sm" role="status"></div>
                <span class="ms-2">加载中...</span>
              </div>
              <div v-else>
                <!-- GitHub OAuth -->
                <div class="card mb-3">
                  <div class="card-header d-flex justify-content-between align-items-center">
                    <span><i class="bi bi-github"></i> GitHub OAuth</span>
                    <div class="form-check form-switch mb-0">
                      <input class="form-check-input" type="checkbox" id="githubEnabled" v-model="githubOAuth.enabled">
                      <label class="form-check-label" for="githubEnabled">启用</label>
                    </div>
                  </div>
                  <div class="card-body">
                    <div class="mb-3">
                      <label class="form-label">Client ID</label>
                      <input type="text" class="form-control" v-model="githubOAuth.client_id" placeholder="GitHub OAuth Client ID">
                    </div>
                    <div class="mb-3">
                      <label class="form-label">Client Secret</label>
                      <input 
                        type="password" 
                        class="form-control" 
                        v-model="githubOAuth.client_secret" 
                        :placeholder="githubOAuth.has_secret ? '已配置（留空保持不变）' : '请输入 Client Secret'"
                      >
                      <div class="form-text">
                        <i class="bi bi-shield-lock"></i> 
                        Client Secret 存储在数据库中，不会写入配置文件
                      </div>
                    </div>
                    <div class="mb-3">
                      <label class="form-label">允许的用户 <small class="text-muted">(留空允许所有)</small></label>
                      <input 
                        type="text" 
                        class="form-control" 
                        v-model="githubOAuth.allowed_users_str" 
                        placeholder="用逗号分隔多个 GitHub 用户名，如: user1,user2"
                      >
                      <div class="form-text">只有列表中的 GitHub 用户可以登录，留空则允许所有 GitHub 用户</div>
                    </div>
                    <div class="alert alert-info mb-0">
                      <i class="bi bi-info-circle"></i>
                      <strong>配置说明:</strong>
                      <ol class="mb-0 ps-3 mt-2">
                        <li>前往 <a href="https://github.com/settings/developers" target="_blank">GitHub Developer Settings</a></li>
                        <li>创建 OAuth App，Callback URL 设置为: <code>{{ callbackUrl }}</code></li>
                        <li>将生成的 Client ID 和 Client Secret 填入上方</li>
                      </ol>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">取消</button>
            <button type="button" class="btn btn-primary" @click="saveAccountSettings">
              <i class="bi bi-check-lg"></i> 保存
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, nextTick, provide } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { Modal, Toast } from 'bootstrap'
import { logout as apiLogout, changeUsername, changePassword, getOAuthProvidersAdmin, saveOAuthProvider } from './api'

const router = useRouter()
const route = useRoute()

// 登录状态
const isLoggedIn = ref(!!localStorage.getItem('token'))

// 导航栏状态
const navCollapsed = ref(true)
const dropdownOpen = ref(false)

const closeNav = () => {
  navCollapsed.value = true
  dropdownOpen.value = false
}

const openAccountSettings = () => {
  dropdownOpen.value = false
  showAccountModal.value = true
}

const handleLogout = () => {
  dropdownOpen.value = false
  logout()
}

// 点击页面其他地方关闭下拉菜单
const handleClickOutside = (e: MouseEvent) => {
  const target = e.target as HTMLElement
  if (!target.closest('.dropdown')) {
    dropdownOpen.value = false
  }
}

// Toast 相关
const toastEl = ref<HTMLElement | null>(null)
const toastTitle = ref('')
const toastMessage = ref('')
const toastType = ref<'success' | 'error' | 'warning' | 'info'>('info')
let toastInstance: Toast | null = null

const toastClass = computed(() => ({
  'bg-success text-white': toastType.value === 'success',
  'bg-danger text-white': toastType.value === 'error',
  'bg-warning': toastType.value === 'warning',
  'bg-info text-white': toastType.value === 'info'
}))

const showToast = (type: 'success' | 'error' | 'warning' | 'info', title: string, message: string) => {
  toastType.value = type
  toastTitle.value = title
  toastMessage.value = message
  if (toastInstance) {
    toastInstance.show()
  }
}

// 提供给子组件使用
provide('showToast', showToast)

// 账户设置模态框
const accountModalEl = ref<HTMLElement | null>(null)
const showAccountModal = ref(false)
const accountTab = ref<'username' | 'password' | 'oauth'>('username')
const newUsername = ref('')
const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
let accountModal: Modal | null = null

// OAuth 设置
const oauthLoading = ref(false)
const githubOAuth = ref({
  enabled: false,
  client_id: '',
  client_secret: '',
  has_secret: false,
  allowed_users: [] as string[],
  allowed_users_str: ''
})

// 计算回调 URL
const callbackUrl = computed(() => {
  return `${window.location.origin}/api/auth/github/callback`
})

// 加载 OAuth 设置
const loadOAuthSettings = async () => {
  oauthLoading.value = true
  try {
    const res = await getOAuthProvidersAdmin()
    if (res.data.success && res.data.data) {
      const github = res.data.data.find((p: any) => p.name === 'github')
      if (github) {
        githubOAuth.value = {
          enabled: github.enabled,
          client_id: github.config?.client_id || '',
          client_secret: '',
          has_secret: github.config?.has_secret || false,
          allowed_users: github.config?.allowed_users || [],
          allowed_users_str: (github.config?.allowed_users || []).join(', ')
        }
      }
    }
  } catch (error: any) {
    showToast('error', '错误', '加载 OAuth 设置失败')
  } finally {
    oauthLoading.value = false
  }
}

watch(showAccountModal, (val) => {
  if (val && accountModal) {
    accountModal.show()
  }
})

const resetAccountForm = () => {
  newUsername.value = ''
  currentPassword.value = ''
  newPassword.value = ''
  confirmPassword.value = ''
  accountTab.value = 'username'
  // 重置 OAuth 表单
  githubOAuth.value = {
    enabled: false,
    client_id: '',
    client_secret: '',
    has_secret: false,
    allowed_users: [],
    allowed_users_str: ''
  }
}

const saveAccountSettings = async () => {
  try {
    if (accountTab.value === 'username') {
      if (!currentPassword.value) {
        showToast('error', '错误', '请输入当前密码')
        return
      }
      if (!newUsername.value.trim()) {
        showToast('error', '错误', '请输入新用户名')
        return
      }
      await changeUsername(currentPassword.value, newUsername.value)
      showToast('success', '成功', '用户名修改成功')
    } else if (accountTab.value === 'password') {
      if (!currentPassword.value || !newPassword.value || !confirmPassword.value) {
        showToast('error', '错误', '请填写所有密码字段')
        return
      }
      if (newPassword.value !== confirmPassword.value) {
        showToast('error', '错误', '两次输入的新密码不一致')
        return
      }
      await changePassword(currentPassword.value, newPassword.value, confirmPassword.value)
      showToast('success', '成功', '密码修改成功')
    } else if (accountTab.value === 'oauth') {
      // 保存 OAuth 设置
      const allowedUsers = githubOAuth.value.allowed_users_str
        .split(/[,，]/)
        .map(s => s.trim())
        .filter(s => s.length > 0)
      
      await saveOAuthProvider('github', {
        enabled: githubOAuth.value.enabled,
        client_id: githubOAuth.value.client_id,
        client_secret: githubOAuth.value.client_secret || undefined,
        allowed_users: allowedUsers
      })
      showToast('success', '成功', 'OAuth 设置已保存')
    }
    if (accountModal) {
      accountModal.hide()
    }
    resetAccountForm()
  } catch (error: any) {
    showToast('error', '错误', error.response?.data?.error || '操作失败')
  }
}

const logout = async () => {
  try {
    await apiLogout()
  } catch (e) {
    // 忽略错误
  }
  localStorage.removeItem('token')
  isLoggedIn.value = false
  router.push('/login')
}

const onLoginSuccess = () => {
  isLoggedIn.value = true
}

// 处理 OAuth 登录回调
const handleOAuthCallback = () => {
  const urlParams = new URLSearchParams(window.location.search)
  const oauthToken = urlParams.get('oauth_token')
  if (oauthToken) {
    localStorage.setItem('token', oauthToken)
    isLoggedIn.value = true
    // 清除 URL 参数
    window.history.replaceState({}, document.title, window.location.pathname)
    // 跳转到用户管理页面
    router.push('/users')
  }
}

onMounted(() => {
  // 处理 OAuth 登录回调
  handleOAuthCallback()
  
  // 初始化 Toast
  nextTick(() => {
    if (toastEl.value) {
      toastInstance = new Toast(toastEl.value, { autohide: true, delay: 3000 })
    }
    if (accountModalEl.value) {
      accountModal = new Modal(accountModalEl.value)
      accountModalEl.value.addEventListener('hidden.bs.modal', () => {
        showAccountModal.value = false
        resetAccountForm()
      })
    }
  })
  // 点击外部关闭下拉菜单
  document.addEventListener('click', handleClickOutside)
})
</script>
