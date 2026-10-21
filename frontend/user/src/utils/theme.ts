import { ref, watch } from 'vue'

const THEME_KEY = 'faka-58_theme'
const LEGACY_THEME_KEY = 'dujiao_theme'

type Theme = 'light' | 'dark'

const getSystemTheme = (): Theme => {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

const getSavedTheme = (): Theme | null => {
    try {
        const saved = localStorage.getItem(THEME_KEY) ?? localStorage.getItem(LEGACY_THEME_KEY)
        return saved === 'light' || saved === 'dark' ? saved : null
    } catch {
        return null
    }
}

const theme = ref<Theme>(getSavedTheme() || getSystemTheme())

const applyTheme = (newTheme: Theme) => {
    const root = document.documentElement
    if (newTheme === 'dark') {
        root.classList.add('dark')
    } else {
        root.classList.remove('dark')
    }
    try {
        localStorage.setItem(THEME_KEY, newTheme)
        localStorage.removeItem(LEGACY_THEME_KEY)
    } catch {
        /* 存储不可写时仍允许切换当前页面的主题。 */
    }
}

// Watch for changes and apply
watch(theme, (newVal) => {
    applyTheme(newVal)
}, { immediate: true })

export const useTheme = () => {
    const toggleTheme = () => {
        theme.value = theme.value === 'dark' ? 'light' : 'dark'
    }

    return {
        theme,
        toggleTheme
    }
}
