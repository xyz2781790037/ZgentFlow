# ZgentFlow 项目交接（2026-07-30，知识库共享更新）

## 1. 当前在做什么

本仓库基于 WeKnora 持续裁剪和重构，当前产品名称统一为 **ZgentFlow**：一个带账号访问控制、按用户 tenant 隔离的文档知识库问答产品。当前总任务是删除前后端没有展示、或已经被产品范围排除的功能及其死代码，同时保证保留业务可以正常构建和运行。

最新一轮已经新增“知识库级共享”。这不是恢复旧组织共享：账号仍拥有独立 tenant，只有明确加入 `knowledge_base_members` 的用户能够跨 tenant 访问指定知识库，不能访问所有者的其他知识库、模型列表、会话或文件。知识库问答、解析、向量化和重排使用知识库所有者 tenant 的模型；提问者的会话和消息仍保存在自己的 tenant/user 下。

这一轮最近完成了“删除深度问答、Wiki 和 OCR”这一大块裁剪。当前不要把旧的 Wiki、深度问答、Agent 或 OCR 流程重新接回去。

此前交接中记录但本轮没有继续处理的两个问题是：

1. 知识库详情中的文档列表加载不出来：先取证定位原因，再修复。
2. `make dev-app` 启动失败时，需要明确打印占用后端端口的实际 PID/进程信息，方便排障。

截至交接时，用户要求服务保持停止。除非用户再次明确允许，不要启动 `make dev-app`、前端开发服务器、Go 服务、Docker 或其他常驻进程；可以做静态检查、单元测试和构建验证。

### 本轮知识库共享实现

- 新增迁移 `000076_knowledge_base_sharing`：知识库所有者、邀请码开关、成员、加入申请、审计日志及文档上传者。
- 知识库列表同时返回自己拥有和别人共享的知识库，并附带 `access_role`、`is_shared`、`owner_username`。
- 邀请码仅所有者可开启、关闭和重新生成；长期有效，关闭只阻止新申请，旧申请及现有成员保留。
- 输入有效邀请码后先显示知识库名称和所有者用户名；提交申请后等待所有者/管理员审批，批准后默认 Reader。
- 角色为 Owner/Admin/Writer/Reader：Owner 管配置与管理员；Admin 审批、增删文档、重建、普通成员写入权限与移除；Writer 上传并重试自己失败的文档；Reader 查看、预览、下载和问答。
- 成员列表所有成员可见，但只显示用户名和角色；共享操作日志仅 Owner/Admin 可见。
- 文档、FAQ、chunk、重建、批量删除/移动和初始化配置路由已经接入 KB ACL；不能靠猜测 UUID 越权访问。
- RAG 搜索按每个 SearchTarget 的存储 tenant 执行；问答模型、Embedding、VLM 和 Rerank 切换到知识库所有者 tenant；会话历史仍按调用者 user 隔离。
- 前端新增“加入已有知识库”弹窗和知识库“共享成员”管理弹窗，申请状态只在页面内展示，不做邮件/消息中心。
- 所有权转让没有实现。现有异步解析与模型加载仍把存储 tenant 当作模型 tenant；安全转让需要新增独立模型归属并改造后台任务，超过用户限定的“小代码量才增加”条件，不能只更新 `owner_user_id`。
- 服务保持停止，因此迁移尚未通过启动自动应用；下次正常启动会由自动迁移执行。不要为应用迁移而私自启动整套服务。

本轮验证已通过：

```bash
go test ./...
cd frontend && npm run type-check
cd frontend && npm run build
```

前端构建仍只有已有的 chunk 体积警告。

## 2. 已确认的产品范围

### 必须保留

- 普通文档知识库、FAQ 知识库。
- 仅通过文件上传自动创建文档知识；保留 FAQ 问题自动生成。
- 快速问答：用户可选择联网或不联网；保留知识库引用、联网能力、提示词和模型管理。
- 会话历史列表、搜索、置顶、删除；保留对话标题生成、重试/编辑、复制等已在用能力。
- 知识库、文档及模型管理；文档可移动至其他知识库。
- 删除后的知识库与文档进入独立回收站，保留 7 天后直接删除。
- 向量完整重建、向量缓存复用、重建失败后的重新解析；不能因为裁剪破坏缓存代际切换或向量完整性校验。
- 文档解析失败数量：不在普通界面展示，但在解析流程中展示；失败应保留到用户主动删除。
- 多模型配置、统一检索参数配置、每个知识库创建时选择一种嵌入模型；可最大化复用已有结果，用户可以主动重建。
- FAQ 保留原始文本分块，允许之后单独重试问题生成；默认召回 3 个结果且可配置，并单独召回。

### 已明确删除或不做

- 深度问答、`agent-chat`、自定义 Agent 的创建/编辑/删除/分享，以及数据分析师。
- Wiki 知识库、Wiki 重建和浏览、知识图谱 / GraphRAG。
- OCR、图片/多模态文档解析、PaddleOCR、文档图片落盘和图片分块。
- 备份与恢复（包括本地定时自动备份）、模型自定义 HTTP 请求头、所有引导流程。
- 标签管理、语音模型、界面偏好设置。
- 旧组织、组织邀请和 Agent 分享仍不恢复。每个注册账号保持独立 tenant；唯一允许的跨 tenant 路径是本轮新增的“知识库成员 ACL”，且只开放获准加入的单个知识库资源树。

## 3. 本轮已经完成的工作

### 后端

- 删除深度 Agent 引擎和相关流式事件依赖。快速问答 SSE 现在独立转发答案、引用、错误、标题和完成事件。
- 删除 Wiki 的重建、后处理、清理、任务队列、测试和完整重建中的 Wiki 发布。
- 保留向量完整重建，暂存代际、向量缓存复用、向量完整性校验和原子切换仍在。
- 删除图片/OCR 多模态任务、图片分块、图片落盘及图片解析器依赖。
- MinerU 解析固定为文本模式：`parse_method=txt`、`return_images=false`。
- 删除 PaddleOCR 解析选择。
- FAQ 问题生成、摘要和文本分块改为只使用文本块。
- 删除或改写已失效的 Wiki/OCR 测试；文本缓存和向量重建测试仍保留。

### 前端

- `settings.ts` 固定快速模式，移除深度模式和 Wiki 本地状态。
- 输入框移除“快速/深度”切换和 Wiki 开关。
- 聊天请求固定为 `/api/v1/knowledge-chat`，不再提交 `agent_enabled`、`agent_id`、`wiki_enabled`。
- 删除 `agentChat` API 与 `BUILTIN_SMART_REASONING_ID`。
- 聊天消息移除深度思考、Agent 流和 RAG 工具进度组件；`RagPipelineProgress.vue` 已删除，不能再引用。
- 知识库详情页移除 Wiki 浏览器、Wiki Tab、Wiki 状态轮询；点击文档改为打开现有详情抽屉。
- 知识库创建页移除 Wiki 开关、Wiki 缓存文案和 Wiki 请求字段。
- 知识库编辑页已移除可见 Wiki、多模态入口；创建/编辑请求不再提交 `vlm_config`、`wiki_config`、`wiki_enabled`。
- `frontend/src/api/knowledge-base/index.ts` 与初始化 API 类型已同步移除这些字段。

### 已完成验证

下列命令在上述裁剪后通过：

```bash
go test ./...
cd frontend && npm run type-check
cd frontend && npm run build
```

前端构建只有已有的大 chunk 警告，没有 TypeScript 或构建错误。`frontend/dist` 是构建产物，工作区里出现它不代表需要回滚用户改动。

## 4. 下一步：先处理两个未解决问题

### A. 知识库文档为何无法加载

目前尚无根因结论，不能凭前端裁剪直接猜测或回退。

从前端入口按调用链排查：

- `frontend/src/views/knowledge/KnowledgeBase.vue`：`loadKnowledgeFiles`、路由 `kbId`、文档列表的响应判断。
- `frontend/src/api/knowledge-base/index.ts`：实际请求路径、参数和返回类型。
- 后端路由与对应 `KnowledgeHandler`：确认列表接口、鉴权/租户过滤、响应结构。

重点确认删除 Wiki 后是否留下了对旧响应字段的判断、错误的 `kbId`、或前后端响应结构不一致。定位后做最小修复，并重新执行至少相关 Go 测试、前端类型检查与构建。

### B. `make dev-app` 失败时打印 PID

先读取 `Makefile` 和 `scripts/` 中 `dev-app` 的实际调用链，再只修改失败诊断路径。目标：如果后端 `8081` 端口已被占用或启动失败，打印监听者的实际 PID 和进程信息，例如：

```bash
lsof -iTCP:8081 -sTCP:LISTEN -n -P
```

或在没有 `lsof` 时采用 `ss -ltnp`。不要使用 `kill 8081`，端口号不是 PID；此需求是增加诊断信息，不是自动终止已有进程。修改后不启动服务，优先用 shell 静态检查或脚本的可验证分支确认行为。

## 5. 仍可能存在的死代码，清理前必须沿调用链确认

OCR/Wiki 表层功能已移除，但以下位置仍可能保留已不可达的旧配置、类型或样式：

- `internal/application/service/knowledge_create.go`
- `internal/application/service/knowledge_process_config.go`
- `internal/handler/knowledge.go`
- `internal/types/knowledge_process.go`
- `internal/types/tenant.go`
- `frontend/src/views/knowledge/components/UploadConfirmDialog.vue`
- `frontend/src/components/knowledge-processing-progress.vue`
- `frontend/src/components/knowledge-processing-timeline.vue`
- `frontend/src/views/knowledge/components/KnowledgeBaseEditorModal.vue`
- `frontend/src/views/knowledge/ZealLibrary.vue`
- Wiki 相关 i18n 文案和 `chunkingSamples.ts`

特别注意：上传确认、普通文档上传和解析进度是保留业务，不能因为它们仍带有旧字段就整文件删除。应当把 `enable_multimodel`、`vlm_config` 等字段从前端请求、DTO、Handler、Service 和类型中一致移除，然后再删无法到达的辅助逻辑。

`KnowledgeBaseEditorModal.vue` 里可能还留有无界面引用的 `WIKI_ONLY_CHUNKING_PRESET`、`resolvedGranularity`、`defaultVLM` 和多模态 CSS；先确认没有调用者再定点删掉。

`knowledge_process.go` 中的 `previewText` 与 `isFinalAsynqAttempt` 是保留下来的通用文本工具，不要误删为 OCR 专属逻辑。`minTextContentRunes = 10` 以及 `checkSufficientSummaryContent` 是纯文本摘要的有效性校验，也不是 OCR 功能。

## 6. 工作区和操作约束

- 工作目录：`/home/zgyx/ZgentFlow`。
- 工作树本来就非常脏：旧工作流/Badger/工具代码被删除，ZgentFlow 后端、前端、迁移、配置、脚本等大量文件仍为未跟踪状态。这是迁移过程的预期状态。
- 绝不能执行 `git reset --hard`、`git checkout --`、广泛 `rm -rf`，也不要清理未跟踪目录。用户只授权删除无用源码，不授权删除数据库、上传文件、迁移或其他业务数据。
- 手工改文件使用 `apply_patch`；修改范围保持小，避免顺手重构无关模块。
- 保留现有正式 `*_test.go`。有些旧测试文件含共享测试工具，不能仅因原功能删除就整文件移除。
- 用户偏好中文沟通；需要澄清时一次只问一个问题。当前范围已经明确时应直接实施，不要反复确认。

## 7. 历史交接中已经失效的内容

本文件旧版本中“保留 Wiki、两种问答模式、重建 Wiki/OCR 缓存、继续清理分享服务”等表述均已被后续用户决定覆盖，不应再作为实施依据。以本文第 2 节为唯一产品范围。

## 8. 建议的下一位接手方式

先保持服务停止，按第 4A 的前后端调用链找出文档列表加载失败的证据并最小修复；随后按第 4B 为 `make dev-app` 增加只读 PID 诊断。两项完成后，再继续第 5 节列出的残余 OCR/Wiki 配置清理，每批改动都执行：

```bash
gofmt -w <本批修改的 Go 文件>
go test ./...
cd frontend && npm run type-check
cd frontend && npm run build
```

除非用户明确允许，不要运行应用栈。
