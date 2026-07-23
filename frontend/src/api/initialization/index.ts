import { get, post, put } from '../../utils/request';
import i18n from '@/i18n'

const t = (key: string) => i18n.global.t(key)

// 初始化配置数据类型
export interface InitializationConfig {
    llm: {
        source: string;
        modelName: string;
        baseUrl?: string;
        apiKey?: string;
    };
    embedding: {
        source: string;
        modelName: string;
        baseUrl?: string;
        apiKey?: string;
        dimension?: number; // 添加embedding维度字段
    };
    rerank: {
        modelName: string;
        baseUrl: string;
        apiKey?: string;
        enabled: boolean;
    };
    multimodal: {
        enabled: boolean;
        vlm?: {
            modelName: string;
            baseUrl: string;
            apiKey?: string;
            interfaceType?: string; // "ollama" or "openai"
        };
    };
    documentSplitting: {
        chunkSize: number;
        chunkOverlap: number;
        separators: string[];
        // Adaptive chunking strategy. Empty / "legacy" = classic recursive splitter.
        // "auto" lets the backend profiler pick a tier; "heading" / "heuristic"
        // pin the tier explicitly. See backend chunker package for details.
        strategy?: string;
        // Cap chunk size in approx tokens. 0 = char-based budget only.
        tokenLimit?: number;
        // Language hints for heuristic patterns ("de", "en", "zh"). Empty = auto-detect.
        languages?: string[];
    };
}

// 下载任务状态类型
export interface DownloadTask {
    id: string;
    modelName: string;
    status: 'pending' | 'downloading' | 'completed' | 'failed';
    progress: number;
    message: string;
    startTime: string;
    endTime?: string;
}

// 简化版知识库配置更新接口（只传模型ID）
export interface KBModelConfigRequest {
    documentSplitting: {
        chunkSize: number
        chunkOverlap: number
        separators: string[]
        parserEngineRules?: { file_types: string[]; engine: string }[]
        enableParentChild?: boolean
        parentChunkSize?: number
        childChunkSize?: number
        // Adaptive chunking strategy ("auto" | "heading" | "heuristic" | "legacy").
        // The backend uses pointer-based DTOs for these three fields:
        // - undefined / not set in payload → no change on server
        // - "" / 0 / [] explicitly sent     → clears the value
        // Send the field whenever the user has opened the editor — even
        // empty values — so the user can always reset back to defaults.
        strategy?: string
        // Approximate token budget per chunk; 0 = char-based.
        tokenLimit?: number
        // Language hints for heuristic patterns. Empty array = auto-detect.
        languages?: string[]
    }
    questionGeneration?: {
        enabled: boolean
        questionCount: number
    }
}

export function updateKBConfig(kbId: string, config: KBModelConfigRequest): Promise<any> {
    return new Promise((resolve, reject) => {
        console.log('Starting KB config update (simplified)...', kbId, config);
        put(`/api/v1/initialization/config/${kbId}`, config)
            .then((response: any) => {
                console.log('KB config update completed', response);
                resolve(response);
            })
            .catch((error: any) => {
                console.error('Failed to update KB config:', error);
                reject(error.error || error);
            });
    });
}

// 检查Ollama服务状态
export function checkOllamaStatus(): Promise<{ available: boolean; version?: string; error?: string; baseUrl?: string }> {
    return new Promise((resolve, reject) => {
        get('/api/v1/initialization/ollama/status')
            .then((response: any) => {
                resolve(response.data || { available: false });
            })
            .catch((error: any) => {
                console.error('Failed to check Ollama status:', error);
                resolve({ available: false, error: error.message || t('error.initialization.checkFailed') });
            });
    });
}

// Ollama 模型详细信息接口
export interface OllamaModelInfo {
    name: string;
    size: number;
    digest: string;
    modified_at: string;
}

// 列出已安装的 Ollama 模型（详细信息）
export function listOllamaModels(): Promise<OllamaModelInfo[]> {
    return new Promise((resolve, reject) => {
        get('/api/v1/initialization/ollama/models')
            .then((response: any) => {
                resolve((response.data && response.data.models) || []);
            })
            .catch((error: any) => {
                console.error('Failed to list Ollama models:', error);
                resolve([]);
            });
    });
}

// 检查Ollama模型状态
export function checkOllamaModels(models: string[]): Promise<{ models: Record<string, boolean> }> {
    return new Promise((resolve, reject) => {
        post('/api/v1/initialization/ollama/models/check', { models })
            .then((response: any) => {
                resolve(response.data || { models: {} });
            })
            .catch((error: any) => {
                console.error('Failed to check Ollama models:', error);
                reject(error);
            });
    });
}

// 启动Ollama模型下载（异步）
export function downloadOllamaModel(modelName: string): Promise<{ taskId: string; modelName: string; status: string; progress: number }> {
    return new Promise((resolve, reject) => {
        post('/api/v1/initialization/ollama/models/download', { modelName })
            .then((response: any) => {
                resolve(response.data || { taskId: '', modelName, status: 'failed', progress: 0 });
            })
            .catch((error: any) => {
                console.error('Failed to start Ollama model download:', error);
                reject(error);
            });
    });
}

// 查询下载进度
export function getDownloadProgress(taskId: string): Promise<DownloadTask> {
    return new Promise((resolve, reject) => {
        get(`/api/v1/initialization/ollama/download/progress/${taskId}`)
            .then((response: any) => {
                resolve(response.data);
            })
            .catch((error: any) => {
                console.error('Failed to get download progress:', error);
                reject(error);
            });
    });
}

// 所有"测试连接"接口共用的通用可选参数。
// extraConfig / interfaceType 对应后端 ModelTestRequest 里的同名字段，
// 会被透传给真正的模型装配流程，保证测试连接与生产调用走完全相同的路径。
interface BaseModelTestPayload {
    extraConfig?: Record<string, string>;
    interfaceType?: string;
    /** 共享 API 配置 ID；显式传空字符串表示测试模型自己的私有凭证。 */
    apiConfigId?: string;
    /** 第二段密钥（如 LKEAP Rerank 的腾讯云 SecretKey） */
    appSecret?: string;
}

// 检查远程API模型
export function checkRemoteModel(modelConfig: {
    modelName: string;
    baseUrl: string;
    apiKey?: string;
    provider?: string;
    // 编辑已存在模型时传 modelId，后端会自动从存储中带出 apiKey
    // （前端不再回显明文密钥，所以测试连接必须用这个回填路径）
    modelId?: string;
} & BaseModelTestPayload): Promise<{
    available: boolean;
    message?: string;
}> {
    return new Promise((resolve, reject) => {
        post('/api/v1/initialization/remote/check', modelConfig)
            .then((response: any) => {
                resolve(response.data || {});
            })
            .catch((error: any) => {
                console.error('Failed to check remote model:', error);
                reject(error);
            });
    });
}

// 测试 Embedding 模型（本地/远程）是否可用
export function testEmbeddingModel(modelConfig: {
    source: 'local' | 'remote';
    modelName: string;
    baseUrl?: string;
    apiKey?: string;
    dimension?: number;
    supportsDimensionOverride?: boolean;
    provider?: string;
    modelId?: string;
} & BaseModelTestPayload): Promise<{ available: boolean; message?: string; dimension?: number }> {
    return new Promise((resolve, reject) => {
        post('/api/v1/initialization/embedding/test', modelConfig)
            .then((response: any) => {
                resolve(response.data || {});
            })
            .catch((error: any) => {
                console.error('Failed to test Embedding model:', error);
                reject(error);
            });
    });
}


export function checkRerankModel(modelConfig: {
    modelName: string;
    baseUrl: string;
    apiKey?: string;
    provider?: string;
    modelId?: string;
} & BaseModelTestPayload): Promise<{
    available: boolean;
    message?: string;
}> {
    return new Promise((resolve, reject) => {
        post('/api/v1/initialization/rerank/check', modelConfig)
            .then((response: any) => {
                resolve(response.data || {});
            })
            .catch((error: any) => {
                console.error('Failed to check Rerank model:', error);
                reject(error);
            });
    });
}

// 模型厂商信息类型
export interface ModelProviderOption {
    value: string;        // provider 标识符
    label: string;        // 显示名称
    description: string;  // 描述
    defaultUrls: Record<string, string>;  // 按模型类型区分的默认 URL
    modelTypes: string[]; // 支持的模型类型
}

// 获取模型厂商列表
export function listModelProviders(modelType?: string): Promise<ModelProviderOption[]> {
    return new Promise((resolve, reject) => {
        const url = modelType
            ? `/api/v1/models/providers?model_type=${encodeURIComponent(modelType)}`
            : '/api/v1/models/providers';
        get(url)
            .then((response: any) => {
                resolve(response.data || []);
            })
            .catch((error: any) => {
                console.error('Failed to list model providers:', error);
                resolve([]); // 失败时返回空数组，前端可以回退到默认值
            });
    });
}
