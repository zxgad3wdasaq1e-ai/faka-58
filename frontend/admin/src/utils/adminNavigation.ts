import type { Component } from 'vue'
import {
  BadgePercent, Bot, CreditCard, KeyRound, Package, Settings, ShoppingBag, Truck, Users,
} from 'lucide-vue-next'

export interface AdminNavItem {
  labelKey: string
  to: string
  permission?: string
}

export interface AdminNavSection {
  id: string
  labelKey: string
  icon: Component
  items: AdminNavItem[]
}

const item = (label: string, to: string, permission?: string): AdminNavItem => ({
  labelKey: `admin.navItems.${label}`, to, permission,
})

// Keep existing URLs and permissions: sections only organize the management pages.
export const adminNavSections: AdminNavSection[] = [
  {
    id: 'products', labelKey: 'admin.sections.products', icon: Package,
    items: [
      item('productList', '/products', 'GET:/admin/products'),
      item('productCategories', '/categories', 'GET:/admin/categories'),
    ],
  },
  {
    id: 'inventory', labelKey: 'admin.sections.inventory', icon: KeyRound,
    items: [
      item('cardSecrets', '/card-secrets', 'GET:/admin/card-secrets'),
      item('cardSecretImports', '/card-secret-imports', 'GET:/admin/card-secrets'),
      item('cardSecretExports', '/card-secret-exports', 'GET:/admin/card-secrets'),
    ],
  },
  {
    id: 'orders', labelKey: 'admin.sections.orders', icon: ShoppingBag,
    items: [
      item('orderList', '/orders', 'GET:/admin/orders'),
      item('orderRefunds', '/order-refunds', 'GET:/admin/order-refunds'),
    ],
  },
  {
    id: 'payments', labelKey: 'admin.sections.payments', icon: CreditCard,
    items: [
      item('payments', '/payments', 'GET:/admin/payments'),
      item('paymentChannels', '/payment-channels', 'GET:/admin/payment-channels'),
      item('walletManagement', '/wallet-recharges', 'GET:/admin/wallet/recharges'),
      item('walletConfig', '/wallet-config', 'GET:/admin/settings'),
      item('callbackRoutes', '/callback-routes', 'GET:/admin/settings'),
    ],
  },
  {
    id: 'customers', labelKey: 'admin.sections.customers', icon: Users,
    items: [
      item('userList', '/users', 'GET:/admin/users'),
      item('userLoginLogs', '/user-login-logs', 'GET:/admin/user-login-logs'),
    ],
  },
  {
    id: 'supply', labelKey: 'admin.sections.supply', icon: Truck,
    items: [
      item('siteConnections', '/site-connections', 'GET:/admin/site-connections'),
      item('productMappings', '/product-mappings', 'GET:/admin/product-mappings'),
      item('procurementOrders', '/procurement-orders', 'GET:/admin/procurement-orders'),
      item('reconciliation', '/reconciliation', 'GET:/admin/reconciliation/jobs'),
      item('apiCredentials', '/api-credentials', 'GET:/admin/api-credentials'),
    ],
  },
  {
    id: 'affiliate', labelKey: 'admin.sections.affiliate', icon: BadgePercent,
    items: [
      item('affiliatesUsers', '/affiliates/users', 'GET:/admin/affiliates/users'),
      item('affiliatesCommissions', '/affiliates/commissions', 'GET:/admin/affiliates/commissions'),
      item('affiliatesWithdraws', '/affiliates/withdraws', 'GET:/admin/affiliates/withdraws'),
      item('affiliatesSettings', '/affiliates/settings', 'GET:/admin/settings/affiliate'),
    ],
  },
  {
    id: 'telegram', labelKey: 'admin.sections.telegram', icon: Bot,
    items: [
      item('telegramBotOverview', '/telegram-bot', 'GET:/admin/settings/telegram-bot'),
      item('telegramBotSettings', '/telegram-bot/settings', 'GET:/admin/settings/telegram-bot'),
      item('telegramBotMenuSettings', '/telegram-bot/menu', 'GET:/admin/settings/telegram-bot'),
      item('telegramBotHelpCenter', '/telegram-bot/help-center', 'GET:/admin/settings/telegram-bot'),
      item('telegramBotStatus', '/telegram-bot/status', 'GET:/admin/settings/telegram-bot'),
      item('telegramBotChannelClients', '/telegram-bot/channel-clients', 'GET:/admin/channel-clients'),
      item('telegramBotBroadcasts', '/telegram-bot/broadcasts', 'GET:/admin/telegram-bot/broadcasts'),
    ],
  },
  {
    id: 'settings', labelKey: 'admin.sections.settings', icon: Settings,
    items: [
      item('siteSettings', '/settings', 'GET:/admin/settings'),
      item('announcementList', '/posts/notice', 'GET:/admin/posts'),
      item('banners', '/banners', 'GET:/admin/banners'),
      item('media', '/media', 'GET:/admin/media'),
      item('notificationCenter', '/settings/notifications', 'GET:/admin/settings/notification-center'),
      item('orderRiskControl', '/order-risk-control', 'GET:/admin/settings'),
      item('security', '/security'),
      item('authz', '/authz', 'GET:/admin/authz/roles'),
      item('authzAudit', '/authz-audit-logs', 'GET:/admin/authz/audit-logs'),
    ],
  },
]

export function accessibleAdminSections(hasPermission: (permission?: string) => boolean) {
  return adminNavSections
    .map(section => ({ ...section, items: section.items.filter(entry => hasPermission(entry.permission)) }))
    .filter(section => section.items.length > 0)
}

export function activeAdminPage(sections: AdminNavSection[], path: string) {
  // Longest match selects broadcasts for /telegram-bot/broadcasts/:id,
  // rather than selecting both broadcasts and the bot overview.
  return sections
    .flatMap(section => section.items.map(entry => ({ section, item: entry })))
    .filter(({ item: entry }) => path === entry.to || path.startsWith(`${entry.to}/`))
    .sort((a, b) => b.item.to.length - a.item.to.length)[0]
}
