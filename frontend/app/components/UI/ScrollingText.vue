<script lang="ts" setup>
interface Props {
  text: string
  speed?: number // 滚动速度(px/s)
  repeatCount?: number // 重复次数(0表示无限)
  separator?: string // 自定义分隔符
  enableWordSplit?: boolean // 是否启用空格分词
  textStyle?: Record<string, any> // 文本样式配置（已废弃，建议使用textClass）
  textClass?: string // 文本CSS类名
}

const props = withDefaults(defineProps<Props>(), {
  speed: 30,
  repeatCount: 0, // 0表示无限滚动
  separator: ' • ', // 默认分隔符
  enableWordSplit: true, // 默认启用分词
  textStyle: () => ({}), // 默认空样式对象（已废弃）
  textClass: '' // 默认空类名
})

const containerRef = ref<HTMLElement | null>(null)
const wrapperRef = ref<HTMLElement | null>(null)
const animationRef = ref<number | null>(null)
const isReady = ref(false)

// 更新现有文本项的内容和样式
const updateTextItems = (contentWrapper: HTMLElement, processedText: string) => {
  const items = contentWrapper.querySelectorAll('.scrolling-text-item')
  items.forEach(item => {
    const textItem = item as HTMLElement
    textItem.textContent = processedText

    // 应用传入的文本样式（兼容旧版，但标记为废弃）
    Object.entries(props.textStyle).forEach(([key, value]) => {
      //@ts-ignore
      textItem.style[key] = value
    })
  })
}

// 调整副本数量以适应容器宽度
const adjustCopyCount = (contentWrapper: HTMLElement, wrapper: HTMLElement, processedText: string) => {
  // 计算单个项目的宽度
  const tempItem = document.createElement('div')
  tempItem.className = 'scrolling-text-item'
  tempItem.textContent = processedText
  tempItem.style.position = 'absolute'
  tempItem.style.visibility = 'hidden'
  tempItem.style.whiteSpace = 'pre'

  // 添加自定义类名用于样式计算
  if (props.textClass) {
    tempItem.classList.add(...props.textClass.split(' ').filter(cls => cls))
  }

  document.body.appendChild(tempItem)
  const itemWidth = tempItem.offsetWidth
  document.body.removeChild(tempItem)

  if (itemWidth === 0) return

  // 获取容器宽度并计算所需副本数
  const containerWidth = wrapper.clientWidth
  const minCopies = Math.ceil(containerWidth / itemWidth) + 2 // 多加2个确保充足
  const targetCopies = Math.max(4, minCopies) // 至少4个副本

  const currentItems = contentWrapper.querySelectorAll('.scrolling-text-item')
  const currentCount = currentItems.length

  if (currentCount < targetCopies) {
    // 需要添加更多副本
    for (let i = currentCount; i < targetCopies; i++) {
      const item = document.createElement('div')
      item.className = 'scrolling-text-item'

      // 添加自定义类名
      if (props.textClass) {
        item.classList.add(...props.textClass.split(' ').filter(cls => cls))
      }

      item.textContent = processedText
      item.style.whiteSpace = 'pre' // 确保保持空格

      // 应用传入的文本样式
      Object.entries(props.textStyle).forEach(([key, value]) => {
        //@ts-ignore
        item.style[key] = value
      })

      contentWrapper.appendChild(item)
    }
  } else if (currentCount > targetCopies) {
    // 需要移除多余副本（保留前targetCopies个）
    for (let i = currentCount - 1; i >= targetCopies; i--) {
      const item = currentItems[i]
      if (item) {
        item.remove()
      }
    }
  }
}

// 处理文本内容
const processText = () => {
  const originalText = props.text

  // 根据分词开关处理文本
  let processedText: string
  if (props.enableWordSplit) {
    // 启用分词：在每个词组后添加自定义分隔符，但保留原始空格结构
    const parts = originalText.split(/(\s+)/) // 按空格分割，但保留空格作为独立元素
    processedText = parts
      .map(part => {
        if (part.trim() === '') {
          // 如果是纯空格，保持原样
          return part
        } else {
          // 如果是非空格内容，在后面添加分隔符
          return part + props.separator
        }
      })
      .join('')
  } else {
    // 不分词：直接在整个文本后添加分隔符
    processedText = originalText + props.separator
  }

  return processedText
}

// 初始化滚动内容 - 创建基础副本
const initScrollContent = () => {
  const wrapper = wrapperRef.value
  const container = containerRef.value

  if (!wrapper || !container) return

  const processedText = processText()

  // 如果还没有内容容器，则创建基础结构
  let contentWrapper = container.querySelector('.scrolling-text-content-wrapper') as HTMLElement
  if (!contentWrapper) {
    // 立即隐藏容器以防止闪烁
    wrapper.style.opacity = '0'
    wrapper.style.transition = 'opacity 0s'

    // 清空现有内容
    container.innerHTML = ''

    // 创建内容容器
    contentWrapper = document.createElement('div')
    contentWrapper.className = 'scrolling-text-content-wrapper'
    contentWrapper.style.display = 'flex'
    contentWrapper.style.width = 'fit-content'
    container.appendChild(contentWrapper)

    // 标记准备就绪并淡入显示
    isReady.value = true
    nextTick(() => {
      wrapper.style.transition = 'opacity 0.3s ease'
      wrapper.style.opacity = '1'
    })
  }

  // 更新文本内容和样式
  updateTextItems(contentWrapper, processedText)

  // 调整副本数量
  adjustCopyCount(contentWrapper, wrapper, processedText)

  // 启动或重启JS动画
  startJSAnimation()
}

// 每个组件实例的动画状态
const animationState = reactive({
  currentPosition: 0,
  itemWidth: 0
})

// 重新计算动画参数
const recalculateAnimationParams = () => {
  const container = containerRef.value
  if (!container) return

  const contentWrapper = container.querySelector('.scrolling-text-content-wrapper') as HTMLElement
  if (!contentWrapper) return

  // 获取单个内容项的宽度
  const firstItem = contentWrapper.querySelector('.scrolling-text-item') as HTMLElement
  if (!firstItem) return

  animationState.itemWidth = firstItem.offsetWidth
  if (animationState.itemWidth === 0) return

  // 设置容器宽度为两个内容项的总宽度
  contentWrapper.style.width = (animationState.itemWidth * 2) + 'px'
}

// JS动画实现 - 核心逻辑
const startJSAnimation = () => {
  const container = containerRef.value
  if (!container) return

  // 重新计算动画参数
  recalculateAnimationParams()
  if (animationState.itemWidth === 0) return

  // 使用requestAnimationFrame实现流畅动画
  const pixelsPerFrame = props.speed / 60 // 60fps下的每帧移动距离

  const animate = () => {
    animationState.currentPosition -= pixelsPerFrame

    // 关键：当移动到一半位置时重置位置，实现无缝循环
    if (Math.abs(animationState.currentPosition) >= animationState.itemWidth) {
      animationState.currentPosition = animationState.currentPosition + animationState.itemWidth
    }

    // 重新获取contentWrapper以防DOM变化
    const currentContainer = containerRef.value
    if (currentContainer) {
      const currentContentWrapper = currentContainer.querySelector('.scrolling-text-content-wrapper') as HTMLElement
      if (currentContentWrapper) {
        currentContentWrapper.style.transform = `translateX(${animationState.currentPosition}px)`
      }
    }

    animationRef.value = requestAnimationFrame(animate)
  }

  // 启动动画循环
  animate()
}

// 停止动画
const stopAnimation = () => {
  if (animationRef.value) {
    cancelAnimationFrame(animationRef.value)
    animationRef.value = null
  }
}

// 监听内容、分隔符和分词开关变化
watch([() => props.text, () => props.separator, () => props.enableWordSplit, () => props.speed], () => {
  nextTick(() => {
    stopAnimation()
    initScrollContent()
  })
})

// 监听窗口大小变化
const handleResize = () => {
  nextTick(() => {
    // 只调整副本数量，不中断动画
    const wrapper = wrapperRef.value
    const container = containerRef.value
    if (!wrapper || !container) return

    const contentWrapper = container.querySelector('.scrolling-text-content-wrapper') as HTMLElement
    if (!contentWrapper) return

    const processedText = processText()
    adjustCopyCount(contentWrapper, wrapper, processedText)

    // 重新计算动画参数但不停止现有动画
    recalculateAnimationParams()
  })
}

onMounted(async () => {
  await nextTick()
  initScrollContent()

  // 添加窗口大小变化监听
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  stopAnimation()
  // 重置动画参数
  animationState.currentPosition = 0
  animationState.itemWidth = 0
  // 移除事件监听
  window.removeEventListener('resize', handleResize)
})
</script>

<template>
  <div ref="wrapperRef" class="scrolling-text-wrapper"
    :style="{ opacity: isReady ? 1 : 0, transition: 'opacity 0.3s ease' }">
    <div ref="containerRef" class="scrolling-text-container">
      <!-- 内容将在脚本中动态生成 -->
    </div>
  </div>
</template>

<style lang="scss" scoped>
.scrolling-text-wrapper {
  overflow: hidden;
  white-space: nowrap;
  width: 100%;
  line-height: 1;
}

.scrolling-text-container {
  display: flex;
  width: max-content;
  position: relative;
  will-change: transform;
  backface-visibility: hidden;
  transform: translateZ(0);
  min-width: 100%;
}

.scrolling-text-content-wrapper {
  display: flex;
  width: 100%;
}

.scrolling-text-item {
  flex-shrink: 0;
  display: inline-block;
  white-space: pre;
  /* 保持原始空格格式 */
}
</style>