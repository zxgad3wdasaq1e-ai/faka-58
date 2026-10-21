import { useI18n } from 'vue-i18n'

const zhCN = {
  plansTitle: '选择适合您的', plansAccent: 'AI 服务方案',
  plansSubtitle: '轻松选购，便捷支付。在订单中查看卡密与交付进度。',
  recommended: '推荐', choosePlan: '选择此方案', allProducts: '全部方案',
  planDetails: '查看方案详情', service: '数字商品与订阅服务',
  delivery: '交付方式', availability: '当前库存', purchase: '购买方式',
  paymentTitle: '便捷支付', paymentHint: '可用支付方式以结账页面为准',
  orderTitle: '订单随时可查', orderHint: '凭购买邮箱和查询密码查看订单',
  deliveryTitle: '清晰的交付进度', deliveryHint: '付款后在订单详情查看交付结果',
  detailTitle: '确认您的方案', detailHint: '选择规格和数量后，前往结账完成购买。',
  description: '方案说明', quantity: '购买数量',
  decrease: '减少数量', increase: '增加数量',
  checkoutHint: '商品的价格、库存和可用支付方式以结账确认为准。',
  lookupTitle: '查询您的订单', lookupSubtitle: '输入购买时填写的邮箱和订单查询密码，查看支付与交付状态。',
  queryPassword: '订单查询密码', optionalOrder: '订单号（选填）',
  lookupHelp: '订单查询密码由您在结账时设置，请妥善保管。',
  resultTitle: '查询结果', resultsEmpty: '填写上方信息后即可查询订单',
  memberOrders: '已登录用户？前往我的订单', backPlans: '返回选购方案',
} as const

type CommerceKey = keyof typeof zhCN

export const commerceMessages: Record<string, Record<CommerceKey, string>> = {
  'zh-CN': zhCN,
  'zh-TW': {
    plansTitle: '選擇適合您的', plansAccent: 'AI 服務方案',
    plansSubtitle: '輕鬆選購，便捷付款。在訂單中查看卡密與交付進度。',
    recommended: '推薦', choosePlan: '選擇此方案', allProducts: '全部方案',
    planDetails: '查看方案詳情', service: '數位商品與訂閱服務',
    delivery: '交付方式', availability: '目前庫存', purchase: '購買方式',
    paymentTitle: '便捷付款', paymentHint: '可用付款方式以結帳頁面為準',
    orderTitle: '隨時查詢訂單', orderHint: '憑購買信箱和查詢密碼查看訂單',
    deliveryTitle: '清楚的交付進度', deliveryHint: '付款後在訂單詳情查看交付結果',
    detailTitle: '確認您的方案', detailHint: '選擇規格和數量後，前往結帳完成購買。',
    description: '方案說明', quantity: '購買數量',
    decrease: '減少數量', increase: '增加數量',
    checkoutHint: '商品的價格、庫存和可用付款方式以結帳確認為準。',
    lookupTitle: '查詢您的訂單', lookupSubtitle: '輸入購買時填寫的信箱和訂單查詢密碼，查看付款與交付狀態。',
    queryPassword: '訂單查詢密碼', optionalOrder: '訂單編號（選填）',
    lookupHelp: '訂單查詢密碼由您在結帳時設定，請妥善保管。',
    resultTitle: '查詢結果', resultsEmpty: '填寫上方資訊後即可查詢訂單',
    memberOrders: '已登入使用者？前往我的訂單', backPlans: '返回選購方案',
  },
  'en-US': {
    plansTitle: 'Find your', plansAccent: 'AI service plan',
    plansSubtitle: 'Choose a plan, pay with ease, and track your delivery in your order.',
    recommended: 'Recommended', choosePlan: 'Choose this plan', allProducts: 'All plans',
    planDetails: 'View plan details', service: 'Digital products and subscriptions',
    delivery: 'Delivery', availability: 'Availability', purchase: 'Purchase',
    paymentTitle: 'Easy payments', paymentHint: 'Available methods are shown at checkout',
    orderTitle: 'Track your order', orderHint: 'Use your purchase email and lookup password',
    deliveryTitle: 'Clear delivery status', deliveryHint: 'View delivery details after payment',
    detailTitle: 'Confirm your plan', detailHint: 'Choose a variant and quantity, then continue to checkout.',
    description: 'Plan details', quantity: 'Quantity',
    decrease: 'Decrease quantity', increase: 'Increase quantity',
    checkoutHint: 'Final prices, availability, and payment methods are confirmed at checkout.',
    lookupTitle: 'Find your order', lookupSubtitle: 'Enter your purchase email and order lookup password to check payment and delivery.',
    queryPassword: 'Order lookup password', optionalOrder: 'Order number (optional)',
    lookupHelp: 'Use the order lookup password you created at checkout. Keep it safe.',
    resultTitle: 'Your orders', resultsEmpty: 'Enter your details above to find your orders',
    memberOrders: 'Have an account? Go to my orders', backPlans: 'Back to plans',
  },
}

// Read a small template dictionary directly: the production i18n runtime only
// accepts precompiled global locale files and does not compile TS strings.
export function useCommerceCopy() {
  const { locale } = useI18n({ useScope: 'global' })
  return (key: CommerceKey) => (commerceMessages[locale.value] || commerceMessages['zh-CN'])![key]
}
