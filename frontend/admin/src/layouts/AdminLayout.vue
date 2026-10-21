<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, RouterView } from 'vue-router'
import { LogOut, ChevronsLeft, ChevronsRight, Home, Sun, Moon, Menu, RefreshCw } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Sheet, SheetContent, SheetTitle } from '@/components/ui/sheet'
import SystemUpdateDialog from '@/components/SystemUpdateDialog.vue'
import { useAdminAuthStore } from '@/stores/auth'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { applySiteIcon } from '@/utils/favicon'
import { adminUrl } from '@/utils/adminBase'

import AdminNavigation from '@/components/admin/AdminNavigation.vue'
import AdminSectionNav from '@/components/admin/AdminSectionNav.vue'

const SIDEBAR_COLLAPSED_STORAGE_KEY = 'admin_sidebar_collapsed'
const SIDEBAR_AUTO_COLLAPSE_WIDTH = 1280

const { t, locale } = useI18n()
const route = useRoute()
const authStore = useAdminAuthStore()
const isDark = ref(false)
const appVersion = ref('')
const siteUrl = ref('')

// 版本检测与一键升级的完整交互都在 SystemUpdateDialog 内，这里只负责开关
const updateCheckOpen = ref(false)

const mobileNavOpen = ref(false)
const sidebarCollapsed = ref(false)
const sidebarUserToggled = ref(false)

watch(() => route.fullPath, () => {
  mobileNavOpen.value = false
})

const applyTheme = (theme: 'light' | 'dark') => {
  isDark.value = theme === 'dark'
  const root = document.documentElement
  if (theme === 'dark') {
    root.classList.add('dark')
  } else {
    root.classList.remove('dark')
  }
  localStorage.setItem('admin_theme', theme)
}

const toggleTheme = () => {
  applyTheme(isDark.value ? 'light' : 'dark')
}

const applyLocale = (value: string) => {
  locale.value = value
  localStorage.setItem('admin_locale', value)
}

const toggleSidebar = () => {
  sidebarCollapsed.value = !sidebarCollapsed.value
  sidebarUserToggled.value = true
  localStorage.setItem(SIDEBAR_COLLAPSED_STORAGE_KEY, String(sidebarCollapsed.value))
}

const handleResize = () => {
  if (sidebarUserToggled.value) return
  sidebarCollapsed.value = window.innerWidth < SIDEBAR_AUTO_COLLAPSE_WIDTH
}

const handleLogout = () => {
  authStore.logout()
  // 必须走 adminUrl()：后台可被挂到运行时任意 web.admin_path 前缀下，
  // 硬编码 '/login' 会跳出后台落到用户前台 SPA 的兜底路由上。
  window.location.href = adminUrl('/login')
}

onMounted(() => {
  const savedTheme = localStorage.getItem('admin_theme')
  if (savedTheme === 'light' || savedTheme === 'dark') {
    applyTheme(savedTheme)
  } else {
    const prefersDark = window.matchMedia?.('(prefers-color-scheme: dark)').matches
    applyTheme(prefersDark ? 'dark' : 'light')
  }

  const savedLocale = localStorage.getItem('admin_locale')
  if (savedLocale) {
    applyLocale(savedLocale)
  }

  // Initialize sidebar collapse state
  const savedCollapsed = localStorage.getItem(SIDEBAR_COLLAPSED_STORAGE_KEY)
  if (savedCollapsed !== null) {
    sidebarCollapsed.value = savedCollapsed === 'true'
    sidebarUserToggled.value = true
  } else {
    // Auto-collapse on medium screens
    sidebarCollapsed.value = window.innerWidth < SIDEBAR_AUTO_COLLAPSE_WIDTH
  }
  window.addEventListener('resize', handleResize)

  // Fetch app version and site URL
  adminAPI.getPublicConfig().then((res) => {
    const payload = res.data?.data
    applySiteIcon(payload?.brand?.site_icon)
    const ver = payload?.app_version
    if (typeof ver === 'string') {
      appVersion.value = ver
    }
    const url = payload?.brand?.site_url
    if (typeof url === 'string' && url) {
      siteUrl.value = url
    }
  }).catch(() => {})
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize)
})
</script>

<template>
  <div class="min-h-screen bg-background text-foreground">
    <div class="flex min-h-screen">
      <!-- Desktop sidebar -->
      <aside
        class="hidden md:flex shrink-0 border-r border-border bg-card flex-col sticky top-0 h-screen overflow-y-auto transition-all duration-300"
        :class="sidebarCollapsed ? 'w-16' : 'w-64'"
      >
        <div class="py-6" :class="sidebarCollapsed ? 'px-2' : 'px-6'">
          <div v-if="!sidebarCollapsed" class="text-xl font-semibold tracking-tight">
            {{ t('admin.brand') }}
          </div>
          <div v-if="!sidebarCollapsed" class="text-xs text-muted-foreground mt-1">{{ t('admin.layout.controlRoom') }}</div>
          <div v-if="sidebarCollapsed" class="text-lg font-semibold tracking-tight text-center">{{ t('admin.brandShort') }}</div>
        </div>
        <AdminNavigation :collapsed="sidebarCollapsed" />
        <!-- Collapse toggle button -->
        <div class="border-t border-border">
          <div v-if="!sidebarCollapsed" class="px-6 py-3 text-[11px] text-muted-foreground space-y-1">
            <p>© {{ new Date().getFullYear() }} {{ t('admin.productName') }} <span v-if="appVersion" class="text-muted-foreground/70">{{ appVersion }}</span></p>
            <a
              href="https://github.com/zxgad3wdasaq1e-ai/faka-58"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex items-center gap-1 underline-offset-2 hover:underline"
            >
              <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                <path d="M12 .5C5.648.5.5 5.648.5 12c0 5.084 3.292 9.4 7.86 10.922.575.106.784-.25.784-.556 0-.273-.01-1-.016-1.962-3.197.694-3.872-1.54-3.872-1.54-.522-1.326-1.274-1.678-1.274-1.678-1.042-.713.079-.699.079-.699 1.152.081 1.758 1.183 1.758 1.183 1.024 1.755 2.688 1.248 3.343.954.104-.742.401-1.248.73-1.535-2.552-.29-5.236-1.276-5.236-5.678 0-1.254.448-2.28 1.182-3.084-.118-.29-.512-1.457.112-3.04 0 0 .964-.308 3.158 1.178a10.98 10.98 0 0 1 2.876-.387c.976.004 1.96.132 2.878.387 2.192-1.486 3.154-1.178 3.154-1.178.626 1.583.232 2.75.114 3.04.736.804 1.18 1.83 1.18 3.084 0 4.413-2.688 5.384-5.248 5.668.412.354.78 1.052.78 2.12 0 1.53-.014 2.764-.014 3.14 0 .31.206.668.79.554C20.212 21.396 23.5 17.083 23.5 12 23.5 5.648 18.352.5 12 .5Z" />
              </svg>
              <span>{{ t('admin.layout.sourceCode') }}</span>
            </a>
          </div>
          <button
            type="button"
            class="flex w-full items-center justify-center py-3 text-muted-foreground hover:text-foreground hover:bg-secondary/70 transition-colors"
            @click="toggleSidebar"
            :title="sidebarCollapsed ? t('admin.layout.expandSidebar') : t('admin.layout.collapseSidebar')"
          >
            <ChevronsLeft v-if="!sidebarCollapsed" class="h-4 w-4" />
            <ChevronsRight v-else class="h-4 w-4" />
          </button>
        </div>
      </aside>

      <!-- Mobile sidebar (Sheet) -->
      <Sheet v-model:open="mobileNavOpen">
        <SheetContent side="left" class="w-72 p-0 flex flex-col">
          <SheetTitle class="sr-only">{{ t('admin.layout.navigation') }}</SheetTitle>
          <div class="px-6 py-6">
            <div class="text-xl font-semibold tracking-tight">
              {{ t('admin.brand') }}
            </div>
            <div class="text-xs text-muted-foreground mt-1">{{ t('admin.layout.controlRoom') }}</div>
          </div>
          <AdminNavigation />
          <div class="px-6 py-4 border-t border-border text-[11px] text-muted-foreground space-y-1">
            <p>© {{ new Date().getFullYear() }} {{ t('admin.productName') }} <span v-if="appVersion" class="text-muted-foreground/70">{{ appVersion }}</span></p>
          </div>
        </SheetContent>
      </Sheet>

      <div class="flex-1 flex flex-col min-w-0">
        <header class="flex items-center justify-between border-b border-border bg-background px-4 md:px-8 py-3 md:py-4 gap-2">
          <div class="flex items-center gap-3">
            <Button size="icon-sm" variant="ghost" class="md:hidden" :aria-label="t('admin.layout.navigation')" @click="mobileNavOpen = true">
              <Menu class="h-5 w-5" />
            </Button>
            <Button
              size="icon-sm"
              variant="ghost"
              class="hidden md:inline-flex"
              @click="toggleSidebar"
              :title="sidebarCollapsed ? t('admin.layout.expandSidebar') : t('admin.layout.collapseSidebar')"
            >
              <ChevronsRight v-if="sidebarCollapsed" class="h-4 w-4" />
              <ChevronsLeft v-else class="h-4 w-4" />
            </Button>
            <div class="text-sm text-muted-foreground hidden sm:block">{{ t('admin.layout.workspace') }}</div>
          </div>
          <div class="flex items-center gap-2">
            <Select
              :model-value="locale"
              @update:modelValue="(value) => { if (value) applyLocale(String(value)) }"
            >
              <SelectTrigger class="h-8 w-[100px] md:w-[140px] text-xs">
                <SelectValue :placeholder="t('admin.common.lang.zhCN')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="zh-CN">{{ t('admin.common.lang.zhCN') }}</SelectItem>
                <SelectItem value="zh-TW">{{ t('admin.common.lang.zhTW') }}</SelectItem>
                <SelectItem value="en-US">{{ t('admin.common.lang.enUS') }}</SelectItem>
              </SelectContent>
            </Select>
            <Button
              size="icon-sm"
              variant="outline"
              :title="t('admin.updateCheck.button')"
              @click="updateCheckOpen = true"
            >
              <RefreshCw class="h-4 w-4" />
            </Button>
            <Button size="icon-sm" variant="outline" @click="toggleTheme">
              <Sun v-if="isDark" class="h-4 w-4" />
              <Moon v-else class="h-4 w-4" />
            </Button>
            <a v-if="siteUrl" :href="siteUrl" target="_blank" rel="noopener noreferrer">
              <Button size="icon-sm" variant="outline" :title="t('admin.layout.homePage')">
                <Home class="h-4 w-4" />
              </Button>
            </a>
            <Button variant="outline" size="sm" class="gap-2" @click="handleLogout">
              <LogOut class="h-4 w-4" />
              <span class="hidden sm:inline">{{ t('admin.common.logout') }}</span>
            </Button>
          </div>
        </header>
        <main class="min-w-0 flex-1 overflow-x-hidden px-4 py-4 md:px-8 md:py-6">
          <div class="min-w-0">
            <AdminSectionNav />
            <RouterView />
          </div>
        </main>
      </div>
    </div>

    <SystemUpdateDialog v-model:open="updateCheckOpen" />
  </div>
</template>
