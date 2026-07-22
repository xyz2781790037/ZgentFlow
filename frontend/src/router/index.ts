import { createRouter, createWebHistory } from 'vue-router'
import type { Pinia } from 'pinia'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
		{
			path: '/login',
			name: 'login',
			component: () => import('../views/auth/Login.vue'),
			meta: { public: true },
		},
		{
			path: '/register',
			name: 'register',
			component: () => import('../views/auth/Register.vue'),
			meta: { public: true },
		},
    {
      path: "/",
      redirect: "/platform/creatChat",
    },
    {
      path: "/platform",
      name: "Platform",
      redirect: "/platform/creatChat",
      component: () => import("../views/platform/index.vue"),
      children: [
        {
          path: "settings",
          name: "settings",
          component: () => import("../views/settings/Settings.vue"),
        },
        {
          path: "knowledge-bases",
          name: "knowledgeBaseList",
          component: () => import("../views/knowledge/ZealLibrary.vue"),
        },
        {
          path: "knowledge-bases/:kbId",
          name: "knowledgeBaseDetail",
          component: () => import("../views/knowledge/KnowledgeBase.vue"),
        },
        {
          path: "recycle-bin",
          name: "recycleBin",
          component: () => import("../views/knowledge/RecycleBin.vue"),
        },
        {
          path: "knowledge-search",
          // 旧路径保留为重定向，打开全局命令面板（⌘K），带上可选的 q 参数
          redirect: (to) => {
            const q = to.query.q
            return {
              path: '/platform/knowledge-bases',
              query: typeof q === 'string' ? { cmdk: q } : { cmdk: '' },
            }
          },
        },
        {
          path: "creatChat",
          name: "globalCreatChat",
          component: () => import("../views/creatChat/creatChat.vue"),
        },
        {
          path: "knowledge-bases/:kbId/creatChat",
          name: "kbCreatChat",
          component: () => import("../views/creatChat/creatChat.vue"),
        },
        {
          path: "chat/:chatid",
          name: "chat",
          component: () => import("../views/chat/index.vue"),
        },
      ],
    },
    {
      path: '/:pathMatch(.*)*',
      redirect: '/platform/creatChat',
    },
  ],
});

export const installAuthGuard = (pinia: Pinia) => {
	router.beforeEach(async (to) => {
		const auth = useAuthStore(pinia)
		const authenticated = await auth.ensureInitialized()
		if (to.meta.public) {
			if (!authenticated) return true
			const redirect = typeof to.query.redirect === 'string' ? to.query.redirect : ''
			return redirect.startsWith('/') && !redirect.startsWith('//') ? redirect : '/platform/creatChat'
		}
		if (authenticated) return true
		return { path: '/login', query: { redirect: to.fullPath } }
	})
}

export default router
