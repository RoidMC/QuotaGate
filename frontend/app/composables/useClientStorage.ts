/**
 * 客户端存储 Hook - 纯客户端使用
 * 封装 localStorage 和 sessionStorage，提供响应式 API
 */

import { ref, watch, type Ref } from 'vue'
import {
  getLocalStorage,
  setLocalStorage,
  removeLocalStorage,
  getSessionStorage,
  setSessionStorage,
  removeSessionStorage,
  type StorageValue
} from '@/utils/storage'

// ==================== 类型定义 ====================
export interface UseStorageOptions<T extends StorageValue> {
  /** 序列化函数 */
  serializer?: (value: T) => string
  /** 反序列化函数 */
  deserializer?: (value: string) => T
  /** 监听变化（仅客户端） */
  watchChanges?: boolean
}

// ==================== LocalStorage Hook ====================
/**
 * 响应式 LocalStorage
 * @param key 存储键名
 * @param initialValue 初始值
 * @param options 配置选项
 */
export function useLocalStorage<T extends StorageValue>(
  key: string,
  initialValue: T,
  options: UseStorageOptions<T> = {}
): Ref<T> {
  const {
    serializer = JSON.stringify,
    deserializer = JSON.parse,
    watchChanges = true
  } = options
  
  // 服务端渲染时返回初始值
  if (typeof window === 'undefined') {
    return ref(initialValue) as Ref<T>
  }
  
  // 从 localStorage 获取值
  const storedValue = getLocalStorage<T>(key)
  const state = ref(storedValue !== null ? storedValue : initialValue) as Ref<T>
  
  // 监听状态变化并同步到 localStorage
  if (watchChanges) {
    watch(
      state,
      (newValue) => {
        if (newValue === null || newValue === undefined) {
          removeLocalStorage(key)
        } else {
          try {
            const serialized = typeof newValue === 'string' ? newValue : serializer(newValue)
            setLocalStorage(key, serialized)
          } catch (error) {
            console.error(`[useLocalStorage] Failed to save "${key}":`, error)
          }
        }
      },
      { deep: true }
    )
  }
  
  return state
}

/**
 * 非响应式 LocalStorage 操作
 */
export function useLocalStorageAPI() {
  return {
    get: <T extends StorageValue>(key: string, defaultValue?: T): T | null => {
      const value = getLocalStorage<T>(key)
      return value !== null ? value : (defaultValue ?? null)
    },
    
    set: <T extends StorageValue>(key: string, value: T): void => {
      setLocalStorage(key, value)
    },
    
    remove: (key: string): void => {
      removeLocalStorage(key)
    },
    
    clear: (): void => {
      // clearLocalStorage()
    }
  }
}

// ==================== SessionStorage Hook ====================
/**
 * 响应式 SessionStorage
 */
export function useSessionStorage<T extends StorageValue>(
  key: string,
  initialValue: T,
  options: UseStorageOptions<T> = {}
): Ref<T> {
  const {
    serializer = JSON.stringify,
    deserializer = JSON.parse,
    watchChanges = true
  } = options
  
  // SSR 保护
  if (typeof window === 'undefined') {
    return ref(initialValue) as Ref<T>
  }
  
  const storedValue = getSessionStorage<T>(key)
  const state = ref(storedValue !== null ? storedValue : initialValue) as Ref<T>
  
  if (watchChanges) {
    watch(
      state,
      (newValue) => {
        if (newValue === null || newValue === undefined) {
          removeSessionStorage(key)
        } else {
          try {
            const serialized = typeof newValue === 'string' ? newValue : serializer(newValue)
            setSessionStorage(key, serialized)
          } catch (error) {
            console.error(`[useSessionStorage] Failed to save "${key}":`, error)
          }
        }
      },
      { deep: true }
    )
  }
  
  return state
}

/**
 * 非响应式 SessionStorage 操作
 */
export function useSessionStorageAPI() {
  return {
    get: <T extends StorageValue>(key: string, defaultValue?: T): T | null => {
      const value = getSessionStorage<T>(key)
      return value !== null ? value : (defaultValue ?? null)
    },
    
    set: <T extends StorageValue>(key: string, value: T): void => {
      setSessionStorage(key, value)
    },
    
    remove: (key: string): void => {
      removeSessionStorage(key)
    }
  }
}

// ==================== Cookie Hook (结合 Nuxt useCookie) ====================
/**
 * Cookie 响应式 Hook
 * 注意：SSR 环境下需要使用 Nuxt 的 useCookie
 * 这个主要用于纯客户端场景
 */
export function useClientCookie<T extends StorageValue>(
  key: string,
  initialValue: T
): Ref<T> {
  // SSR 保护
  if (typeof document === 'undefined') {
    return ref(initialValue) as Ref<T>
  }
  
  const storedValue = getCookieTyped<T>(key)
  const state = ref(storedValue !== null ? storedValue : initialValue) as Ref<T>
  
  watch(
    state,
    (newValue) => {
      if (newValue === null || newValue === undefined) {
        // 这里可以调用 removeCookie
      } else {
        try {
          const serialized = typeof newValue === 'string' ? newValue : JSON.stringify(newValue)
          setCookieWithExpiry(key, serialized)
        } catch (error) {
          console.error(`[useClientCookie] Failed to save "${key}":`, error)
        }
      }
    },
    { deep: true }
  )
  
  return state
}

// Cookie 辅助函数（带过期时间）
function setCookieWithExpiry(key: string, value: string, maxAge = 604800): void {
  if (typeof document === 'undefined') return
  
  const expires = new Date(Date.now() + maxAge * 1000)
  document.cookie = `${encodeURIComponent(key)}=${encodeURIComponent(value)}; max-age=${maxAge}; path=/; samesite=lax`
}

function getCookieTyped<T>(key: string): T | null {
  if (typeof document === 'undefined') return null
  
  const cookies = document.cookie.split(';').map(c => c.trim())
  for (const cookie of cookies) {
    const parts = cookie.split('=')
    const cookieName = decodeURIComponent(parts[0]?.trim() || '')
    const cookieValue = parts.length > 1 ? decodeURIComponent(parts[1] || '') : ''
    if (cookieName === key) {
      try {
        return cookieValue as unknown as T
      } catch {
        return cookieValue as T
      }
    }
  }
  return null
}

// ==================== 统一导出 ====================
/**
 * 统一的存储 Hook 接口
 */
export const useClientStorage = () => {
  return {
    local: useLocalStorageAPI(),
    session: useSessionStorageAPI(),
    
    // 响应式版本
    useLocalStorage,
    useSessionStorage,
    useClientCookie
  }
}

// 默认导出
export default useClientStorage
