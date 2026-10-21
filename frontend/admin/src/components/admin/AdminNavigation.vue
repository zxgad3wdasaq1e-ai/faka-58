<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { LayoutDashboard } from 'lucide-vue-next'
import { Input } from '@/components/ui/input'
import { useAdminAuthStore } from '@/stores/auth'
import { accessibleAdminSections, activeAdminPage } from '@/utils/adminNavigation'

const props = defineProps<{ collapsed?: boolean }>()
const route = useRoute()
const { t } = useI18n()
const auth = useAdminAuthStore()
const search = ref('')
const sections = computed(() => accessibleAdminSections(auth.hasPermission))
const active = computed(() => activeAdminPage(sections.value, route.path))
const keyword = computed(() => props.collapsed ? '' : search.value.trim().toLocaleLowerCase())
const dashboardLabel = computed(() => t('admin.sections.overview'))
const showDashboard = computed(() => !keyword.value || dashboardLabel.value.toLocaleLowerCase().includes(keyword.value))
const results = computed(() => sections.value.map(section => ({
  ...section,
  items: t(section.labelKey).toLocaleLowerCase().includes(keyword.value)
    ? section.items
    : section.items.filter(entry => t(entry.labelKey).toLocaleLowerCase().includes(keyword.value)),
})).filter(section => section.items.length > 0))
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col">
    <div v-if="!collapsed" class="px-3 pb-3">
      <Input v-model="search" class="h-9 text-sm" :aria-label="t('admin.navSearch.placeholder')" :placeholder="t('admin.navSearch.placeholder')" />
    </div>
    <nav :aria-label="t('admin.layout.navigation')" class="flex-1 space-y-1 overflow-y-auto pb-3" :class="collapsed ? 'px-1.5' : 'px-3'">
      <RouterLink
        v-if="showDashboard" to="/" :title="dashboardLabel" :aria-label="dashboardLabel"
        :aria-current="route.path === '/' ? 'page' : undefined"
        class="flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm transition-colors"
        :class="[collapsed ? 'justify-center px-2' : '', route.path === '/' ? 'bg-secondary font-medium text-foreground' : 'text-muted-foreground hover:bg-secondary/70 hover:text-foreground']"
      >
        <LayoutDashboard class="h-4 w-4 shrink-0" />
        <span v-if="!collapsed">{{ dashboardLabel }}</span>
      </RouterLink>
      <template v-for="section in results" :key="section.id">
        <RouterLink
          v-if="!keyword" :to="section.items[0]!.to" :title="t(section.labelKey)" :aria-label="t(section.labelKey)"
          :aria-current="active?.section.id === section.id ? 'location' : undefined"
          class="flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm transition-colors"
          :class="[collapsed ? 'justify-center px-2' : '', active?.section.id === section.id ? 'bg-secondary font-medium text-foreground' : 'text-muted-foreground hover:bg-secondary/70 hover:text-foreground']"
        >
          <component :is="section.icon" class="h-4 w-4 shrink-0" />
          <span v-if="!collapsed">{{ t(section.labelKey) }}</span>
        </RouterLink>
        <div v-else class="space-y-1 pt-2">
          <p class="px-3 py-1 text-xs font-medium text-muted-foreground">{{ t(section.labelKey) }}</p>
          <RouterLink
            v-for="entry in section.items" :key="entry.to" :to="entry.to"
            :aria-current="active?.item.to === entry.to ? 'page' : undefined"
            class="block rounded-lg px-3 py-2 text-sm transition-colors"
            :class="active?.item.to === entry.to ? 'bg-secondary text-foreground' : 'text-muted-foreground hover:bg-secondary/70 hover:text-foreground'"
          >{{ t(entry.labelKey) }}</RouterLink>
        </div>
      </template>
      <p v-if="!showDashboard && !results.length" class="px-3 py-4 text-sm text-muted-foreground">{{ t('admin.navSearch.empty') }}</p>
    </nav>
  </div>
</template>
