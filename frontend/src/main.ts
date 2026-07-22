import { createApp } from "vue";
import { createPinia } from "pinia";
import App from "./App.vue";
import router, { installAuthGuard } from "./router";
import "./assets/fonts.css";
import TDesign from "tdesign-vue-next";
// 引入组件库的少量全局样式变量
import "tdesign-vue-next/es/style/index.css";
import "@/assets/dropdown-menu.less";
import "@/assets/theme/redesign.css";
import "@/components/css/chat-hljs-dark.less";
import i18n from "./i18n";
import { installTDesignIconOfflineGuard } from "@/utils/tdesign-icon-offline";
import { useAuthStore } from '@/stores/auth'
import { setUnauthorizedHandler } from '@/utils/request'
import '@/assets/auth.css'

// 必须在 Vue 组件挂载之前执行，避免 tdesign-icons 运行时请求 tdesign.gtimg.com
installTDesignIconOfflineGuard();

document.documentElement.setAttribute("theme-mode", "light");

const app = createApp(App);
const pinia = createPinia();

app.use(TDesign);
app.use(pinia);
installAuthGuard(pinia);
const auth = useAuthStore(pinia);
setUnauthorizedHandler(() => {
  auth.clearAuth();
  if (!router.currentRoute.value.meta.public) {
    void router.replace({ path: '/login', query: { redirect: router.currentRoute.value.fullPath } });
  }
});
app.use(router);
app.use(i18n);

// 等首屏路由（含导航守卫、Lite 自动登录）完成后再挂载，避免先闪默认页再跳转
router.isReady().finally(() => {
  app.mount("#app");
});
