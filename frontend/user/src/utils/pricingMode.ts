export interface PricingConfig {
  uniform_pricing?: unknown
}

// 只有显式关闭统一定价时，才恢复旧版营销价格。
export const isUniformPricing = (config?: PricingConfig | null) => config?.uniform_pricing !== false
