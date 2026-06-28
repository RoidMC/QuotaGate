/**
 * 客户端专用 Composable
 * 解决 SSR 水合不匹配问题
 */
import type { WatchOptions } from 'vue'

/**
 * 创建客户端专用的响应式引用
 * @param initialValue 初始值
 * @param clientValue 客户端实际值（可选）
 * @returns 响应式引用
 */
export function useClientRef<T>(initialValue: T, clientValue?: T) {
  const value = ref(initialValue)
  
  if (import.meta.client) {
    onMounted(() => {
      if (clientValue !== undefined) {
        value.value = clientValue
      }
    })
  }
  
  return value
}

/**
 * 创建客户端专用的计算属性
 * @param getter 计算函数
 * @param initialValue 初始值（服务端使用）
 * @returns 计算属性
 */
export function useClientComputed<T>(getter: () => T, initialValue: T) {
  const isMounted = ref(false)
  
  onMounted(() => {
    isMounted.value = true
  })
  
  return computed(() => {
    if (import.meta.server || !isMounted.value) {
      return initialValue
    }
    return getter()
  })
}

/**
 * 检查是否在客户端环境
 * @returns boolean
 */
export function useIsClient() {
  return computed(() => import.meta.client)
}

/**
 * 客户端专用副作用
 * @param effectFn 副作用函数
 * @param options watch 选项
 */
export function useClientEffect(
  effectFn: () => void | (() => void), 
  options?: WatchOptions
) {
  const cleanup = ref<(() => void) | null>(null)
  
  onMounted(() => {
    if (import.meta.client) {
      const result = effectFn()
      if (typeof result === 'function') {
        cleanup.value = result
      }
    }
  })
  
  onUnmounted(() => {
    if (cleanup.value) {
      cleanup.value()
      cleanup.value = null
    }
  })
}