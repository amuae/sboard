<template>
  <div class="login-wrapper">
    <div class="login-container">
      <div class="card login-card shadow-lg">
        <div class="card-body p-5">
          <div class="text-center mb-4">
            <h2 class="fw-bold text-primary">
              <i class="bi bi-shield-lock"></i> SBoard
            </h2>
            <p class="text-muted">管理面板登录</p>
          </div>
          
          <!-- 密码登录表单（如果未禁用） -->
          <form v-if="!disablePasswordLogin" @submit.prevent="handleLogin">
            <div class="mb-3">
              <label class="form-label">用户名</label>
              <div class="input-group">
                <span class="input-group-text"><i class="bi bi-person"></i></span>
                <input 
                  type="text" 
                  class="form-control" 
                  v-model="username" 
                  placeholder="请输入用户名"
                  required
                >
              </div>
            </div>
            
            <div class="mb-4">
              <label class="form-label">密码</label>
              <div class="input-group">
                <span class="input-group-text"><i class="bi bi-lock"></i></span>
                <input 
                  type="password" 
                  class="form-control" 
                  v-model="password" 
                  placeholder="请输入密码"
                  required
                >
              </div>
            </div>
            
            <div v-if="error" class="alert alert-danger py-2 mb-3">
              <i class="bi bi-exclamation-circle"></i> {{ error }}
            </div>
            
            <button type="submit" class="btn btn-primary w-100" :disabled="loading">
              <span v-if="loading" class="spinner-border spinner-border-sm me-2"></span>
              <i v-else class="bi bi-box-arrow-in-right me-2"></i>
              {{ loading ? '登录中...' : '登录' }}
            </button>
          </form>

          <!-- 密码登录已禁用提示 -->
          <div v-if="disablePasswordLogin && oauthProviders.length > 0" class="text-center mb-3">
            <div v-if="error" class="alert alert-danger py-2 mb-3">
              <i class="bi bi-exclamation-circle"></i> {{ error }}
            </div>
            <p class="text-muted mb-0">
              <i class="bi bi-info-circle me-1"></i>
              密码登录已禁用，请使用以下方式登录
            </p>
          </div>

          <!-- OAuth 登录分隔线 -->
          <div v-if="oauthProviders.length > 0 && !disablePasswordLogin" class="divider my-4">
            <span class="divider-text">或</span>
          </div>

          <!-- OAuth 提供商按钮 -->
          <div v-if="oauthProviders.length > 0" class="oauth-buttons">
            <button 
              v-for="provider in oauthProviders"
              :key="provider.name"
              type="button"
              class="btn btn-outline-secondary w-100 mb-2"
              :disabled="oauthLoading"
              @click="handleOAuthLogin(provider.name)"
            >
              <i :class="'bi bi-' + provider.icon + ' me-2'"></i>
              使用 {{ provider.label }} 登录
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { login, getOAuthProviders, getGitHubLoginUrl } from '@/api'

interface OAuthProvider {
  name: string
  label: string
  icon: string
  enabled: boolean
}

const emit = defineEmits(['login-success'])
const router = useRouter()
const route = useRoute()

const username = ref('')
const password = ref('')
const loading = ref(false)
const oauthLoading = ref(false)
const error = ref('')
const oauthProviders = ref<OAuthProvider[]>([])
const disablePasswordLogin = ref(false)

// 获取 OAuth 提供商列表
const fetchOAuthProviders = async () => {
  try {
    const response = await getOAuthProviders()
    oauthProviders.value = response.data.data || []
    disablePasswordLogin.value = response.data.disable_password_login || false
  } catch (e) {
    // OAuth 未配置时忽略错误
    console.log('OAuth providers not available')
  }
}

// 处理 OAuth 登录错误（从 URL 参数）
const handleOAuthError = () => {
  const errorParam = route.query.error as string
  if (errorParam) {
    const errorMessages: Record<string, string> = {
      'oauth_disabled': 'OAuth 登录未启用',
      'invalid_state': '登录请求已过期，请重试',
      'token_exchange_failed': '获取令牌失败，请重试',
      'user_fetch_failed': '获取用户信息失败',
      'user_not_allowed': '您的账户未被授权登录',
      'user_create_failed': '创建用户失败',
      'token_generate_failed': '生成令牌失败'
    }
    error.value = errorMessages[errorParam] || '登录失败：' + errorParam
    // 清除 URL 参数
    router.replace({ query: {} })
  }
}

onMounted(() => {
  // 清除过期 token
  const token = localStorage.getItem('token')
  if (token) {
    try {
      const payload = JSON.parse(atob(token.split('.')[1]))
      if (payload.exp && Date.now() >= payload.exp * 1000) {
        localStorage.removeItem('token')
      }
    } catch {}
  }
  fetchOAuthProviders()
  handleOAuthError()
})

const handleLogin = async () => {
  if (!username.value || !password.value) {
    error.value = '请输入用户名和密码'
    return
  }
  
  loading.value = true
  error.value = ''
  
  try {
    const response = await login(username.value, password.value)
    localStorage.setItem('token', response.data.data.token)
    emit('login-success')
    router.push('/users')
  } catch (e: any) {
    error.value = e.response?.data?.error || '登录失败，请检查用户名和密码'
  } finally {
    loading.value = false
  }
}

const handleOAuthLogin = async (provider: string) => {
  oauthLoading.value = true
  error.value = ''
  
  try {
    if (provider === 'github') {
      const response = await getGitHubLoginUrl()
      // 兼容两种响应格式: { data: { url: "..." } } 或 { data: "..." }
      const url = typeof response.data.data === 'string' 
        ? response.data.data 
        : response.data.data?.url
      if (response.data.success && url) {
        window.location.href = url
      }
    }
  } catch (e: any) {
    error.value = e.response?.data?.error || 'OAuth 登录失败'
    oauthLoading.value = false
  }
}
</script>

<style scoped>
.login-wrapper {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(-45deg, #667eea, #764ba2, #6B8DD6, #8E37D7);
  background-size: 400% 400%;
  animation: gradientBG 15s ease infinite;
}

@keyframes gradientBG {
  0% {
    background-position: 0% 50%;
  }
  50% {
    background-position: 100% 50%;
  }
  100% {
    background-position: 0% 50%;
  }
}

.login-container {
  width: 100%;
  max-width: 420px;
  padding: 20px;
}

.login-card {
  border: none;
  border-radius: 15px;
  backdrop-filter: blur(10px);
  background: rgba(255, 255, 255, 0.95);
}

.login-card .card-body h2 {
  color: #667eea;
}

.form-control:focus {
  border-color: #667eea;
  box-shadow: 0 0 0 0.2rem rgba(102, 126, 234, 0.25);
}

.btn-primary {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border: none;
  padding: 12px;
  font-weight: 500;
  transition: transform 0.2s, box-shadow 0.2s;
}

.btn-primary:hover {
  transform: translateY(-2px);
  box-shadow: 0 5px 20px rgba(102, 126, 234, 0.4);
}

.btn-primary:disabled {
  transform: none;
  box-shadow: none;
}

.input-group-text {
  background: #f8f9fa;
  border-right: none;
}

.input-group .form-control {
  border-left: none;
}

.input-group:focus-within .input-group-text {
  border-color: #667eea;
}

/* OAuth 分隔线 */
.divider {
  display: flex;
  align-items: center;
  text-align: center;
}

.divider::before,
.divider::after {
  content: '';
  flex: 1;
  border-bottom: 1px solid #dee2e6;
}

.divider-text {
  padding: 0 1rem;
  color: #6c757d;
  font-size: 0.875rem;
}

/* OAuth 按钮 */
.oauth-buttons .btn {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 10px;
  transition: all 0.2s;
}

.oauth-buttons .btn:hover {
  background-color: #f8f9fa;
  border-color: #667eea;
  color: #667eea;
}

.oauth-buttons .btn i {
  font-size: 1.1rem;
}
</style>
