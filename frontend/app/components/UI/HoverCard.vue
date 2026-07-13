<script setup lang="ts">
/**
 * 无样式 HoverCard 组件
 * 桌面端 hover 触发，移动端自动切换为点击切换
 * 使用 CSS transition 实现动画
 * trigger 使用 display: contents 不影响布局
 * 支持溢出检测：自动翻转 + 水平修正
 */

interface Props {
  /** 显示延迟 (ms) */
  openDelay?: number
  /** 隐藏延迟 (ms) */
  closeDelay?: number
  /** 弹出方向 */
  side?: 'top' | 'bottom' | 'left' | 'right'
  /** 与触发元素的间距 (px) */
  sideOffset?: number
  /** 对齐方式 */
  align?: 'start' | 'center' | 'end'
  /** 触发方式：hover 仅桌面悬停、click 点击切换、auto 根据设备自动选择 */
  trigger?: 'hover' | 'click' | 'auto'
}

const props = withDefaults(defineProps<Props>(), {
  openDelay: 100,
  closeDelay: 50,
  side: 'top',
  sideOffset: 8,
  align: 'center',
  trigger: 'auto'
})

const isOpen = ref(false)
const isVisible = ref(false)
const triggerRef = ref<HTMLElement | null>(null)
const contentRef = ref<HTMLElement | null>(null)
const contentStyle = ref<Record<string, string>>({})
const resolvedSide = ref(props.side)
const isHoverMode = ref(true)

let openTimer: ReturnType<typeof setTimeout> | null = null
let closeTimer: ReturnType<typeof setTimeout> | null = null
let hoverModeMediaQuery: MediaQueryList | null = null

const VIEWPORT_PADDING = 8

const clearTimers = () => {
  if (openTimer) {
    clearTimeout(openTimer)
    openTimer = null
  }
  if (closeTimer) {
    clearTimeout(closeTimer)
    closeTimer = null
  }
}

// 根据 props.trigger 和设备能力决定当前使用 hover 还是 click
const updateTriggerMode = () => {
  if (props.trigger === 'hover') {
    isHoverMode.value = true
  } else if (props.trigger === 'click') {
    isHoverMode.value = false
  } else if (typeof window !== 'undefined') {
    isHoverMode.value = window.matchMedia('(hover: hover) and (pointer: fine)').matches
  }
}

// 获取触发元素的实际位置（display: contents 时用第一个子元素）
const getTriggerRect = () => {
  if (!triggerRef.value) return null
  // trigger 使用 display: contents，该元素本身不生成盒模型
  // getBoundingClientRect() 会返回全 0，需要用内部子元素获取真实位置
  const el = triggerRef.value.firstElementChild || triggerRef.value
  return el.getBoundingClientRect()
}

// 检测最优方向（自动翻转）
const resolveSide = (rect: DOMRect): typeof props.side => {
  const spaceAbove = rect.top
  const spaceBelow = window.innerHeight - rect.bottom
  const spaceLeft = rect.left
  const spaceRight = window.innerWidth - rect.right

  switch (props.side) {
    case 'top':
      return spaceAbove < 60 && spaceBelow > spaceAbove ? 'bottom' : 'top'
    case 'bottom':
      return spaceBelow < 60 && spaceAbove > spaceBelow ? 'top' : 'bottom'
    case 'left':
      return spaceLeft < 60 && spaceRight > spaceLeft ? 'right' : 'left'
    case 'right':
      return spaceRight < 60 && spaceLeft > spaceRight ? 'left' : 'right'
    default:
      return props.side
  }
}

// 计算定位（含防撞）
const updatePosition = () => {
  const rect = getTriggerRect()
  if (!rect) return

  // 先渲染一次让浏览器计算内容尺寸
  const style: Record<string, string> = {
    position: 'fixed',
    zIndex: '100000',
    visibility: 'hidden'
  }
  contentStyle.value = style

  // 下一帧获取内容尺寸并计算最终位置
  requestAnimationFrame(() => {
    if (!contentRef.value) return
    const contentRect = contentRef.value.getBoundingClientRect()
    const side = resolveSide(rect)
    resolvedSide.value = side

    const finalStyle: Record<string, string> = {
      position: 'fixed',
      zIndex: '100000',
      visibility: 'visible'
    }

    // 垂直定位
    if (side === 'top') {
      finalStyle.top = `${rect.top - contentRect.height - props.sideOffset}px`
    } else if (side === 'bottom') {
      finalStyle.top = `${rect.bottom + props.sideOffset}px`
    } else if (side === 'left') {
      finalStyle.left = `${rect.left - contentRect.width - props.sideOffset}px`
    } else if (side === 'right') {
      finalStyle.left = `${rect.right + props.sideOffset}px`
    }

    // 水平对齐
    if (side === 'top' || side === 'bottom') {
      if (props.align === 'center') {
        let left = rect.left + rect.width / 2 - contentRect.width / 2
        left = Math.max(VIEWPORT_PADDING, Math.min(left, window.innerWidth - contentRect.width - VIEWPORT_PADDING))
        finalStyle.left = `${left}px`
      } else if (props.align === 'start') {
        let left = rect.left
        left = Math.min(left, window.innerWidth - contentRect.width - VIEWPORT_PADDING)
        finalStyle.left = `${left}px`
      } else {
        let right = window.innerWidth - rect.right
        right = Math.max(VIEWPORT_PADDING, Math.min(right, window.innerWidth - contentRect.width - VIEWPORT_PADDING))
        finalStyle.right = `${right}px`
      }
    }

    // 垂直对齐（left/right 方向时）
    if (side === 'left' || side === 'right') {
      if (props.align === 'center') {
        let top = rect.top + rect.height / 2 - contentRect.height / 2
        top = Math.max(VIEWPORT_PADDING, Math.min(top, window.innerHeight - contentRect.height - VIEWPORT_PADDING))
        finalStyle.top = `${top}px`
      } else if (props.align === 'start') {
        let top = rect.top
        top = Math.min(top, window.innerHeight - contentRect.height - VIEWPORT_PADDING)
        finalStyle.top = `${top}px`
      } else {
        let bottom = window.innerHeight - rect.bottom
        bottom = Math.max(VIEWPORT_PADDING, Math.min(bottom, window.innerHeight - contentRect.height - VIEWPORT_PADDING))
        finalStyle.bottom = `${bottom}px`
      }
    }

    // 垂直防撞（top/bottom 方向时）
    if (side === 'top' || side === 'bottom') {
      const topVal = parseFloat(finalStyle.top || '0')
      if (topVal < VIEWPORT_PADDING) {
        finalStyle.top = `${VIEWPORT_PADDING}px`
      }
      if (topVal + contentRect.height > window.innerHeight - VIEWPORT_PADDING) {
        finalStyle.top = `${window.innerHeight - contentRect.height - VIEWPORT_PADDING}px`
      }
    }

    contentStyle.value = finalStyle
  })
}

// 计算动画偏移方向（基于实际解析后的方向）
const contentTransform = computed(() => {
  let transform = ''

  if (props.align === 'center') {
    if (resolvedSide.value === 'top' || resolvedSide.value === 'bottom') {
      // center 对齐时不需要额外 transform，定位已通过 left 计算
    } else {
      // left/right 方向的 center 对齐也不需要
    }
  }

  // 动画偏移（未显示时添加）
  if (!isVisible.value) {
    const offset = '4px'
    switch (resolvedSide.value) {
      case 'top':
        transform += `translateY(${offset})`
        break
      case 'bottom':
        transform += `translateY(-${offset})`
        break
      case 'left':
        transform += `translateX(${offset})`
        break
      case 'right':
        transform += `translateX(-${offset})`
        break
    }
  }

  return transform || 'none'
})

const contentFinalStyle = computed(() => {
  const style = { ...contentStyle.value }
  style.transform = contentTransform.value
  style.opacity = isVisible.value ? '1' : '0'
  style.transition = 'opacity 0.2s ease, transform 0.2s ease'
  return style
})

const showContent = () => {
  updatePosition()
  isOpen.value = true
  requestAnimationFrame(() => {
    requestAnimationFrame(() => {
      isVisible.value = true
    })
  })
}

const hideContent = () => {
  isVisible.value = false
  setTimeout(() => {
    isOpen.value = false
  }, 220)
}

const handleMouseEnter = () => {
  clearTimers()
  openTimer = setTimeout(() => {
    showContent()
  }, props.openDelay)
}

const handleMouseLeave = () => {
  clearTimers()
  closeTimer = setTimeout(() => {
    hideContent()
  }, props.closeDelay)
}

const handleContentMouseEnter = () => {
  clearTimers()
}

const handleContentMouseLeave = () => {
  closeTimer = setTimeout(() => {
    hideContent()
  }, props.closeDelay)
}

// 点击触发（仅 click 模式生效）
const handleTriggerClick = () => {
  if (isHoverMode.value) return
  if (isOpen.value) {
    hideContent()
  } else {
    showContent()
  }
}

// 点击外部关闭（仅 click 模式生效）
const handleDocumentClick = (e: MouseEvent) => {
  if (!isOpen.value || isHoverMode.value) return
  const target = e.target as Node
  if (triggerRef.value?.contains(target) || contentRef.value?.contains(target)) return
  hideContent()
}

// ESC 关闭
const handleKeydown = (e: KeyboardEvent) => {
  if (!isOpen.value) return
  if (e.key === 'Escape') {
    hideContent()
  }
}

onMounted(() => {
  updateTriggerMode()
  if (props.trigger === 'auto' && typeof window !== 'undefined') {
    hoverModeMediaQuery = window.matchMedia('(hover: hover) and (pointer: fine)')
    if (hoverModeMediaQuery.addEventListener) {
      hoverModeMediaQuery.addEventListener('change', updateTriggerMode)
    } else {
      hoverModeMediaQuery.addListener(updateTriggerMode)
    }
  }
  document.addEventListener('click', handleDocumentClick, true)
  document.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  clearTimers()
  if (hoverModeMediaQuery) {
    if (hoverModeMediaQuery.removeEventListener) {
      hoverModeMediaQuery.removeEventListener('change', updateTriggerMode)
    } else {
      hoverModeMediaQuery.removeListener(updateTriggerMode)
    }
  }
  document.removeEventListener('click', handleDocumentClick, true)
  document.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <div
    ref="triggerRef"
    class="hover-card-trigger"
    @mouseenter="handleMouseEnter"
    @mouseleave="handleMouseLeave"
    @click="handleTriggerClick"
  >
    <slot />
  </div>
  <Teleport to="body">
    <div
      v-if="isOpen"
      ref="contentRef"
      class="hover-card-content"
      :style="contentFinalStyle"
      :data-side="resolvedSide"
      @mouseenter="handleContentMouseEnter"
      @mouseleave="handleContentMouseLeave"
    >
      <slot name="content" />
    </div>
  </Teleport>
</template>

<style scoped>
.hover-card-trigger {
  display: contents;
}

.hover-card-content {
  pointer-events: auto;
}
</style>
