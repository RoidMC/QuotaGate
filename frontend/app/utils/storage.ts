/**
 * 通用存储工具 - 支持 SSR 环境
 * 提供 Cookie 和 LocalStorage 的统一访问接口
 */

// ==================== 类型定义 ====================
export type StorageValue = string | number | boolean | object | null | undefined

export interface StorageOptions {
  /** 过期时间（秒） */
  maxAge?: number
  /** Cookie 路径 */
  path?: string
  /** Cookie 域名 */
  domain?: string
  /** 是否仅 HTTPS */
  secure?: boolean
  /** 是否允许跨域访问 */
  sameSite?: 'strict' | 'lax' | 'none'
}

// ==================== 环境检测 ====================
const isClient = typeof window !== 'undefined'

const hasLocalStorage = (): boolean => {
  if (!isClient) return false
  try {
    const testKey = '__storage_test__'
    localStorage.setItem(testKey, testKey)
    localStorage.removeItem(testKey)
    return true
  } catch {
    return false
  }
}

const hasCookie = (): boolean => isClient && typeof document !== 'undefined'

// ==================== Cookie 操作 ====================
/**
 * 设置 Cookie
 * SSR 环境下也可以使用（服务端设置 Cookie header）
 */
export function setCookie(
  name: string,
  value: string,
  options: StorageOptions = {}
): void {
  if (!hasCookie()) return
  
  const {
    maxAge,
    path = '/',
    domain,
    secure = false,
    sameSite = 'lax'
  } = options
  
  let cookieString = `${encodeURIComponent(name)}=${encodeURIComponent(value)}`
  
  if (maxAge) {
    const expires = new Date(Date.now() + maxAge * 1000)
    cookieString += `; expires=${expires.toUTCString()}`
  }
  
  cookieString += `; path=${path}`
  
  if (domain) {
    cookieString += `; domain=${domain}`
  }
  
  if (secure) {
    cookieString += '; secure'
  }
  
  cookieString += `; samesite=${sameSite}`
  
  document.cookie = cookieString
}

/**
 * 获取 Cookie
 */
export function getCookie(name: string): string | null {
  if (!hasCookie()) return null
  
  const cookies = document.cookie.split(';').map(cookie => cookie.trim())
  
  for (const cookie of cookies) {
    const parts = cookie.split('=')
    const cookieName = decodeURIComponent(parts[0]?.trim() || '')
    const cookieValue = parts.length > 1 ? decodeURIComponent(parts[1] || '') : ''
    
    if (cookieName === name) {
      return cookieValue
    }
  }
  
  return null
}

/**
 * 删除 Cookie
 */
export function removeCookie(name: string, path = '/'): void {
  if (!hasCookie()) return
  document.cookie = `${name}=; path=${path}; max-age=0`
}

// ==================== LocalStorage 操作 ====================
/**
 * 设置 LocalStorage
 * 自动序列化对象为 JSON
 */
export function setLocalStorage<T extends StorageValue>(
  key: string,
  value: T
): void {
  if (!hasLocalStorage()) return
  
  try {
    const stringValue = typeof value === 'object' ? JSON.stringify(value) : String(value ?? '')
    localStorage.setItem(key, stringValue)
  } catch (error) {
    console.error(`[Storage] Failed to set localStorage key "${key}":`, error)
  }
}

/**
 * 获取 LocalStorage
 * 自动反序列化 JSON 字符串
 */
export function getLocalStorage<T extends StorageValue>(key: string): T | null {
  if (!hasLocalStorage()) return null
  
  try {
    const item = localStorage.getItem(key)
    if (item === null) return null
    
    // 尝试解析 JSON
    if (item.startsWith('{') || item.startsWith('[')) {
      return JSON.parse(item) as T
    }
    
    // 返回原始字符串
    return item as T
  } catch (error) {
    console.error(`[Storage] Failed to get localStorage key "${key}":`, error)
    return null
  }
}

/**
 * 删除 LocalStorage
 */
export function removeLocalStorage(key: string): void {
  if (!hasLocalStorage()) return
  try {
    localStorage.removeItem(key)
  } catch (error) {
    console.error(`[Storage] Failed to remove localStorage key "${key}":`, error)
  }
}

/**
 * 清空所有 LocalStorage
 */
export function clearLocalStorage(): void {
  if (!hasLocalStorage()) return
  try {
    localStorage.clear()
  } catch (error) {
    console.error(`[Storage] Failed to clear localStorage:`, error)
  }
}

// ==================== SessionStorage 操作 ====================
const hasSessionStorage = (): boolean => {
  if (!isClient) return false
  try {
    const testKey = '__storage_test__'
    sessionStorage.setItem(testKey, testKey)
    sessionStorage.removeItem(testKey)
    return true
  } catch {
    return false
  }
}

export function setSessionStorage<T extends StorageValue>(
  key: string,
  value: T
): void {
  if (!hasSessionStorage()) return
  
  try {
    const stringValue = typeof value === 'object' ? JSON.stringify(value) : String(value ?? '')
    sessionStorage.setItem(key, stringValue)
  } catch (error) {
    console.error(`[Storage] Failed to set sessionStorage key "${key}":`, error)
  }
}

export function getSessionStorage<T extends StorageValue>(key: string): T | null {
  if (!hasSessionStorage()) return null
  
  try {
    const item = sessionStorage.getItem(key)
    if (item === null) return null
    
    if (item.startsWith('{') || item.startsWith('[')) {
      return JSON.parse(item) as T
    }
    
    return item as T
  } catch (error) {
    console.error(`[Storage] Failed to get sessionStorage key "${key}":`, error)
    return null
  }
}

export function removeSessionStorage(key: string): void {
  if (!hasSessionStorage()) return
  try {
    sessionStorage.removeItem(key)
  } catch (error) {
    console.error(`[Storage] Failed to remove sessionStorage key "${key}":`, error)
  }
}
