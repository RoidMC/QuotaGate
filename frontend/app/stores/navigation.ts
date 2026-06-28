import { defineStore } from 'pinia'

export const useNavigationStore = defineStore('navigation', () => {
  // 从媒体查询获取断点信息
  const { currentBreakpoint } = useMediaQuery()

  // 开发环境调试开关
  const devMode = ref(process.env.NODE_ENV === 'development')
  const forceMobile = ref(false)
  const forceDesktop = ref(false)

  // 导航动画状态 - 由 Navmenu 组件控制
  const isNavAnimating = ref(false)

  // 计算属性：判断是否为移动端视图
  const isMobileOrTablet = computed(() => {
    // 开发环境下优先使用调试开关
    if (devMode) {
      // 如果强制移动端，返回 true
      if (forceMobile.value) return true
      // 如果强制桌面端，返回 false（这个判断必须优先于断点判断）
      if (forceDesktop.value) return false
    }

    // 没有强制开关时，根据断点判断
    return currentBreakpoint.value === 'xs' || currentBreakpoint.value === 'sm' || currentBreakpoint.value === 'md'
  })

  // 直接计算视图状态，确保响应调试开关
  const isMobileView = computed(() => isMobileOrTablet.value)
  const isDesktopView = computed(() => !isMobileOrTablet.value)

  // 调试控制函数
  const toggleForceMobile = () => {
    forceMobile.value = !forceMobile.value
    if (forceMobile.value) forceDesktop.value = false
  }

  const toggleForceDesktop = () => {
    forceDesktop.value = !forceDesktop.value
    if (forceDesktop.value) forceMobile.value = false
  }

  // 重置调试状态
  const resetDebugModes = () => {
    forceMobile.value = false
    forceDesktop.value = false
  }

  // 导航样式类型
  const navStyleCase = computed(() => isMobileView.value)

  // 设置导航动画状态的方法
  const setNavAnimating = (animating: boolean) => {
    isNavAnimating.value = animating
  }

  return {
    // 状态
    isMobileView,
    isDesktopView,
    currentBreakpoint,
    navStyleCase,
    isNavAnimating,

    // 调试相关
    devMode,
    forceMobile,
    forceDesktop,

    // 方法
    toggleForceMobile,
    toggleForceDesktop,
    resetDebugModes,
    setNavAnimating
  }
})