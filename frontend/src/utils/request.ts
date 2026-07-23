// src/utils/request.js
import axios from "axios";
import { generateRandomString, MAX_FILE_SIZE_MB } from "./index";
import i18n from '@/i18n'
import { getApiBaseUrl } from './api-base';

const t = (key: string) => i18n.global.t(key)

// API基础URL
const BASE_URL = getApiBaseUrl();

let csrfToken = ''
let unauthorizedHandler: (() => void) | null = null

export const setCSRFToken = (token: string) => {
  csrfToken = token
}

export const getCSRFToken = () => csrfToken

export const setUnauthorizedHandler = (handler: (() => void) | null) => {
  unauthorizedHandler = handler
}


// 创建Axios实例
const instance = axios.create({
  baseURL: BASE_URL, // 使用配置的API基础URL
	withCredentials: true,
  timeout: 30000, // 请求超时时间
  headers: {
    "Content-Type": "application/json",
    "X-Request-ID": `${generateRandomString(12)}`,
  },
});

// 获取当前用户语言（用于 Accept-Language header）
function getCurrentLanguage(): string {
  return i18n.global.locale?.value || localStorage.getItem('locale') || 'zh-CN'
}


instance.interceptors.request.use(
  (config) => {
    // 添加用户语言偏好
    config.headers["Accept-Language"] = getCurrentLanguage();
    
    config.headers["X-Request-ID"] = `${generateRandomString(12)}`;
		const method = String(config.method || 'get').toLowerCase();
		if (csrfToken && !['get', 'head', 'options'].includes(method)) config.headers["X-CSRF-Token"] = csrfToken;
    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

instance.interceptors.response.use(
  (response) => {
    // 根据业务状态码处理逻辑
    const { status, data } = response;
    if (status >= 200 && status < 300) {
      return data;
    } else {
      return Promise.reject(data);
    }
  },
  async (error: any) => {
    if (!error.response) {
      return Promise.reject({ message: t('error.networkError') });
    }
    
    // 处理 Nginx 413 Request Entity Too Large
    if (error.response.status === 413) {
      return Promise.reject({ 
        status: 413, 
        message: i18n.global.t('error.fileSizeExceeded', { size: MAX_FILE_SIZE_MB }),
        success: false
      });
    }

    const { status, data } = error.response;
		const requestURL = String(error.config?.url || '')
		if (status === 401 && !requestURL.includes('/auth/')) unauthorizedHandler?.()
    // 将HTTP状态码一并抛出，方便上层判断401等场景
    // 后端返回格式: { success: false, error: { code, message, details } }
    // 提取 error.message 作为顶层 message，方便前端使用 error?.message 获取
    let errorMessage: string | undefined;
    if (typeof data === 'object') {
      if (typeof data?.error === 'string') {
        errorMessage = data.error;
      } else if (data?.error?.message) {
        errorMessage = data.error.message;
      } else {
        errorMessage = data?.message;
      }
    } else if (typeof data === 'string') {
      errorMessage = data;
    }
    return Promise.reject({ 
      status, 
      message: errorMessage,
      ...(typeof data === 'object' ? data : {}) 
    });
  }
);

export function get<T = any>(url: string, config?: any): Promise<T> {
  return instance.get(url, config) as unknown as Promise<T>;
}

export function getDown(url: string): Promise<Blob> {
  return instance.get(url, {
    responseType: "blob",
  }) as unknown as Promise<Blob>;
}

export function postUpload<T = any>(url: string, data = {}, onUploadProgress?: (progressEvent: any) => void): Promise<T> {
  return instance.post(url, data, {
    headers: {
      "Content-Type": "multipart/form-data",
      "X-Request-ID": `${generateRandomString(12)}`,
    },
    onUploadProgress,
  }) as unknown as Promise<T>;
}

export function postChat<T = any>(url: string, data = {}): Promise<T> {
  return instance.post(url, data, {
    headers: {
      "Content-Type": "text/event-stream;charset=utf-8",
      "X-Request-ID": `${generateRandomString(12)}`,
    },
  }) as unknown as Promise<T>;
}

export function post<T = any>(url: string, data = {}, config?: any): Promise<T> {
  return instance.post(url, data, config) as unknown as Promise<T>;
}

export function put<T = any>(url: string, data = {}): Promise<T> {
  return instance.put(url, data) as unknown as Promise<T>;
}

export function del<T = any>(url: string, data?: any): Promise<T> {
  return instance.delete(url, { data }) as unknown as Promise<T>;
}
