/**
 * Breakpoint 检测 Composable
 * 提供现代化的响应式断点检测功能
 */

// 标准断点配置 (Tailwind CSS 风格)
export const BREAKPOINTS = {
  xs: 0,      // 超小屏
  sm: 640,    // 小屏 (mobile)
  md: 768,    // 中屏 (tablet)
  lg: 1024,   // 大屏 (desktop)
  xl: 1280,   // 超大屏
  '2xl': 1536 // 2倍超大屏
} as const

export type BreakpointKey = keyof typeof BREAKPOINTS

/**
 * 基础媒体查询 Composable
 * @returns 屏幕宽度和断点检测功能
 */
export function useMediaQuery() {
  // 响应式屏幕尺寸
  const width = ref(typeof window !== 'undefined' ? window.innerWidth : BREAKPOINTS.lg)
  const height = ref(typeof window !== 'undefined' ? window.innerHeight : 900)
  
  // 当前激活的断点
  const currentBreakpoint = computed<BreakpointKey>(() => {
    const w = width.value
    if (w >= BREAKPOINTS['2xl']) return '2xl'
    if (w >= BREAKPOINTS.xl) return 'xl'
    if (w >= BREAKPOINTS.lg) return 'lg'
    if (w >= BREAKPOINTS.md) return 'md'
    if (w >= BREAKPOINTS.sm) return 'sm'
    return 'xs'
  })

  /**
   * 检查是否达到指定断点
   * @param breakpoint 断点键名
   * @returns 是否达到该断点
   */
  const atBreakpoint = (breakpoint: BreakpointKey) => {
    return computed(() => width.value >= BREAKPOINTS[breakpoint])
  }

  /**
   * 检查是否在两个断点之间
   * @param min 最小断点
   * @param max 最大断点
   * @returns 是否在范围内
   */
  const betweenBreakpoints = (min: BreakpointKey, max: BreakpointKey) => {
    return computed(() => {
      const w = width.value
      return w >= BREAKPOINTS[min] && w < BREAKPOINTS[max]
    })
  }

  /**
   * 检查是否匹配 CSS 媒体查询
   * @param query CSS 媒体查询字符串
   * @returns 是否匹配
   */
  const matches = (query: string): boolean => {
    if (typeof window === 'undefined') return false
    return window.matchMedia(query).matches
  }

  /**
   * 更新屏幕尺寸
   */
  const updateDimensions = () => {
    if (typeof window === 'undefined') return
    width.value = window.innerWidth
    height.value = window.innerHeight
  }

  // SSR 支持 - 使用现代的 import.meta.server
  if (import.meta.server) {
    // 服务端默认值
    width.value = BREAKPOINTS.lg
    height.value = 900
  }

  // 客户端初始化
  if (import.meta.client) {
    // 立即更新尺寸，确保首次渲染正确
    updateDimensions()

    onMounted(() => {
      // 防抖的 resize 监听器
      let resizeTimer: NodeJS.Timeout
      const handleResize = () => {
        clearTimeout(resizeTimer)
        resizeTimer = setTimeout(updateDimensions, 150)
      }

      window.addEventListener('resize', handleResize, { passive: true })

      onUnmounted(() => {
        window.removeEventListener('resize', handleResize)
        clearTimeout(resizeTimer)
      })
    })
  }

  return {
    // 基础尺寸
    width: readonly(width),
    height: readonly(height),
    
    // 断点信息
    currentBreakpoint,
    
    // 检测方法
    atBreakpoint,
    betweenBreakpoints,
    matches,
    updateDimensions
  }
}

// 便捷设备检测函数

/**
 * 检查是否为移动设备 (小于 md 断点)
 */
export function useIsMobile() {
  const mq = useMediaQuery()
  return computed(() => mq.width.value < BREAKPOINTS.md)
}

/**
 * 检查是否为平板设备 (md 到 lg 之间)
 */
export function useIsTablet() {
  const mq = useMediaQuery()
  return computed(() => {
    const w = mq.width.value
    return w >= BREAKPOINTS.md && w < BREAKPOINTS.lg
  })
}

/**
 * 检查是否为桌面设备 (大于等于 lg 断点)
 */
export function useIsDesktop() {
  const mq = useMediaQuery()
  return computed(() => mq.width.value >= BREAKPOINTS.lg)
}

/**
 * 检查是否为大屏桌面设备 (大于等于 xl 断点)
 */
export function useIsLargeDesktop() {
  const mq = useMediaQuery()
  return computed(() => mq.width.value >= BREAKPOINTS.xl)
}

/**
 * 获取设备类型字符串
 */
export function useDeviceType() {
  const isMobile = useIsMobile()
  const isTablet = useIsTablet()
  const isDesktop = useIsDesktop()
  
  return computed(() => {
    if (isMobile.value) return 'mobile'
    if (isTablet.value) return 'tablet'
    if (isDesktop.value) return 'desktop'
    return 'unknown'
  })
}

/**
 * 使用自定义数值断点
 * @param breakpoint 数值断点
 * @returns 是否达到该断点
 */
export function useCustomBreakpoint(breakpoint: number) {
  const mq = useMediaQuery()
  return computed(() => mq.width.value >= breakpoint)
}

/**
 * 批量检查多个断点
 * @param breakpoints 断点数组
 * @returns 每个断点的激活状态
 */
export function useBreakpoints(breakpoints: Record<string, number>) {
  const mq = useMediaQuery()
  const result: Record<string, ComputedRef<boolean>> = {}
  
  for (const [key, value] of Object.entries(breakpoints)) {
    result[key] = computed(() => mq.width.value >= value)
  }
  
  return result
}