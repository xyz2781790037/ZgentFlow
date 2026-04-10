package storage

import (
	"context"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// 1. 定义极其干净的面向 Agent 的记忆接口
type MemoryStore interface {
	// 存入记忆：将 Agent 的思考或执行结果存入长期记忆
	SaveMemory(ctx context.Context, collectionName string, text string, vector []float32, metadata map[string]string) error

	// 检索记忆：根据当前问题的向量，找出最相似的历史 Top-K 记忆
	SearchMemory(ctx context.Context, collectionName string, queryVector []float32, limit uint64) ([]MemoryResult, error)
}

type MemoryResult struct {
    Text  string            // 原文
    Score float32           // 相似度得分 (0~1)
    Meta  map[string]string // 其他元数据 (比如这条记忆的来源、时间)
}

// 2. Qdrant 具体实现
type QdrantStore struct {
    client qdrant.PointsClient // 负责调用增删改查的 gRPC 客户端
    conn   *grpc.ClientConn    // 底层的 TCP 长连接
}

// NewQdrantStore 完整函数：初始化 Qdrant 客户端
func NewQdrantStore(addr string) (*QdrantStore, error) {
	// 修改点：使用 grpc.NewClient 替换已废弃的 grpc.Dial
	// grpc.NewClient 提供了更好的名称解析和负载均衡初始化机制
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	client := qdrant.NewPointsClient(conn)
	return &QdrantStore{client: client, conn: conn}, nil
}

// SaveMemory 完整函数：存入记忆
func (q *QdrantStore) SaveMemory(ctx context.Context, collection string, text string, vector []float32, metadata map[string]string) error {
	// 生成唯一的记忆 ID
	pointId := uuid.New().String()

	// 将元数据和原始文本存入 Payload (使得搜出来的不仅是向量，还有人类可读的文字)
	payload := make(map[string]*qdrant.Value)
	payload["text"] = &qdrant.Value{Kind: &qdrant.Value_StringValue{StringValue: text}}
	for k, v := range metadata {
		payload[k] = &qdrant.Value{Kind: &qdrant.Value_StringValue{StringValue: v}}
	}

	_, err := q.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: collection,
		Wait:           func() *bool { b := true; return &b }(), // 强一致性等待
		Points: []*qdrant.PointStruct{
			{
				Id:      &qdrant.PointId{PointIdOptions: &qdrant.PointId_Uuid{Uuid: pointId}},
				Vectors: &qdrant.Vectors{VectorsOptions: &qdrant.Vectors_Vector{Vector: &qdrant.Vector{Data: vector}}},
				Payload: payload,
			},
		},
	})
	return err
}

// SearchMemory 完整函数：检索记忆
func (q *QdrantStore) SearchMemory(ctx context.Context, collection string, queryVector []float32, limit uint64) ([]MemoryResult, error) {
	searchResult, err := q.client.Search(ctx, &qdrant.SearchPoints{
		CollectionName: collection,
		Vector:         queryVector,
		Limit:          limit,
		WithPayload:    &qdrant.WithPayloadSelector{SelectorOptions: &qdrant.WithPayloadSelector_Enable{Enable: true}}, // 要求返回 Payload 文本
	})
	if err != nil {
		return nil, err
	}

	var results []MemoryResult
	for _, point := range searchResult.GetResult() {
		res := MemoryResult{
			Score: point.Score,
			Meta:  make(map[string]string),
		}
		// 从 Payload 中提取原始文本
		if textVal, ok := point.Payload["text"]; ok {
			res.Text = textVal.GetStringValue()
		}
		for k, v := range point.Payload {
			if k != "text" {
				res.Meta[k] = v.GetStringValue()
			}
		}
		results = append(results, res)
	}
	return results, nil
}

func (q *QdrantStore) Close() error {
	return q.conn.Close()
}