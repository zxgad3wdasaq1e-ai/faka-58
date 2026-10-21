<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAdminAuthStore } from '@/stores/auth'
import { accessibleAdminSections, activeAdminPage } from '@/utils/adminNavigation'

const route = useRoute()
const { t } = useI18n()
const auth = useAdminAuthStore()
const active = computed(() => activeAdminPage(accessibleAdminSections(auth.hasPermission), route.path))
</script>

<template>
  <nav v-if="active" :aria-label="t(active.section.labelKey)" class="mb-6 flex flex-wrap gap-1 rounded-xl border border-border bg-card p-2">
    <RouterLink
      v-for="entry in active.section.items" :key="entry.to" :to="entry.to"
      :aria-current="active.item.to === entry.to ? 'page' : undefined"
      class="rounded-lg px-3 py-2 text-sm transition-colors"
      :class="active.item.to === entry.to ? 'bg-secondary font-medium text-foreground' : 'text-muted-foreground hover:bg-secondary/70 hover:text-foreground'"
    >{{ t(entry.labelKey) }}</RouterLink>
  </nav>
</template>
