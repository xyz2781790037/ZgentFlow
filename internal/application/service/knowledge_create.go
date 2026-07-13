package service

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	werrors "github.com/xyz2781790037/ZealRAG/internal/errors"
	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/tracing/langfuse"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	secutils "github.com/xyz2781790037/ZealRAG/internal/utils"
)

// CreateKnowledgeFromFile creates a knowledge entry from an uploaded file
func (s *knowledgeService) CreateKnowledgeFromFile(ctx context.Context,
	kbID string, file *multipart.FileHeader, metadata map[string]string, enableMultimodel *bool, customFileName string,
	processOverrides *types.KnowledgeProcessOverrides,
) (*types.Knowledge, error) {
	logger.Info(ctx, "Start creating knowledge from file")

	// Use custom filename if provided, otherwise use original filename
	fileName := file.Filename
	if customFileName != "" {
		fileName = customFileName
		logger.Infof(ctx, "Using custom filename: %s (original: %s)", customFileName, file.Filename)
	}

	logger.Infof(ctx, "Knowledge base ID: %s, file: %s", kbID, fileName)

	if IsVideoType(getFileType(fileName)) {
		logger.Error(ctx, "Video file upload is not supported")
		return nil, werrors.NewBadRequestError("暂不支持上传视频文件")
	}

	// Get knowledge base configuration
	logger.Info(ctx, "Getting knowledge base configuration")
	kb, err := s.kbService.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		logger.Errorf(ctx, "Failed to get knowledge base: %v", err)
		return nil, err
	}

	// FAQ knowledge bases should not accept file uploads — use the FAQ import API instead
	if kb.Type == types.KnowledgeBaseTypeFAQ {
		return nil, werrors.NewBadRequestError("FAQ 知识库不支持文件上传，请使用 FAQ 导入功能")
	}

	// Validate file type
	logger.Infof(ctx, "Checking file type: %s", fileName)
	if !isValidFileType(fileName) {
		logger.Error(ctx, "Invalid file type")
		return nil, ErrInvalidFileType
	}

	// Calculate file hash for deduplication
	logger.Info(ctx, "Calculating file hash")
	hash, err := calculateFileHash(file)
	if err != nil {
		logger.Errorf(ctx, "Failed to calculate file hash: %v", err)
		return nil, err
	}

	// Check if file already exists
	tenantID := ctx.Value(types.TenantIDContextKey).(uint64)
	logger.Infof(ctx, "Checking if file exists, tenant ID: %d", tenantID)
	exists, existingKnowledge, err := s.repo.CheckKnowledgeExists(ctx, tenantID, kbID, &types.KnowledgeCheckParams{
		Type:     "file",
		FileName: fileName,
		FileSize: file.Size,
		FileHash: hash,
	})
	if err != nil {
		logger.Errorf(ctx, "Failed to check knowledge existence: %v", err)
		return nil, err
	}
	if exists {
		logger.Infof(ctx, "File already exists: %s", fileName)
		// Update creation time for existing knowledge
		if err := s.repo.UpdateKnowledgeColumn(ctx, existingKnowledge.ID, "created_at", time.Now()); err != nil {
			logger.Errorf(ctx, "Failed to update existing knowledge: %v", err)
			return nil, err
		}
		return existingKnowledge, types.NewDuplicateFileError(existingKnowledge)
	}

	// Check storage quota
	tenantInfo := ctx.Value(types.TenantInfoContextKey).(*types.Tenant)
	if tenantInfo.StorageQuota > 0 && tenantInfo.StorageUsed >= tenantInfo.StorageQuota {
		logger.Error(ctx, "Storage quota exceeded")
		return nil, types.NewStorageQuotaExceededError()
	}

	// Convert metadata to JSON format if provided
	var metadataJSON types.JSON
	if metadata != nil {
		metadataBytes, err := json.Marshal(metadata)
		if err != nil {
			logger.Errorf(ctx, "Failed to marshal metadata: %v", err)
			return nil, err
		}
		metadataJSON = types.JSON(metadataBytes)
	}

	// 验证文件名安全性
	safeFilename, isValid := secutils.ValidateInput(fileName)
	if !isValid {
		logger.Errorf(ctx, "Invalid filename: %s", fileName)
		return nil, werrors.NewValidationError("文件名包含非法字符")
	}

	eff := ResolveProcessConfig(kb, processOverrides)
	if enableMultimodel != nil && (processOverrides == nil || processOverrides.EnableMultimodel == nil) {
		eff.EnableMultimodel = *enableMultimodel
	}

	if processOverrides != nil {
		if err := ValidateProcessOverrides(ctx, kb, processOverrides, []string{getFileType(safeFilename)}); err != nil {
			return nil, err
		}
	} else {
		// 检查多模态配置完整性 - 只在图片文件时校验
		if IsImageType(getFileType(safeFilename)) {
			if !kb.VLMConfig.Enabled || kb.VLMConfig.ModelID == "" {
				logger.Error(ctx, "VLM model is not configured")
				return nil, werrors.NewBadRequestError("上传图片文件需要设置VLM模型")
			}
		}

	}

	// Prepare knowledge record
	logger.Info(ctx, "Preparing knowledge record")
	knowledge := &types.Knowledge{
		ID:               uuid.New().String(),
		TenantID:         tenantID,
		KnowledgeBaseID:  kbID,
		UploadedByUserID: currentUserID(ctx),
		Type:             "file",
		Title:            safeFilename,
		FileName:         safeFilename,
		FileType:         getFileType(safeFilename),
		FileSize:         file.Size,
		FileHash:         hash,
		ParseStatus:      "pending",
		EnableStatus:     "disabled",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		EmbeddingModelID: kb.EmbeddingModelID,
		Metadata:         metadataJSON,
	}

	if processOverrides != nil {
		if err := knowledge.SetProcessOverrides(processOverrides); err != nil {
			logger.Errorf(ctx, "Failed to set process overrides: %v", err)
			return nil, err
		}
	}

	// Save the file to storage (use KB-level storage engine if configured)
	logger.Infof(ctx, "Saving file, knowledge ID: %s", knowledge.ID)
	fileSvc := s.resolveFileService(ctx, kb)
	filePath, err := fileSvc.SaveFile(ctx, file, knowledge.TenantID, knowledge.ID)
	if err != nil {
		logger.Errorf(ctx, "Failed to save file, knowledge ID: %s, error: %v", knowledge.ID, err)
		return nil, err
	}
	knowledge.FilePath = filePath

	// Save knowledge record to database after the file is safely stored.
	logger.Info(ctx, "Saving knowledge record to database")
	if err := s.repo.CreateKnowledge(ctx, knowledge); err != nil {
		logger.Errorf(ctx, "Failed to create knowledge record, ID: %s, error: %v", knowledge.ID, err)
		if deleteErr := fileSvc.DeleteFile(ctx, filePath); deleteErr != nil {
			logger.Errorf(ctx, "Failed to delete saved file after knowledge creation failed, path: %s, error: %v", filePath, deleteErr)
		}
		return nil, err
	}

	// Enqueue document processing task to Asynq
	logger.Info(ctx, "Enqueuing document processing task to Asynq")
	enableMultimodelValue := eff.EnableMultimodel

	enableQuestionGeneration := eff.QuestionGenerationConfig.Enabled
	questionCount := eff.QuestionGenerationConfig.QuestionCount
	if questionCount <= 0 {
		questionCount = 3
	}

	lang, _ := types.LanguageFromContext(ctx)
	taskPayload := types.DocumentProcessPayload{
		TenantID:                 tenantID,
		KnowledgeID:              knowledge.ID,
		KnowledgeBaseID:          kbID,
		FilePath:                 filePath,
		FileName:                 safeFilename,
		FileType:                 getFileType(safeFilename),
		EnableMultimodel:         enableMultimodelValue,
		EnableQuestionGeneration: enableQuestionGeneration,
		QuestionCount:            questionCount,
		Language:                 lang,
	}

	langfuse.InjectTracing(ctx, &taskPayload)
	payloadBytes, err := json.Marshal(taskPayload)
	if err != nil {
		logger.Errorf(ctx, "Failed to marshal document process task payload: %v", err)
		// 即使入队失败，也返回knowledge，因为文件已保存
		return knowledge, nil
	}

	task := asynq.NewTask(
		types.TypeDocumentProcess,
		payloadBytes,
	)
	info, err := s.task.Enqueue(task,
		documentProcessTaskOptions(s.config, asynq.MaxRetry(3))...)
	if err != nil {
		logger.Errorf(ctx, "Failed to enqueue document process task: %v", err)
		// 即使入队失败，也返回knowledge，因为文件已保存
		return knowledge, nil
	}
	logger.Infof(
		ctx,
		"Enqueued document process task: id=%s queue=%s knowledge_id=%s",
		info.ID,
		info.Queue,
		knowledge.ID,
	)

	logger.Infof(ctx, "Knowledge from file created successfully, ID: %s", knowledge.ID)
	return knowledge, nil
}

// CreateKnowledgeFromPassage creates a knowledge entry from text passages
func (s *knowledgeService) CreateKnowledgeFromPassage(ctx context.Context,
	kbID string, passage []string,
) (*types.Knowledge, error) {
	return s.createKnowledgeFromPassageInternal(ctx, kbID, passage, false)
}

// CreateKnowledgeFromPassageSync creates a knowledge entry from text passages and waits for indexing to complete.
func (s *knowledgeService) CreateKnowledgeFromPassageSync(ctx context.Context,
	kbID string, passage []string,
) (*types.Knowledge, error) {
	return s.createKnowledgeFromPassageInternal(ctx, kbID, passage, true)
}

func (s *knowledgeService) createKnowledgeFromPassageInternal(ctx context.Context,
	kbID string, passage []string, syncMode bool,
) (*types.Knowledge, error) {
	if syncMode {
		logger.Info(ctx, "Start creating knowledge from passage (sync)")
	} else {
		logger.Info(ctx, "Start creating knowledge from passage")
	}
	logger.Infof(ctx, "Knowledge base ID: %s, passage count: %d", kbID, len(passage))

	// 验证段落内容安全性
	safePassages := make([]string, 0, len(passage))
	for i, p := range passage {
		safePassage, isValid := secutils.ValidateInput(p)
		if !isValid {
			logger.Errorf(ctx, "Invalid passage content at index %d", i)
			return nil, werrors.NewValidationError(fmt.Sprintf("段落 %d 包含非法内容", i+1))
		}
		safePassages = append(safePassages, safePassage)
	}

	// Get knowledge base configuration
	logger.Info(ctx, "Getting knowledge base configuration")
	kb, err := s.kbService.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		logger.Errorf(ctx, "Failed to get knowledge base: %v", err)
		return nil, err
	}

	// Create knowledge record
	if syncMode {
		logger.Info(ctx, "Creating knowledge record (sync)")
	} else {
		logger.Info(ctx, "Creating knowledge record")
	}
	knowledge := &types.Knowledge{
		ID:               uuid.New().String(),
		TenantID:         ctx.Value(types.TenantIDContextKey).(uint64),
		KnowledgeBaseID:  kbID,
		UploadedByUserID: currentUserID(ctx),
		Type:             "passage",
		ParseStatus:      "pending",
		EnableStatus:     "disabled",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		EmbeddingModelID: kb.EmbeddingModelID,
	}

	// Save knowledge record
	logger.Infof(ctx, "Saving knowledge record to database, ID: %s", knowledge.ID)
	if err := s.repo.CreateKnowledge(ctx, knowledge); err != nil {
		logger.Errorf(ctx, "Failed to create knowledge record: %v", err)
		return nil, err
	}

	// Process passages
	if syncMode {
		logger.Info(ctx, "Processing passage synchronously")
		s.processDocumentFromPassage(ctx, kb, knowledge, safePassages)
		logger.Infof(ctx, "Knowledge from passage created successfully (sync), ID: %s", knowledge.ID)
	} else {
		// Enqueue passage processing task to Asynq
		logger.Info(ctx, "Enqueuing passage processing task to Asynq")
		tenantID := ctx.Value(types.TenantIDContextKey).(uint64)

		// Check question generation config
		enableQuestionGeneration := false
		questionCount := 3 // default
		if kb.QuestionGenerationConfig != nil && kb.QuestionGenerationConfig.Enabled {
			enableQuestionGeneration = true
			if kb.QuestionGenerationConfig.QuestionCount > 0 {
				questionCount = kb.QuestionGenerationConfig.QuestionCount
			}
		}

		lang, _ := types.LanguageFromContext(ctx)
		taskPayload := types.DocumentProcessPayload{
			TenantID:                 tenantID,
			KnowledgeID:              knowledge.ID,
			KnowledgeBaseID:          kbID,
			Passages:                 safePassages,
			EnableMultimodel:         false, // 文本段落不支持多模态
			EnableQuestionGeneration: enableQuestionGeneration,
			QuestionCount:            questionCount,
			Language:                 lang,
		}

		langfuse.InjectTracing(ctx, &taskPayload)
		payloadBytes, err := json.Marshal(taskPayload)
		if err != nil {
			logger.Errorf(ctx, "Failed to marshal passage process task payload: %v", err)
			// 即使入队失败，也返回knowledge
			return knowledge, nil
		}

		task := asynq.NewTask(
			types.TypeDocumentProcess,
			payloadBytes,
		)
		info, err := s.task.Enqueue(task,
			documentProcessTaskOptions(s.config, asynq.MaxRetry(3))...)
		if err != nil {
			logger.Errorf(ctx, "Failed to enqueue passage process task: %v", err)
			return knowledge, nil
		}
		logger.Infof(ctx, "Enqueued passage process task: id=%s queue=%s knowledge_id=%s", info.ID, info.Queue, knowledge.ID)
		logger.Infof(ctx, "Knowledge from passage created successfully, ID: %s", knowledge.ID)
	}
	return knowledge, nil
}

func currentUserID(ctx context.Context) string {
	id, _ := types.UserIDFromContext(ctx)
	return id
}
