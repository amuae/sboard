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
      <div class="modal-dialog">
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
import { logout as apiLogout, changeUsername, changePassword } from './api'

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
const accountTab = ref<'username' | 'password'>('username')
const newUsername = ref('')
const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
let accountModal: Modal | null = null

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
    } else {
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

onMounted(() => {
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
