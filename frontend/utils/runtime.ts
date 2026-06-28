import { execSync } from 'child_process'
import { readFileSync } from 'fs'
import { join } from 'path'

interface RUNTIME_CONFIG {
  HORIZON_SITE_ENV: string
  [key: string]: string | undefined
}

// 使用 Set 控制日志去重，只对白名单命令显示日志
const allowedCommands = new Set(['build', 'dev', 'generate', 'analyze'])
const loggedKeys = new Set<string>()
const shouldLog = allowedCommands.has(process.env.npm_lifecycle_event || '')

function logOnce(key: string, message: string) {
  if (shouldLog && !loggedKeys.has(key)) {
    loggedKeys.add(key)
    console.log(message)
  }
}

// 格式化构建时间（带时区）
function formatBuildTime(): string {
  const timeZone = 'Asia/Shanghai'
  const now = new Date()
  const timeString = now.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
    timeZone
  })

  // 获取时区偏移
  const formatter = new Intl.DateTimeFormat('en', { timeZone, timeZoneName: 'short' })
  const timeZoneName = formatter.formatToParts(now).find(p => p.type === 'timeZoneName')?.value || ''

  // 提取 GMT 偏移量，如果没有则使用本地时区
  let gmtOffset = ''
  if (timeZoneName.includes('GMT')) {
    gmtOffset = ` (${timeZoneName})`
  } else {
    // 计算本地时区的 GMT 偏移
    const localOffset = -now.getTimezoneOffset()
    const sign = localOffset >= 0 ? '+' : '-'
    const hours = Math.floor(Math.abs(localOffset) / 60)
    const minutes = Math.abs(localOffset) % 60
    gmtOffset = minutes > 0 ? ` (GMT${sign}${hours}:${minutes})` : ` (GMT${sign}${hours})`
  }

  return timeString + gmtOffset
}

// 获取 Git 提交 hash（环境变量获取失败时，使用 git 命令）
function getGitHash(): string {
  try {
    return execSync('git rev-parse HEAD', { encoding: 'utf8' }).trim().substring(0, 8)
  } catch {
    return 'unknown'
  }
}

// 获取 Git 分支（环境变量获取失败时，使用 git 命令）
function getGitBranch(): string {
  try {
    return execSync('git rev-parse --abbrev-ref HEAD', { encoding: 'utf8' }).trim()
  } catch {
    return 'unknown'
  }
}

// 优先使用 CI/CD 平台的环境变量，同时返回平台名
function getGitInfo(): { hash: string; branch: string; platform: string } {
  // Cloudflare Pages
  if (process.env.CF_PAGES_COMMIT_SHA) {
    return {
      hash: process.env.CF_PAGES_COMMIT_SHA.substring(0, 8),
      branch: process.env.CF_PAGES_BRANCH || 'unknown',
      platform: 'Cloudflare-Pages'
    }
  }

  // 腾讯云 EdgeOne Pages
  if (process.env.CI_PLATFORM_TYPE === 'EdgeOne-Pages') {
    const hash = process.env.EDGEONE_COMMIT_SHA?.substring(0, 8) || getGitHash()
    const branch = process.env.EDGEONE_BRANCH || process.env.CI_COMMIT_REF_NAME || getGitBranch()
    return { hash, branch, platform: 'EdgeOne-Pages' }
  }

  // Vercel
  if (process.env.VERCEL_GIT_COMMIT_SHA) {
    return {
      hash: process.env.VERCEL_GIT_COMMIT_SHA.substring(0, 8),
      branch: process.env.VERCEL_GIT_COMMIT_REF || 'unknown',
      platform: 'Vercel'
    }
  }

  // GitHub Actions
  if (process.env.GITHUB_SHA) {
    return {
      hash: process.env.GITHUB_SHA.substring(0, 8),
      branch: process.env.GITHUB_REF_NAME || 'unknown',
      platform: 'GitHub-Actions'
    }
  }

  // 本地开发环境使用 git 命令
  return { hash: getGitHash(), branch: getGitBranch(), platform: 'local' }
}

// EdgeOne Pages 缺少需要的 Git 信息，需要手动配置 CI_PLATFORM_TYPE
export function createRuntimeConfig() {
  const envScript = process.env.npm_lifecycle_script?.split(' ') || []
  let envName = 'development' // 默认环境

  // 通过启动命令区分环境
  if (envScript.length > 0) {
    // 查找命令中的实际操作部分（nuxt build -> build, nuxt dev -> dev）
    const commandIndex = envScript.findIndex(cmd => ['build', 'dev', 'generate', 'preview'].includes(cmd))
    const command = commandIndex !== -1 ? envScript[commandIndex] : envScript[0]

    if (command === 'build' || command === 'generate') {
      envName = 'production'
    } else if (command === 'dev') {
      envName = 'development'
    }

    // 检查是否有明确的环境参数
    const lastArg = envScript[envScript.length - 1]
    if (lastArg === 'production') {
      envName = 'production'
    } else if (lastArg === 'development' || lastArg === 'dev') {
      envName = 'development'
    }
  }

  const envData = process.env as unknown as RUNTIME_CONFIG

  // 从环境变量获取 APP_ENV，如果没有则使用 envName
  const appEnv = envData.HORIZON_SITE_ENV || envName.toUpperCase()

  // 获取 package.json 版本信息
  let packageVersion = '0.0.0-Unknown'
  let packageName = 'Unknown'
  try {
    const pkg = JSON.parse(readFileSync(join(process.cwd(), 'package.json'), 'utf8'))
    packageVersion = pkg.version || '0.0.0-Unknown'
    packageName = pkg.name || 'Unknown'
    logOnce('package', `✅ Package: ${packageName}@${packageVersion}`)
  } catch (error) {
    console.warn('❌ 无法获取 Package 信息:', error)
  }

  // Git 信息 + 平台检测（合并处理，避免重复判断）
  const { hash: gitHash, branch: gitBranch, platform } = getGitInfo()
  logOnce('git', `✅ Git: ${gitHash} | ${gitBranch} (${platform})`)
  logOnce('env', `✅ Env: ${envName.charAt(0).toUpperCase() + envName.slice(1)}`)

  // 生成格式：timestamp-gitHash-branch-platform
  const timestamp = Date.now().toString(36)
  const buildId = `${timestamp}-${gitHash}-${gitBranch}-${platform}`
  logOnce('build', `✅ Build ID: ${buildId}`)

  return {
    runtimeConfig: {
      public: {
        appEnv,
        gitHash,
        gitBranch,
        packageName,
        packageVersion,
        buildId,
        buildTime: formatBuildTime(),
        // 百度统计 ID，留空不加载
        baiduAnalyticsId: process.env.BAIDU_ANALYTICS_ID || ''
      } as any // 把env放入这个里面，通过useRuntimeConfig获取
    }
  }
}
