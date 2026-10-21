export type StorefrontTemplate = 'classic' | 'vault' | 'faka-58'

export const STOREFRONT_TEMPLATES: StorefrontTemplate[] = ['classic', 'vault', 'faka-58']
export const DEFAULT_TEMPLATE: StorefrontTemplate = 'faka-58'

export const normalizeStorefrontTemplate = (value: unknown): StorefrontTemplate | null => {
  const template = typeof value === 'string' ? value.trim() : ''
  // 已保存的旧模板配置、预览链接继续使用改名后的模板。
  if (template === 'getgpt') return 'faka-58'
  return (STOREFRONT_TEMPLATES as string[]).includes(template) ? template as StorefrontTemplate : null
}
