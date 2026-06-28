export default defineNuxtPlugin(() => {
  const config = useRuntimeConfig()
  const analyticsId = config.baiduAnalyticsId as string

  // 没有配置百度统计 ID 则不加载
  if (!analyticsId || !import.meta.client) return

  var _hmt: any[] = (window as any)._hmt || [];
  (function() {
    var hm = document.createElement("script");
    hm.src = `https://hm.baidu.com/hm.js?${analyticsId}`;
    var s = document.getElementsByTagName("script")[0];
    if (s && s.parentNode) {
      s.parentNode.insertBefore(hm, s);
    }
  })();

  if (window.innerWidth > 992) {
    parent.postMessage({ drawer: false }, '*');
  }
})
