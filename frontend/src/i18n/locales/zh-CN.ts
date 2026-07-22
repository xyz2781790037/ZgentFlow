export default {
    menu: {
        knowledgeBase: "知识库",
        newChat: "新对话",
        settings: "系统设置",
        batchManage: "批量管理"
    },
    knowledgeBase: {
        fileContent: "文件内容",
        settings: "设置",
        uploadSuccess: "文件上传成功！",
        uploadFailed: "文件上传失败！",
        fileExists: "文件已存在",
        uploadAllSuccess: "成功上传 {count} 个文件！",
        uploadPartialSuccess: "上传完成：成功 {success} 个，失败 {fail} 个",
        uploadAllFailed: "所有文件上传失败",
        uploadingFolder: "正在上传文件夹中的 {total} 个文件...",
        unsupportedVideos: "已跳过 {count} 个视频文件（暂不支持视频上传）",
        unsupportedTypesHint: "部分文档类型（{types}）暂无可用解析引擎，上传后将无法解析",
        goToParserSettings: "前往配置",
        addDocument: "添加文档",
        typeFile: "文件",
        createdAt: "创建",
        updatedAt: "更新",
        clickToViewFull: "点击卡片查看全文与分段",
        characters: "字符",
        segment: "片段",
        chunkCount: "共 {count} 个片段",
        viewChunks: "查看分块",
        viewMerged: "全文",
        questions: "问题",
        generatedQuestions: "生成的问题",
        childChunk: "子块",
        viewParentContext: "查看父块上下文",
        parentContextLoadFailed: "加载父上下文失败",
        confirmDeleteQuestion: "确定要删除这个问题吗？删除后将同时移除对应的向量索引。",
        legacyQuestionCannotDelete: "旧格式问题无法删除，请重新生成问题",
        notInitialized: "该知识库尚未完成初始化配置，请先前往设置页面配置模型信息后再上传文件",
        getInfoFailed: "获取知识库信息失败，无法上传文件",
        missingId: "缺少知识库ID",
        deleteFailed: "删除失败，请稍后再试！",
        uploadTime: "上传时间",
        newSession: "新会话",
        rebuildDocument: "重建知识",
        rebuildConfirm: '确认重建文档"{fileName}"？该操作会清理现有分块并重新解析。',
        rebuildSubmitted: "重建任务已提交",
        fullRebuild: "全量重建",
        fullRebuildTip: "重建索引并复用可用缓存",
        fullRebuildConfirm: "将按当前配置重建索引并复用仍兼容的缓存。确认开始吗？",
        fullRebuildSubmitted: "全量重建任务已提交",
        fullRebuildRunning: "全量重建进行中",
        fullRebuildSucceeded: "全量重建完成",
        fullRebuildFailedReason: "全量重建失败：{reason}",
        createSessionFailed: "创建会话失败",
        createSessionError: "创建会话失败，请稍后重试",
        rebuildFailed: "重建失败，请稍后再试",
        rebuildInProgress: "当前文档正在解析中，请稍后重试",
        cancelParse: "停止解析",
        cancelParseConfirmBody: '确认停止解析"{title}"？已写入的分块会保留，可稍后通过"重建"重新触发；优化阶段（摘要 / 问答）的待执行任务会被立即丢弃。',
        cancelParseSubmitted: "已停止解析",
        cancelParseFailed: "停止失败，请稍后再试",
        draft: "草稿",
        draftTip: "暂存内容，未参与检索",
        untitledDocument: "未命名文档",
        deleteDocument: "删除文档",
        moveDocument: "移动到...",
        moveToKnowledgeBase: "移动到知识库",
        moveNoTargets: "没有兼容的目标知识库（需要相同类型和 Embedding 模型）",
        moveModeReuseVectors: "复用向量（快速）",
        moveModeReuseVectorsDesc: "直接移动分块和向量索引，适用于分片配置相同的情况",
        moveModeReparse: "重新解析",
        moveModeReparseDesc: "使用目标知识库的分片配置重新解析文档",
        moveConfirm: "确认移动",
        moveConfirmTitle: "确认移动设置",
        moveStarted: "移动任务已提交",
        moveFailed: "移动失败",
        moveCompleted: "移动完成",
        moveCompletedWithErrors: "移动完成：{success} 成功，{failed} 失败",
        generatingSummary: "生成摘要中...",
        documentSummary: "摘要",
        confirmDeleteDocument: '确认删除文档"{fileName}"，删除后将无法恢复',
        confirmDelete: "确认删除",
        viewModeGrid: "卡片视图",
        viewModeList: "列表视图",
        viewModeToggle: "切换视图",
        columnName: "文件名",
        columnSize: "大小",
        columnStatus: "状态",
        columnUpdatedAt: "更新时间",
        columnActions: "操作",
        selectAll: "全选",
        selectedCount: "已选 {count} 项",
        clearSelection: "取消选择",
        batchDelete: "批量删除",
        confirmBatchDeleteDocument: "确认删除选中的 {count} 个文档？删除后将无法恢复。",
        batchDeleteSuccess: "成功删除 {count} 个文档",
        batchDeleteFailed: "批量删除失败",
        statusDraft: "草稿",
        pdfDocFormat: "pdf、doc 格式文件，不超过10M",
        textMarkdownFormat: "text、markdown格式文件，不超过200K",
        dragFileNotText: "请拖拽文件而不是文本或链接",
        searchPlaceholder: "搜索知识库...",
        docSearchPlaceholder: "搜索文档名称...",
        fileTypeFilter: "文件类型",
        allFileTypes: "全部类型",
        parseStatusFilter: "解析状态",
        allParseStatuses: "全部状态",
        parseStatusPending: "等待中",
        parseStatusProcessing: "处理中",
        parseStatusCompleted: "已完成",
        parseStatusFailed: "失败",
        updatedTimeFrom: "起始时间",
        updatedTimeTo: "结束时间",
        noMatch: "未找到匹配的知识库",
        noKnowledge: "暂无可用知识库",
        selectKnowledgeBase: "请选择知识库",
        loadingFailed: "加载知识库失败",
        operationNotSupportedForType: "当前知识库类型不支持该操作",
        allFilesSkippedNoEngine: "所选文件类型暂无可用解析引擎，已全部跳过",
        filesSkippedNoEngine: "{count} 个文件因无可用解析引擎被跳过",
        allUploadSuccess: "所有文件上传成功（{count}个）",
        partialUploadSuccess: "部分文件上传成功（成功：{success}，失败：{fail}）",
        allUploadFailed: "所有文件上传失败（{count}个）",
        deleteSuccess: "知识删除成功！",
        chunkLoadFailed: "分块加载失败"
    },
    uploadConfirm: {
        title: "上传文档确认",
        fileList: "待上传文件",
        overviewDesc: "点击任一项可修改配置",
        backToOverview: "返回配置总览",
        summaryChunkOverlapShort: "重叠 {overlap}",
        summaryParentChildShort: "父子分块",
        summaryParserBuiltin: "内置引擎（默认）",
        summaryStrategyDefault: "默认",
        summaryParentChildOff: "未开启",
        summaryQuestionCountValue: "{count} 个",
        navChunkingSummary: "分块 {size}",
        statusOn: "已开启",
        statusOff: "未开启",
        notSet: "未设置",
        confirm: "确认上传并解析",
        cancel: "取消",
        tabParser: "解析引擎",
        tabChunking: "分块",
        tabMultimodal: "多模态",
        tabQuestion: "问题生成",
        noItems: "请至少添加一个文件或 URL",
        vlmModelRequired: "本批次包含图片，请启用多模态并选择 VLM 模型",
        multimodalRequiredForImages: "未开启（本批次含图片）",
        vlmModelSelectRequired: "已启用多模态，请选择 VLM 模型",
        continueAdd: "继续添加",
        filesAdded: "已添加 {count} 个文件",
        filesAllDuplicate: "所选文件已在列表中",
        titleReparse: "重新解析确认",
        overviewDescReparse: "确认解析配置后将清除现有内容并重新解析该文档",
        confirmReparse: "确认并重新解析",
        reparseSource: "待重新解析文档",
        reparseHint: "将沿用上次解析的配置，可在此调整"
    },
    knowledgeStages: {
        title: "处理流水线",
        root: "知识处理",
        processConfig: {
            title: "本次解析配置",
            kbDefault: "使用知识库默认配置",
            chunking: "分块",
            chunkSize: "大小 {n}",
            parentChildOn: "父子分块开",
            parentChildOff: "父子分块关",
            parser: "解析引擎",
            multimodal: "多模态",
            question: "问题生成",
            questionOn: "开（{n} 个/块）",
            on: "开",
            off: "关"
        },
        attempt: "第 {n} 次尝试",
        attemptLatest: "第 {n} 次尝试（最新）",
        retry: "重新解析",
        refresh: "立即刷新",
        copy: "复制",
        copyDetails: "复制详情",
        copied: "已复制到剪贴板",
        close: "关闭",
        live: "LIVE",
        liveTooltip: "解析进行中，每 2 秒自动刷新一次",
        autoRefreshOn: "自动刷新中",
        autoRefreshOff: "已停止自动刷新",
        fetchFailed: "最近 {n} 次刷新都失败了，数据可能已过期，点击刷新按钮重试",
        fetchFailedShort: "刷新失败",
        viewTrace: "查看 Trace",
        traceBtn: "Trace",
        expandBranch: "展开子项",
        collapseBranch: "收起子项",
        rowSelectHint: "点击查看详情；左侧箭头展开或收起子项",
        resizeDrawer: "拖拽调整面板宽度",
        justNow: "刚刚",
        secondsAgo: "{n} 秒前",
        minutesAgo: "{n} 分钟前",
        noActivity: "暂无解析记录",
        totalDuration: "总耗时：{d}",
        total: "总耗时 {d}",
        head: {
            duration: "总耗时",
            stages: "阶段",
            stagesDone: "已完成阶段",
            stagesProgress: "当前阶段",
            stage: "阶段",
            status: "状态",
            attempt: "尝试",
            updated: "更新于"
        },
        tab: {
            overview: "概览",
            raw: "原始 JSON"
        },
        detail: {
            started: "开始",
            finished: "结束",
            duration: "耗时",
            offset: "起点偏移",
            timing: "时序",
            identity: "身份信息",
            stageBreakdown: "阶段分解",
            stageOrder: "阶段顺序",
            childCount: "子 span 数",
            kind: "类型",
            status: "状态",
            name: "名称",
            input: "输入",
            output: "输出",
            metadata: "元数据",
            traceMetadata: "Trace 级元数据",
            metadataHint: "记录与 Langfuse 等可观测性系统关联的辅助字段（如 trace ID）。各阶段的业务入参/出参在「输入」「输出」标签中查看。",
            metadataEmpty: "当前 span 没有元数据。阶段与子任务的入参/出参请查看「输入」「输出」；若已接入 Langfuse，trace 级字段会显示在「概览」。",
            error: "错误",
            empty: "暂无数据",
            inProgress: "进行中",
            elapsed: "已用",
            placeholderHint: "此阶段未记录详细 span，仅展示推断状态。",
            showJson: "展开 JSON",
            hideJson: "收起 JSON",
            includingChildren: "含子任务"
        },
        stage: {
            docreader: "文档解析",
            chunking: "分块",
            embedding: "向量化",
            multimodal: "多模态识别",
            postprocess: "后处理"
        },
        status: {
            pending: "等待中",
            running: "进行中",
            finalizing: "优化中",
            done: "已完成",
            failed: "失败",
            skipped: "已跳过",
            cancelled: "已取消"
        },
        errorCode: {
            DOCREADER_TIMEOUT: "文档解析超时",
            DOCREADER_TIMEOUT_SUGGESTION: "文件可能过大或解析服务繁忙，请稍后重试或拆分文档。",
            DOCREADER_UNAVAILABLE: "文档解析服务不可用",
            DOCREADER_UNAVAILABLE_SUGGESTION: "解析服务离线，请联系管理员。",
            DOCREADER_PARSE_FAILED: "文档解析失败",
            DOCREADER_PARSE_FAILED_SUGGESTION: "无法解析该文件，请确认文件未损坏。",
            CHUNKING_FAILED: "分块失败",
            CHUNKING_FAILED_SUGGESTION: "请尝试调整知识库的分块配置。",
            EMBEDDING_RATE_LIMIT: "向量服务被限流",
            EMBEDDING_RATE_LIMIT_SUGGESTION: "向量服务触发限流，请稍后重试。",
            EMBEDDING_PROVIDER_FAIL: "向量服务错误",
            EMBEDDING_PROVIDER_FAIL_SUGGESTION: "向量服务返回错误，请检查供应商配置。",
            VECTORSTORE_WRITE_FAILED: "向量库写入失败",
            VECTORSTORE_WRITE_FAILED_SUGGESTION: "向量库拒绝写入，请检查向量库可用性。",
            MULTIMODAL_VLM_FAILED: "图像理解失败",
            MULTIMODAL_VLM_FAILED_SUGGESTION: "部分图像无法处理，文档可能仍可使用。",
            MULTIMODAL_ALL_FAILED: "全部图像多模态处理失败",
            MULTIMODAL_ALL_FAILED_SUGGESTION: "请检查多模态模型配置。",
            TASK_TIMEOUT: "任务超过最大运行时间",
            TASK_TIMEOUT_SUGGESTION: "任务运行时间超出限制，请重试或联系支持人员。",
            SERVER_RESTART: "服务重启导致任务中断",
            SERVER_RESTART_SUGGESTION: "任务队列会继续重试；若仍失败，请点击重试。",
            UPSTREAM_FAILED: "上游阶段失败已中止",
            UPSTREAM_FAILED_SUGGESTION: "前置阶段失败，导致本步骤无法执行。",
            UNKNOWN: "未知错误",
            UNKNOWN_SUGGESTION: "请查看应用日志获取详细信息。"
        }
    },
    chat: {
        thinking: "思考中...",
        emptyContentWarning: "回答内容为空",
        copySuccess: "已复制",
        copyFailed: "复制失败",
        deepThoughtCompleted: "已深度思考",
        deepThoughtAlt: "深度思考完成",
        referencesTitle: "参考了{count}个相关内容",
        referencesDocCount: "引用了{count}篇文档",
        referencesDocAndWebCount: "引用了{docCount}篇文档和{webCount}条网页",
        referenceChunkCount: "{count}个片段",
        fallbackHint: "未从知识库中检索到相关内容，以上为模型直接回答",
        requestInfoTitle: "请求信息",
        requestInfoRequestId: "Request ID",
        requestInfoMessageId: "消息 ID",
        requestInfoSessionId: "会话 ID",
        requestInfoUrl: "请求",
        requestInfoSentAt: "发起时间",
        requestInfoEmpty: "暂无请求信息",
        chunkLabel: "片段{index}:",
        navigateToDocument: "查看文档详情",
        chunkIdLabel: "片段ID:",
        documentIdLabel: "文档ID:",
        faqIdLabel: "FAQ ID:",
        faqContainerIdLabel: "所属文档ID:",
        faqAnswersLabel: "答案:",
        chunkOrdinal: "片段 {index}",
        previewContent: "预览内容",
        noPlanSteps: "未提供具体步骤",
        chunkIndexLabel: "片段 #{index}",
        chunkPositionLabel: "(位置: {position})",
        noRelatedChunks: "没有找到相关片段",
        noSearchResults: "没有找到搜索结果",
        relevanceHigh: "高相关",
        relevanceMedium: "中相关",
        relevanceLow: "低相关",
        relevanceWeak: "弱相关",
        webSearchNoResults: "未找到搜索结果",
        otherSource: "其他来源",
        webGroupIntro: "以下 {count} 条内容来自",
        unknownLink: "未知链接",
        contentLengthLabel: "长度 {value}",
        notProvided: "未提供",
        promptLabel: "提示词",
        errorMessageLabel: "错误信息",
        summaryLabel: "总结",
        rawTextLabel: "原始文本",
        collapseRaw: "收起原文",
        expandRaw: "展开原文",
        noWebContent: "未获取到网页内容",
        lengthChars: "{value} 字",
        lengthThousands: "{value} 千字",
        lengthTenThousands: "{value} 万字",
        chunkCountValue: "{count} 个片段",
        documentDescriptionLabel: "文档描述:",
        documentSourceLabel: "来源:",
        documentFileLabel: "文件信息:",
        documentMetadataLabel: "元数据",
        documentInfoEmpty: "暂无文档信息",
        positionLabel: "位置:",
        chunkPositionValue: "第 {index} 个片段",
        contentLengthLabelSimple: "内容长度:",
        fullContentLabel: "完整内容",
        copyContent: "复制内容",
        knowledgeBaseCount: "共 {count} 个知识库",
        noKnowledgeBases: "没有可用的知识库",
        rawOutputLabel: "原始输出",
        processError: "处理出错",
        sessionExcerpt: "会话摘录",
        noAnswerContent: "（无回答内容）",
        noMatchFound: "未找到匹配的内容",
        imageTooMany: "最多上传5张图片",
        imageTypeSizeError: "仅支持 JPG/PNG/GIF/WEBP 格式，单张不超过 10MB",
        imageUploadTooltip: "上传图片（支持粘贴/拖拽）",
        attachmentUploadTooltip: "上传附件（文档、音频等）",
        attachmentWithCount: "已上传 {count} 个附件",
        attachmentTooMany: "最多上传 {max} 个附件",
        attachmentTooLarge: "文件 {name} 超过 {max}MB 限制",
        attachmentTypeNotSupported: "不支持的文件类型：{name}"
    },
    settings: {
        parserEngine: "解析引擎"
    },
    webSearchSettings: {
        title: "网络搜索配置",
        description: "配置网络搜索功能，在回答问题时可以从互联网获取实时信息补充知识库内容",
        // Section keys for the redesigned drawer
        basicSection: "基本信息",
        credentialsSection: "连接配置",
        optionsSection: "选项",
        // Provider entity management
        providersTitle: "搜索引擎配置",
        addProvider: "添加搜索引擎",
        editProvider: "编辑搜索引擎",
        deleteConfirm: "确定要删除此搜索引擎配置吗？",
        providerNameLabel: "名称",
        providerNamePlaceholder: "例如：生产环境 Bing 搜索",
        providerTypeLabel: "引擎类型",
        providerDescLabel: "备注",
        providerDescPlaceholder: "可选，如：测试环境用",
        engineIdLabel: "搜索引擎 ID",
        setAsDefault: "设为默认",
        testConnection: "测试连接",
        testing: "测试中...",
        viewDocs: "查看文档获取密钥",
        setAsDefaultDesc: "当智能体没有指定特定的搜索引擎时，将默认使用此配置",
        proxyUrlLabel: "HTTP 代理",
        proxyUrlPlaceholder: "例如 http://proxy.example.com:3128（可选，仅支持 http/https）",
        proxyUrlHelp: "若环境访问搜索 API 需代理，在此填写；留空则使用系统环境变量 HTTP(S)_PROXY。",
        apiKeyLabel: "API 密钥",
        baseUrlLabel: "实例地址",
        baseUrlPlaceholder: "https://searxng.example.com",
        apiKeyPlaceholder: "请输入 API 密钥",
        toasts: {
            providerCreated: "搜索引擎配置已创建",
            providerUpdated: "搜索引擎配置已更新",
            providerDeleted: "搜索引擎配置已删除",
            testSuccess: "连接测试成功",
            testFailed: "连接测试失败"
        }
    },
    retrievalSettings: {
        title: "搜索设置",
        description: "配置知识库搜索和消息搜索的全局检索参数",
        embeddingTopKLabel: "向量检索数量 (Top K)",
        vectorThresholdLabel: "向量相似度阈值",
        keywordThresholdLabel: "关键词匹配阈值",
        rerankTopKLabel: "Rerank 数量 (Top K)",
        rerankThresholdLabel: "Rerank 阈值",
        rerankModelLabel: "Rerank 模型",
        rerankModelDescription: "选择用于搜索结果重排序的模型",
        rerankModelRequired: "请选择 Rerank 模型，搜索功能需要此模型对结果进行重排序",
        toasts: {
            saveSuccess: "检索配置已保存",
            saveFailed: "保存配置失败: {message}"
        }
    },
    common: {
        confirm: "确认",
        cancel: "取消",
        save: "保存",
        delete: "删除",
        edit: "编辑",
        copy: "复制",
        copied: "已复制",
        download: "下载",
        loading: "加载中...",
        noData: "暂无数据",
        noMoreData: "已加载全部内容",
        error: "错误",
        success: "成功",
        failed: "失败",
        close: "关闭",
        finish: "完成",
        all: "全部",
        clear: "清空",
        confirmDelete: "确认删除",
        deleteSuccess: "删除成功",
        deleteFailed: "删除失败",
        file: "文件",
        knowledgeBase: "知识库",
        noResult: "无结果",
        remove: "移除",
        copyFailed: "复制失败",
        unknownError: "未知错误",
        operationFailed: "操作失败",
        retry: "重试"
    },
    mentionDetail: {
        faqCount: "共 {count} 条问答",
        kbCount: "共 {count} 个文档",
        belongsToKb: "所属知识库：",
        noCompatibleKbForAgent: "当前智能体的工具与作用域内知识库的能力不匹配，暂无可引用的知识库。"
    },
    agent: {
        taskLabel: "任务:",
        think: "思考",
        copy: "复制",
        updatePlan: "更新计划",
        webSearchFound: "找到 <strong>{count}</strong> 个网络搜索结果",
        toolFallback: "工具",
        stepsCompleted: "已完成 <strong>{steps}</strong> 个步骤",
        reasoningRounds: "思考 <strong>{rounds}</strong> 轮",
        toolCalls: "调用 <strong>{tools}</strong> 次工具",
        durationSuffix: "耗时 <strong>{duration}</strong>",
        stepSummarySeparator: " · "
    },
    file: {
        upload: "上传文件",
        downloadFailed: "文件下载失败"
    },
    error: {
        invalidImageLink: "无效的图片链接",
        networkError: "网络错误，请检查您的网络连接",
        fileSizeExceeded: "文件大小不能超过 {size}M！",
        unsupportedFileType: "不支持的文件类型！",
        invalidFileType: "文件类型错误！",
        missingKbId: "缺少知识库ID",
        streamFailed: "流式连接失败",
        model: {
            createFailed: "创建模型失败",
            updateFailed: "更新模型失败",
            deleteFailed: "删除模型失败"
        },
        initialization: {
            checkFailed: "检查失败"
        }
    },
    model: {
        modelName: "模型名称",
        defaultTag: "默认",
        addModelInSettings: "前往全局设置添加模型",
        loadFailed: "加载模型列表失败",
        selectModelPlaceholder: "请选择模型",
        searchPlaceholder: "搜索模型...",
        editor: {
            addTitle: "添加模型",
            editTitle: "编辑模型",
            sectionType: "模型类型",
            typeLabel: "模型类型",
            sectionSource: "模型来源",
            sectionProvider: "接入配置",
            sectionAdvanced: "高级选项",
            sourceLabel: "模型来源",
            sourceLocal: "Ollama",
            sourceRemote: "API",
            description: {
                chat: "配置用于对话的大语言模型",
                embedding: "配置用于文本向量化的嵌入模型",
                rerank: "配置用于结果重排序的模型",
                vllm: "配置用于视觉理解和多模态的视觉语言模型",
                default: "配置模型信息"
            },
            modelNamePlaceholder: {
                local: "例如：llama2:latest",
                remote: "例如：gpt-4, claude-3-opus",
                localVllm: "例如：llava:latest",
                remoteVllm: "例如：gpt-4-vision-preview"
            },
            baseUrlLabel: "Base URL",
            displayNameLabel: "显示名称（可选）",
            displayNamePlaceholder: "例如：客服问答模型",
            displayNameDesc: "仅用于界面展示，实际调用仍使用上面的模型名称。",
            baseUrlPlaceholder: "例如：https://api.openai.com/v1",
            baseUrlPlaceholderVllm: "例如：http://localhost:11434/v1",
            apiKeyOptional: "API Key（可选）",
            apiKeyPlaceholder: "输入 API Key",
            lkeap: {
                secretIdLabel: "SecretId",
                secretIdPlaceholder: "腾讯云 API 密钥 SecretId",
                secretKeyLabel: "SecretKey",
                secretKeyPlaceholder: "腾讯云 API 密钥 SecretKey",
                regionLabel: "地域",
                regionPlaceholder: "ap-guangzhou",
                regionDesc: "RunRerank 支持 ap-beijing、ap-guangzhou 等，默认 ap-guangzhou",
                rerankCredentialHint: "Rerank 使用腾讯云 API 签名（非 OpenAI API Key）。请在云 API 密钥控制台创建 SecretId/SecretKey。"
            },
            connectionTest: "连接测试",
            testing: "测试中...",
            testConnection: "测试连接",
            searchPlaceholder: "搜索模型...",
            downloadLabel: "下载: {keyword}",
            refreshList: "刷新列表",
            dimensionLabel: "向量维度",
            dimensionPlaceholder: "例如：1536",
            checkDimension: "检测维度",
            dimensionDetected: "检测成功，向量维度：{value}",
            dimensionFailed: "检测失败，请手动输入维度",
            remoteDimensionDetected: "检测到向量维度：{value}",
            dimensionOverrideLabel: "自定义输出维度",
            dimensionOverrideDesc: "仅在确认该模型支持 dimensions 参数时开启；默认只使用检测到的实际维度。",
            supportsVisionLabel: "支持视觉/多模态",
            supportsVisionDesc: "模型是否支持图片等多模态输入",
            maxConcurrencyLabel: "后台并发上限",
            maxConcurrencyPlaceholder: "0 表示使用全局默认",
            maxConcurrencyDesc: "限制文档入库/富化等后台任务对该模型的并发调用数（按模型全副本共享）。0 或留空表示沿用全局默认；不影响交互式对话。",
            thinkingControlLabel: "思考模式参数格式",
            thinkingControlDesc: "决定智能体「思考模式」开/关时如何写入 API。已尝试按厂商/模型预选，若与实际情况不符请按 API 文档手动修改；选「不写入」时，智能体「思考模式」开关不生效。",
            thinkingControl: {
                none: {
                    label: "不写入思考参数",
                    hint: "智能体「思考模式」开关不生效，不会在请求中写入思考相关参数"
                },
                chatTemplateKwargs: {
                    label: "chat_template_kwargs",
                    hint: "自定义 OpenAI 兼容、NVIDIA NIM、vLLM / 本地 Qwen 部署"
                },
                enableThinking: {
                    label: "enable_thinking",
                    hint: "阿里云 DashScope：qwen3、qwen-plus、qwen-max、qwen-turbo"
                },
                thinkingType: {
                    label: "thinking.type",
                    hint: "火山引擎 Ark；腾讯云 LKEAP（DeepSeek V3 等，选 LKEAP 时默认此项；R1 请改「不写入」）"
                }
            },
            dimensionHint: '模型已选择，点击"检测维度"按钮自动获取向量维度',
            loadModelListFailed: "加载模型列表失败",
            listRefreshed: "列表已刷新",
            fillModelAndUrl: "请先填写模型标识和 Base URL",
            remoteBaseUrlRequired: "Remote API 类型必须填写 Base URL",
            unsupportedModelType: "不支持的模型类型",
            connectionSuccess: "连接成功",
            connectionFailed: "连接失败",
            connectionConfigError: "连接失败，请检查配置",
            downloadStarted: "开始下载 {name}",
            downloadCompleted: "{name} 下载完成",
            downloadFailed: "{name} 下载失败",
            downloadStartFailed: "启动下载失败",
            ollamaUnavailable: "Ollama服务不可用，无法选择本地模型",
            ollamaNotSupportRerank: "Ollama 不支持 ReRank 模型，请使用远程接口配置",
            goToOllamaSettings: "查看设置",
            validation: {
                modelNameRequired: "请输入模型名称",
                modelNameEmpty: "模型名称不能为空",
                modelNameMax: "模型名称不能超过100个字符",
                baseUrlRequired: "请输入 Base URL",
                baseUrlEmpty: "Base URL 不能为空",
                baseUrlInvalid: "Base URL 格式不正确，请输入有效的 URL"
            },
            // Provider (厂商) 相关翻译
            providerLabel: "服务商",
            providerPlaceholder: "选择模型服务商",
            providers: {
                openai: {
                    label: "OpenAI",
                    description: "gpt-5.2, gpt-5-mini, etc."
                },
                anthropic: {
                    label: "Anthropic",
                    description: "Claude models via native Anthropic Messages API"
                },
                azure_openai: {
                    label: 'Azure OpenAI',
                    description: 'Microsoft Azure 上的 OpenAI 服务'
                },
                aliyun: {
                    label: "阿里云 DashScope",
                    description: "qwen-plus, tongyi-embedding-vision-plus, qwen3-rerank, etc."
                },
                zhipu: {
                    label: "智谱 BigModel",
                    description: "glm-4.7, embedding-3, rerank, etc."
                },
                openrouter: {
                    label: "OpenRouter",
                    description: "openai/gpt-5.2-chat, google/gemini-3-flash-preview, etc."
                },
                requesty: {
                    label: "Requesty",
                    description: "openai/gpt-4o-mini, anthropic/claude-sonnet-4-5, etc."
                },
                generic: {
                    label: "自定义 (OpenAI兼容接口)",
                    description: "Generic API endpoint (OpenAI-compatible)"
                },
                siliconflow: {
                    label: "硅基流动 SiliconFlow",
                    description: "deepseek-ai/DeepSeek-V3.1, etc."
                },
                jina: {
                    label: "Jina",
                    description: "jina-clip-v1, jina-embeddings-v2-base-zh, etc."
                },
                volcengine: {
                    label: "火山引擎 Volcengine",
                    description: "doubao-1-5-pro-32k-250115, doubao-embedding-vision-250615, etc."
                },
                deepseek: {
                    label: "DeepSeek",
                    description: "deepseek-chat, deepseek-reasoner 等"
                },
                hunyuan: {
                    label: "腾讯混元 Hunyuan",
                    description: "hunyuan-pro, hunyuan-standard, hunyuan-embedding, etc."
                },
                minimax: {
                    label: "MiniMax",
                    description: "MiniMax-M3, MiniMax-M2.7, MiniMax-M2.7-highspeed 等"
                },
                mimo: {
                    label: "小米 MiMo",
                    description: "mimo-v2-flash"
                },
                gemini: {
                    label: "Google Gemini",
                    description: "gemini-3-flash-preview, gemini-2.5-pro 等"
                },
                gpustack: {
                    label: "GPUStack",
                    description: "Choose your deployed model on GPUStack"
                },
                modelscope: {
                    label: "魔搭 ModelScope",
                    description: "Qwen/Qwen3-8B, Qwen/Qwen3-Embedding-8B, etc."
                },
                qiniu: {
                    label: "七牛云 Qiniu",
                    description: "deepseek/deepseek-v3.2-251201, z-ai/glm-4.7, etc."
                },
                moonshot: {
                    label: "月之暗面 Moonshot",
                    description: "kimi-k2-turbo-preview, moonshot-v1-8k-vision-preview, etc."
                },
                qianfan: {
                    label: "百度千帆 Baidu Cloud",
                    description: "ernie-5.0-thinking-preview, embedding-v1, bce-reranker-base, etc."
                },
                longcat: {
                    label: "LongCat AI",
                    description: "LongCat-Flash-Chat, LongCat-Flash-Thinking, etc."
                },
                lkeap: {
                    label: "腾讯云 LKEAP",
                    description: "DeepSeek-R1、DeepSeek-V3、lke-reranker-base 等"
                },
                nvidia: {
                    label: "NVIDIA",
                    description: "deepseek-ai-deepseek-v3_1, nv-embed-v1, rerank-qa-mistral-4b, etc."
                },
                novita: {
                    label: "Novita AI",
                    description: "moonshotai/kimi-k2.5, zai-org/glm-5, minimax/minimax-m2.7, qwen/qwen3-embedding-0.6b 等"
                }
            }
        },
        builtinTag: "内置"
    },
    general: {
        close: "关闭设置"
    },
    upload: {
        uploadDocument: "上传文档",
        uploadFolder: "上传文件夹"
    },
    createChat: {
        newSessionTitle: "新会话",
        messages: {
            createFailed: "创建会话失败",
            createError: "创建会话失败，请稍后重试"
        }
    },
    knowledgeEditor: {
        navGroups: {
            basic: "基础",
            processing: "索引与解析"
        },
        sidebar: {
            basic: "基本信息",
            chunking: "分块设置",
            advanced: "高级设置",
            faq: "FAQ 设置",
            multimodal: "图像处理"
        },
        basic: {
            title: "基本信息",
            description: "设置知识库的名称和描述信息",
            kbId: "知识库 ID",
            kbIdDesc: "API 集成时可使用此 ID 指定知识库",
            typeLabel: "知识库类型",
            typeDocument: "文档",
            typeFAQ: "问答",
            typeDescription: "FAQ 类型适合结构化问答数据；文档型支持文件解析、分块与混合检索。",
            nameLabel: "知识库名称",
            namePlaceholder: "请输入知识库名称",
            descriptionLabel: "知识库描述",
            descriptionPlaceholder: "请输入知识库描述（可选）"
        },
        indexing: {
            rebuildConfirmTitle: "重建索引",
            rebuildConfirmBody: "索引策略已变更，是否对已有的 {count} 篇文档重新处理？这可能需要一些时间。",
            rebuildSkip: "稍后可在数据源中手动触发重建"
        },
        buttons: {
            save: "保存配置"
        },
        messages: {
            loadModelsFailed: "加载模型列表失败",
            loadDataFailed: "加载知识库数据失败",
            notFound: "知识库不存在",
            nameRequired: "请输入知识库名称",
            multimodalInvalid: "多模态配置验证失败",
            createSuccess: "知识库创建成功",
            createFailed: "创建知识库失败",
            missingId: "缺少知识库 ID",
            buildDataFailed: "数据构建失败",
            updateSuccess: "配置保存成功",
            indexModeRequired: "请选择 FAQ 的索引方式"
        },
        document: {
            title: "文档",
            subtitle: "支持点击或拖拽上传，多格式文档自动解析并智能分块，快速构建可检索的知识库"
        },
        faq: {
            title: "问答",
            subtitle: "结构化问答管理，支持标准问、相似问和反例，精准匹配用户查询，提升问答准确率",
            description: "设置 FAQ 知识库的索引策略和问答组织方式",
            indexModeLabel: "索引方式",
            indexModeDescription: "仅索引问题可提升精度，索引问答可提高召回率",
            questionIndexModeLabel: "问题索引方式",
            questionIndexModeDescription: "合并索引：标准问和相似问合并索引；分别索引：标准问和每个相似问独立索引，检索更精确但需要更多存储",
            modes: {
                questionOnly: "仅标准问/相似问",
                questionAnswer: "标准问 + 答案",
                combined: "合并索引",
                separate: "分别索引"
            },
            standardQuestion: "标准问",
            standardQuestionDesc: "设置问题的标准表述，这是用户最常问的问题形式。",
            answers: "答案",
            answersDesc: "提供完整准确的答案内容，可添加多个答案以覆盖不同场景。",
            similarQuestions: "相似问",
            similarQuestionsDesc: "添加与标准问意思相同但表述不同的问题，帮助系统更好地匹配用户查询。",
            negativeQuestions: "反例",
            negativeQuestionsDesc: "添加不应匹配此答案的问题，用于排除误匹配的情况。",
            editorCreate: "新增 FAQ 条目",
            editorEdit: "编辑 FAQ 条目",
            answerPlaceholder: "请输入答案内容，支持多行文本，按 Ctrl+Enter 或点击按钮添加",
            similarPlaceholder: "输入相似问题后点击加号添加",
            negativePlaceholder: "输入反例后点击加号添加",
            answerRequired: "请至少填写一个答案",
            emptyTitle: "暂无 FAQ 条目",
            emptyDesc: '点击上方"新增 FAQ 条目"按钮开始创建',
            searchPlaceholder: "搜索问题和答案...",
            searchTest: "检索测试",
            createGroup: "新建",
            searchTestTitle: "FAQ 检索测试",
            queryLabel: "查询内容",
            queryPlaceholder: "请输入要检索的问题",
            vectorThresholdDesc: "范围 0-1，默认 0.7",
            matchCountLabel: "结果数量",
            matchCountDesc: "范围 1-50，默认 10",
            searchButton: "开始检索",
            searching: "检索中...",
            searchResults: "检索结果",
            noResults: "未找到匹配的 FAQ 条目",
            matchedQuestion: "命中问题",
            matchTypeEmbedding: "向量匹配",
            matchTypeKeywords: "关键词匹配",
            similarityThresholdLabel: "相似度阈值",
            statusEnabled: "已启用",
            statusDisabled: "已禁用",
            statusEnableSuccess: "FAQ 条目已启用",
            statusDisableSuccess: "FAQ 条目已禁用",
            statusUpdateFailed: "更新状态失败",
            recommended: "推荐",
            recommendedEnabled: "已开启推荐",
            recommendedDisabled: "已关闭推荐",
            recommendedEnableSuccess: "FAQ 条目已开启推荐",
            recommendedDisableSuccess: "FAQ 条目已关闭推荐",
            recommendedUpdateFailed: "更新推荐状态失败",
            selectEntriesFirst: "请先选择 FAQ 条目"
        },
        faqImport: {
            title: "批量导入 FAQ",
            modeLabel: "导入模式",
            appendMode: "追加导入",
            replaceMode: "替换现有条目",
            fileLabel: "选择文件",
            fileTip: "支持 JSON / CSV / Excel。CSV/Excel 表头：标签(必填)、问题(必填)、相似问题(选填-多个用##分隔)、反例问题(选填-多个用##分隔)、机器人回答(必填-多个用##分隔)、是否全部回复(选填-默认FALSE)、是否停用(选填-默认FALSE)、是否禁止被推荐(选填-默认False 可被推荐)。也支持旧表头「分类」及旧格式：standard_question、answers、similar_questions、negative_questions",
            clickToUpload: "点击上传文件",
            dragDropTip: "或拖拽文件到此处",
            importButton: "导入 FAQ",
            deleteSuccess: "选中条目已删除",
            previewCount: "共解析 {count} 条记录",
            previewMore: "还有 {count} 条未展示",
            importSuccess: "导入成功",
            parseFailed: "解析文件失败",
            unsupportedFormat: "暂不支持该文件格式",
            selectFile: "请先选择需要导入的文件",
            downloadExample: "下载示例文件",
            downloadExampleCSV: "下载 CSV 示例",
            downloadExampleExcel: "下载 Excel 示例"
        },
        faqExport: {
            exportButton: "导出 CSV",
            exportSuccess: "导出成功",
            exportFailed: "导出失败"
        },
        chunking: {
            description: "控制上传文档在嵌入前的切分方式。默认值适用于大多数场景，仅在检索质量异常时调整。",
            sizeLabel: "分块大小",
            sizeDescription: "每个分块的最大字符数（100-4000）。默认 512 ≈ 中文 300 tokens / 英文 100-130 tokens。FAQ 用 200-400，叙述性长文档用 1000-2000。",
            characters: "字符",
            overlapLabel: "分块重叠",
            overlapDescription: "相邻分块之间共享的字符数（0-500）。默认 80 ≈ 分块大小的 15%，符合当前研究推荐。FAQ/结构化数据用 0，长篇叙述用 150-200。",
            separatorsLabel: "分隔符",
            separatorsDescription: "切分时优先使用的字符或字符串。优先级高的分隔符先尝试；默认顺序优先段落 → 句子 → 标点。",
            separatorsPlaceholder: "选择或自定义分隔符",
            separators: {
                doubleNewline: "双换行 (\\n\\n)",
                singleNewline: "单换行 (\\n)",
                periodCn: "中文句号 (。)",
                exclamationCn: "感叹号 (！)",
                questionCn: "问号 (？)",
                semicolonCn: "中文分号 (；)",
                semicolonEn: "英文分号 (;)",
                space: "空格 ( )"
            },
            parentChildLabel: "父子分块",
            parentChildDescription: "两级分块：小的子块用于向量匹配（精准命中），大的父块返回给 LLM（更丰富上下文）。建议用于长文档（>10 页）；短 FAQ 可关闭以节省存储。",
            parentChunkSizeLabel: "父块大小",
            parentChunkSizeDescription: "返回给 LLM 的上下文块大小（512-8192）。默认 4096 ≈ 1000 英文 tokens，适合所有现代 LLM 上下文窗口。",
            childChunkSizeLabel: "子块大小",
            childChunkSizeDescription: "用于向量匹配的嵌入块大小（64-2048）。默认 384 ≈ 80 tokens，是 sentence-transformer / BGE 类嵌入模型的最佳点。",
            strategyLabel: "分块策略",
            strategyDescription: "选择文档的分块方式。自动模式会分析每个文档的结构并选择最佳策略。",
            strategyPlaceholder: "选择分块策略（不填则按长度切分）",
            strategies: {
                auto: {
                    label: "自动",
                    tooltip: "文档分析器根据内容结构自动在「按标题切分」「结构感知」「按长度切分」之间选择。"
                },
                heading: {
                    label: "按标题切分",
                    tooltip: "在 Markdown 标题（#、##、###）边界处切分，每块自动带上所在标题路径。适合结构清晰的 Markdown 文档。"
                },
                heuristic: {
                    label: "结构感知",
                    tooltip: "识别分页符、编号章节、多语言章节标记（DE/EN/ZH）、全大写标题等结构信号进行切分。适合没有 Markdown 标题的 PDF / 扫描件。"
                },
                legacy: {
                    label: "按长度切分",
                    tooltip: "忽略结构，仅按字符数和分隔符递归切分——原始行为。当上述策略对你的内容效果不佳时使用。"
                }
            },
            overlapWarning: "重叠相对于分块大小较大——分块之间会共享大部分内容。",
            advancedLabel: "高级选项",
            tokenLimitLabel: "每块 Token 上限",
            tokenLimitDescription: "每个分块的硬性 Token 上限（0-8192）。0 = 关闭（仅按字符数）。当嵌入模型 Token 上限较小时启用：MiniLM (256 tok) 用 200，BGE/Cohere (512 tok) 用 400。现代嵌入器（OpenAI、Voyage、Jina-v3）支持 >2000 tokens，保持 0 即可。",
            languagesLabel: "语言提示",
            languagesDescription: "限制启发式模式只识别选定的语言（DE/EN/ZH）。留空 = 自动检测。同质化语料库可显式设置以避免跨语言误匹配。",
            languagesPlaceholder: "自动检测",
            languageOptions: {
                de: "德语",
                en: "英语",
                zh: "中文"
            },
            debug: {
                toggle: "测试分块效果",
                toggleHint: "无需重新上传即可对示例文本运行分块器",
                sampleLabel: "示例文本",
                samplePlaceholder: "粘贴 Markdown / 纯文本片段以查看当前配置的分块结果…",
                presetLabel: "载入示例：",
                samples: {
                    markdown: "Markdown 文档",
                    faq: "FAQ 列表",
                    chapter: "PDF 章节",
                    plain: "纯文本"
                },
                runButton: "运行预览",
                loading: "正在对示例运行分块器…",
                errorPrefix: "预览失败",
                selectedTier: "选定策略",
                rejected: "被拒绝的层级",
                contextHeader: "上下文标题",
                fallbackWarning: "策略链已穷尽 — 当前设置无法智能分块此内容",
                profile: {
                    lines: "行",
                    chars: "字符",
                    headings: "Markdown 标题",
                    pageBreaks: "分页符",
                    chapterMarkers: "章节标记",
                    languages: "语言"
                },
                stats: {
                    chunks: "块",
                    truncated: "已截断；总数 {total}"
                }
            }
        },
        multimodal: {
            title: "图像处理配置",
            description: "配置图像内容理解能力，启用后支持图片等非文本内容的解析和检索"
        },
        advanced: {
            title: "高级设置",
            description: "配置问题生成等高级功能",
            questionGeneration: {
                label: "AI 问题生成",
                description: "解析文档时调用大模型为每个分块生成相关问题，提高检索召回率。启用后会增加文档解析耗时。",
                countLabel: "生成问题数量",
                countDescription: "每个文档分块生成的问题数量（1-10）"
            },
            multimodal: {
                label: "多模态功能",
                description: "启用图片等多模态内容的理解能力",
                vllmLabel: "VLLM 视觉模型",
                vllmDescription: "用于多模态理解的视觉语言模型（必选）"
            }
        }
    },
    input: {
        placeholder: "直接向模型提问",
        placeholderWithContext: "输入问题，将基于上方选中的知识库/文件回答",
        placeholderWebOnly: "输入问题，将结合网络搜索回答",
        placeholderKbAndWeb: "输入问题，将基于知识库和网络搜索回答",
        goToSettings: "前往设置 →",
        webSearch: {
            toggleOn: "开启网络搜索",
            toggleOff: "关闭网络搜索",
            notConfigured: "未配置网络搜索引擎"
        },
        knowledgeBase: "知识库",
        knowledgeBaseWithCount: "知识库({count})",
        stopGeneration: "停止生成",
        send: "发送",
        messages: {
            enterContent: "请先输入内容!",
            replying: "正在回复中，请稍后再试!",
            agentSwitchedOn: "已切换到智能推理",
            agentSwitchedOff: "已切换到快速问答",
            webSearchNotConfigured: "未配置网络搜索引擎，请先在设置中完成搜索引擎选择与接口配置。",
            webSearchEnabled: "网络搜索已开启",
            webSearchDisabled: "网络搜索已关闭",
            sessionMissing: "会话 ID 不存在",
            messageMissing: "无法获取消息 ID，请刷新页面后重试",
            stopSuccess: "已停止生成",
            stopFailed: "停止失败，请重试"
        },
        webSearchDisabledByAgent: "当前智能体已禁用网络搜索",
        kbLockedByAgent: "当前智能体已锁定知识库配置",
        kbDisabledByAgent: "当前智能体已禁用知识库功能",
        imageUploadDisabledByAgent: "当前智能体未启用图片上传"
    },
    // 新增：模型设置
    modelSettings: {
        title: "模型配置",
        description: "管理不同类型的 AI 模型，支持 Ollama 本地模型和远程 API",
        typeShort: {
            chat: "对话",
            embedding: "Embedding",
            rerank: "ReRank",
            vllm: "视觉"
        },
        actions: {
            addModel: "添加模型"
        },
        source: {
            remote: "Remote",
            openaiCompatible: "OpenAI兼容"
        },
        toasts: {
            nameRequired: "模型名称不能为空",
            nameTooLong: "模型名称不能超过100个字符",
            displayNameTooLong: "显示名称不能超过100个字符",
            baseUrlRequired: "Remote API 类型必须填写 Base URL",
            baseUrlInvalid: "Base URL 格式不正确，请输入有效的 URL",
            dimensionInvalid: "Embedding 模型必须填写有效的向量维度（128-4096）",
            updated: "模型已更新",
            added: "模型已添加",
            saveFailed: "保存模型失败",
            deleted: "模型已删除",
            deleteFailed: "删除模型失败",
            builtinCannotEdit: "内置模型不能编辑",
            builtinCannotDelete: "内置模型不能删除",
            builtinCannotCopy: "内置模型不能复制",
            copied: "模型已复制",
            copyFailed: "复制模型失败"
        },
        copySuffix: " 副本",
        builtinTag: "内置",
        confirmDelete: "确定删除模型「{name}」吗？"
    },
    preview: {
        tab: "预览",
        loading: "正在加载文档预览...",
        loadFailed: "加载文档预览失败",
        retry: "重试",
        unsupported: "该文件类型暂不支持在线预览",
        unsupportedHint: "请下载文件后使用本地应用查看",
        fullscreen: "全屏预览",
        exitFullscreen: "退出全屏"
    },
    commandPalette: {
        placeholder: "搜索知识库、文件、对话…",
        clearRecent: "清除",
        retrieval: "检索参数",
        untitledSession: "未命名对话",
        scope: {
            placeholder: "在本知识库中搜索…",
            remove: "移除范围过滤（Backspace）"
        },
        group: {
            chunks: "知识库文件",
            messages: "对话消息",
            kbs: "知识库",
            sessionsByTitle: "对话（按标题）",
            commands: "命令",
            recent: "最近搜索",
            quickActions: "快捷操作"
        },
        match: {
            vector: "向量",
            keyword: "关键字"
        },
        quick: {
            newChat: "新建对话",
            knowledgeBases: "打开知识库",
            settings: "打开设置"
        },
        empty: {
            noResults: "没有找到匹配结果",
            askAi: "直接向 AI 提问",
            adjustRetrieval: "调整检索参数"
        },
        hotkey: {
            select: "选择",
            enter: "打开",
            cmdNumber: "直接打开",
            cmdEnter: "发起对话",
            esc: "关闭"
        }
    },
    // ---- i18n keys for hardcoded Chinese extraction ----
    tools: {
        multiKbSearch: "跨库搜索",
        knowledgeSearch: "知识库搜索",
        grepChunks: "搜索关键词",
        getChunkDetail: "获取片段详情",
        listKnowledgeChunks: "查看知识分块",
        listKnowledgeBases: "列出知识库",
        getDocumentInfo: "获取文档信息",
        think: "深度思考",
        todoWrite: "制定计划"
    },
    kbSettings: {
        parser: {
            description: "为不同文件类型选择文档解析引擎。未配置的文件类型将使用内置解析引擎。",
            loading: "加载中...",
            noEngineAvailable: "暂无可用解析引擎，或文档解析服务未配置。",
            default: "默认",
            unavailable: "不可用",
            goSettings: "去设置 →",
            goConfig: "前往配置 →",
            noEngine: "无可用引擎",
            fileTypePdf: "PDF 文档",
            fileTypeWord: "Word 文档",
            fileTypePpt: "演示文稿",
            fileTypeExcel: "Excel 表格",
            fileTypeEbook: "电子书",
            fileTypeWebArchive: "网页归档",
            fileTypeCsv: "CSV 文件",
            fileTypeText: "纯文本",
            fileTypeJson: "JSON 文件",
            fileTypeImage: "图片",
            engines: {
                builtin: {
                    name: "内置",
                    desc: "DocReader 内置解析引擎（docx/pdf/xlsx 等复杂格式）"
                },
                simple: {
                    name: "Simple",
                    desc: "简单格式 & 图片解析（无需外部服务）"
                },
                mineru: {
                    name: "MinerU",
                    desc: "MinerU 自部署服务"
                },
                mineru_cloud: {
                    name: "MinerU Cloud",
                    desc: "MinerU Cloud API"
                },
                paddleocr_vl: {
                    name: "PaddleOCR-VL",
                    desc: "PaddleOCR-VL 自部署服务"
                },
                paddleocr_vl_cloud: {
                    name: "PaddleOCR-VL Cloud",
                    desc: "PaddleOCR-VL 云 API"
                },
                weknoracloud: {
                    name: "WeKnora Cloud",
                    desc: "使用 WeKnora Cloud 进行文档解析"
                },
                markitdown: {
                    name: "MarkItDown",
                    desc: "Microsoft MarkItDown 文档转换工具（支持 PDF/Office/HTML 等）"
                },
                opendataloader: {
                    name: "OpenDataLoader",
                    desc: "OpenDataLoader PDF 解析引擎（版面分析，需 Java 11+ 与 opendataloader-pdf）"
                }
            }
        },
        supportedFormats: "支持格式"
    },
    agentStream: {
        tools: {
            searchKnowledge: "知识库检索",
            grepChunks: "搜索关键词",
            webSearch: "网络搜索",
            webFetch: "网页抓取",
            getDocumentInfo: "获取文档信息",
            listKnowledgeChunks: "查看知识分块",
            getRelatedDocuments: "查找相关文档",
            getDocumentContent: "获取文档内容",
            wikiReadSourceDoc: "精读源文档",
            todoWrite: "计划管理",
            thinking: "思考",
            attachmentParsing: "解析附件",
            imageAnalysis: "查看图片内容",
            queryUnderstand: "理解问题"
        },
        citation: {
            notFound: "未找到内容",
            loadFailed: "加载失败",
            noKbForWiki: "无法识别关联的知识库，无法打开 Wiki"
        },
        toolSummary: {
            getDocument: "获取文档：{title}",
            document: "文档",
            listChunks: "查看 {title}",
            listFaqEntry: "查看 FAQ：{question}",
            deepThinking: "深度思考"
        },
        plan: {
            inProgress: "进行中",
            pending: "待处理",
            completed: "已完成"
        },
        search: {
            noResults: "未找到匹配的内容",
            foundResultsFromFiles: "找到 {count} 个结果，来自 {files} 个文件",
            foundResults: "找到 {count} 个结果",
            webResults: "找到 {count} 条网页",
            grepSummary: "找到 {chunks} 个匹配片段，来自 {docs} 个文档"
        },
        grepResults: {
            chunkHits: "{count} 片段",
            keywordHits: "{count} 次",
            titleMatch: "标题匹配",
            faqEntry: "FAQ 条目"
        },
        knowledgeChunksList: {
            chunkRange: "已加载 {fetched} / {total} 个分块",
            page: "第 {page} 页，每页 {pageSize} 个"
        },
        ragPipeline: {
            searching: "正在检索知识库...",
            searchingWithQuery: "正在检索知识库：「{query}」",
            searchDone: "检索完成",
            referencedDocs: "引用 <strong>{count}</strong> 篇文档",
            referencedWebs: "引用 <strong>{count}</strong> 条网页",
            referencedDocAndWeb: "引用 <strong>{docCount}</strong> 篇文档和 <strong>{webCount}</strong> 条网页"
        },
        toolStatus: {
            calling: "正在调用 {name}...",
            searchKb: "检索知识库",
            searchKbFailed: "检索知识库失败",
            webSearch: "网络搜索",
            webSearchFailed: "网络搜索失败",
            grepSearch: "搜索关键词",
            grepSearchFailed: "搜索关键词失败",
            getDocInfo: "获取文档信息",
            getDocInfoFailed: "获取文档信息失败",
            viewDocument: "查看文档",
            thinkingDone: "完成思考",
            thinkingFailed: "思考失败",
            updateTodos: "更新任务列表",
            updateTodosFailed: "更新任务列表失败",
            imageAnalyzing: "正在查看图片内容...",
            imageAnalysisDone: "已查看图片内容",
            imageAnalysisFailed: "图片内容查看失败",
            queryUnderstanding: "正在理解问题...",
            queryUnderstandDone: "已完成问题理解",
            called: "调用 {name}",
            calledFailed: "调用 {name} 失败"
        },
        copy: {
            emptyContent: "当前回答为空，无法复制",
            success: "已复制到剪贴板",
            failed: "复制失败，请手动复制"
        }
    },
    faqManager: {
        import: {
            recentResult: "最近导入结果",
            totalData: "导入数据",
            success: "成功",
            failed: "失败",
            skipped: "跳过",
            unit: "条",
            downloadReasons: "下载原因",
            appendMode: "追加模式",
            replaceMode: "替换模式",
            importing: "导入中...",
            importDone: "导入完成",
            importFailed: "导入失败",
            waiting: "等待中...",
            importInProgress: "导入正在进行中，请等待完成后再试",
            noFailedRecords: "暂无失败记录可下载"
        }
    },
    mermaid: {
        diagram: "图表",
        expand: "全屏查看",
        zoomIn: "放大",
        zoomOut: "缩小",
        reset: "重置",
        download: "下载图片",
        close: "关闭",
        downloading: "下载中..."
    },
    credential: {
        configured: "已配置",
        unconfigured: "未配置",
        configure: "配置",
        update: "更换",
        remove: "移除",
        inputPlaceholder: "请输入",
        savedToast: "凭据已保存",
        saveFailed: "保存凭据失败",
        removedToast: "凭据已移除",
        removeFailed: "移除凭据失败",
        confirmRemovePrompt: "确认移除？此操作不可撤销",
        confirmRemove: "确认移除"
    }
};
