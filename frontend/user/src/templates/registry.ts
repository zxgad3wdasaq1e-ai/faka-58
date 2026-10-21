import { useAppStore } from '../stores/app'
import { DEFAULT_TEMPLATE, normalizeStorefrontTemplate, type StorefrontTemplate } from '../../../shared/storefrontTemplate'

export { DEFAULT_TEMPLATE, STOREFRONT_TEMPLATES, type StorefrontTemplate } from '../../../shared/storefrontTemplate'

/**
 * 店面模板系统（站长全局切换 · 渐进并行迁移）
 *
 * - 当前激活模板优先级：本地预览覆盖(?template=) > 站点全局配置(storefront_template) > 默认 faka-58
 * - classic 沿用现有 ../views/*，vault 落在 ./vault/*；vault 缺页时自动回退 classic，
 *   因此可以一页一页地把新设计搬进 vault，旧站全程可用。
 */

const OVERRIDE_KEY = 'faka-58-storefront-template'
const LEGACY_OVERRIDE_KEY = 'dj-storefront-template'

const readOverride = (): StorefrontTemplate | null => {
    try {
        const saved = localStorage.getItem(OVERRIDE_KEY)
        const legacy = localStorage.getItem(LEGACY_OVERRIDE_KEY)
        const value = normalizeStorefrontTemplate(saved) ?? normalizeStorefrontTemplate(legacy)
        if (value && (saved !== value || legacy !== null)) {
            try {
                localStorage.setItem(OVERRIDE_KEY, value)
                localStorage.removeItem(LEGACY_OVERRIDE_KEY)
            } catch {
                /* 存储只读时仍沿用已保存的预览配置。 */
            }
        }
        return value
    } catch {
        return null
    }
}

/**
 * 预览用：URL 带 ?template=vault / ?template=classic 时持久化到 localStorage，
 * ?template=reset 清除覆盖。站长正式切换走站点配置，不依赖此入口。
 * 在 app 挂载前调用一次即可。
 */
export const initTemplateOverride = (): void => {
    if (typeof window === 'undefined') return
    try {
        const param = new URLSearchParams(window.location.search).get('template')
        if (param === 'reset') {
            localStorage.removeItem(OVERRIDE_KEY)
            localStorage.removeItem(LEGACY_OVERRIDE_KEY)
        } else {
            const template = normalizeStorefrontTemplate(param)
            if (template) {
                localStorage.setItem(OVERRIDE_KEY, template)
                localStorage.removeItem(LEGACY_OVERRIDE_KEY)
            }
        }
    } catch {
        /* localStorage 不可用时忽略 */
    }
}

/** 当前激活的店面模板。 */
export const getActiveTemplate = (): StorefrontTemplate => {
    const override = readOverride()
    if (override) return override
    try {
        const appStore = useAppStore()
        const fromConfig = normalizeStorefrontTemplate(appStore.config?.storefront_template)
        if (fromConfig) return fromConfig
    } catch {
        /* pinia 尚未就绪时退回默认 */
    }
    return DEFAULT_TEMPLATE
}

// vault 模板页面（按需动态加载）。key 形如 './vault/Home.vue'
const vaultViews = import.meta.glob('./vault/**/*.vue')
const faka58Views = import.meta.glob('./faka-58/**/*.vue')

type ViewLoader = () => Promise<unknown>

/**
 * 路由 view 解析器：vault 模板下若存在同名页面则用 vault 版，否则回退传入的 classic loader。
 * 用法：`component: templateView('Home', () => import('../views/Home.vue'))`
 */
export const templateView = (name: string, classicLoader: ViewLoader): ViewLoader => {
    return () => {
        if (getActiveTemplate() === 'faka-58') {
            const loader = faka58Views[`./faka-58/${name}.vue`]
            if (loader) return loader()
        }
        if (getActiveTemplate() === 'vault') {
            const loader = vaultViews[`./vault/${name}.vue`]
            if (loader) return loader()
        }
        return classicLoader()
    }
}
