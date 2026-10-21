import type { CartItem } from '../stores/cart'
import { amountToCents, centsToAmount } from './money.ts'

// 同步目录基础价；统一定价时清除旧购物车的批发阶梯，最终金额以服务端预览为准。
export const resolveCartPricingSnapshot = (product: any, sku: any, uniformPricing = true): Partial<CartItem> => {
  const patch: Partial<CartItem> = {
    wholesalePrices: !uniformPricing && Array.isArray(product?.wholesale_prices)
      ? product.wholesale_prices
      : undefined,
  }

  const priceCents = amountToCents(sku?.price_amount)
  if (priceCents !== null && priceCents > 0) {
    patch.priceAmount = centsToAmount(priceCents)
  }

  return patch
}
