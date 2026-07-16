/**
 * 主题切换 composable
 * 提供主题切换逻辑和图标计算，供多个组件复用
 */
export const useThemeToggle = () => {
    const colorMode = useColorMode()

    const themeIcon = computed(() => {
        const preference = colorMode.preference || 'system'
        switch (preference) {
            case 'dark':
                return 'tdesign:mode-dark'
            case 'light':
                return 'tdesign:mode-light'
            default:
                return 'tdesign:brightness-1'
        }
    })

    const themeLabel = computed(() => {
        const preference = colorMode.preference || 'system'
        switch (preference) {
            case 'dark':
                return 'i18n-common-string.uni.theme-dark'
            case 'light':
                return 'i18n-common-string.uni.theme-light'
            default:
                return 'i18n-common-string.uni.theme-system'
        }
    })

    const toggleTheme = () => {
        const values = ['system', 'light', 'dark'] as const
        const currentPreference = colorMode.preference || 'system'
        const index = values.indexOf(currentPreference as 'system' | 'light' | 'dark')
        const next = (index + 1) % values.length

        // 添加背景切换动画效果
        if (import.meta.client) {
            const backgroundElement = document.querySelector('.rmc-background')
            if (backgroundElement) {
                backgroundElement.classList.add('theme-transitioning')
                setTimeout(() => {
                    backgroundElement.classList.remove('theme-transitioning')
                }, 800)
            }
        }

        colorMode.preference = values[next] as 'system' | 'light' | 'dark'
    }

    return {
        colorMode,
        themeIcon,
        themeLabel,
        toggleTheme
    }
}
