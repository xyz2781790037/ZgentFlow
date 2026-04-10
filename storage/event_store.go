package storage

import (
	wfpb "ZgentFlow/proto/workflow"
	"ZgentFlow/tool"
	"fmt"
	"time"

	"ZgentFlow/storage/badger"

	"google.golang.org/protobuf/proto"
)

type EventStore struct {
	db *badger.DB
}

func NewEventStore(dirPath string) (*EventStore, error) {
	opt := badger.DefaultOptions(dirPath)
	db, err := badger.Open(opt)
	if err != nil {
		return nil, fmt.Errorf("无法启动底层 Badger 引擎: %v", err)
	}
	return &EventStore{
		db: db,
	}, nil
}
func (s *EventStore) AppendEvent(workflowID string, eventType wfpb.EventType, taskID string, payload []byte) error {
	event := &wfpb.WorkflowEvent{
		WorkflowID: workflowID,
		TaskID:     taskID,
		EventType:  eventType,
		TimeStamp:  time.Now().UnixNano(),
		Payload:    payload,
	}
	data, err := proto.Marshal(event)
	if err != nil {
		return fmt.Errorf("事件 protobuf 序列化失败: %v", err)
	}
	key := []byte(fmt.Sprintf("event:%s:%020d:%s", workflowID, event.TimeStamp, taskID))
	_, err = s.db.Put(key, data)
	if err != nil {
		tool.Log.Error("[EventStore] 落盘失败 Workflow=%s, Err=%v", workflowID, err)
		return err
	}
	return nil
}
func (s *EventStore)Close() error{
	return s.db.Close()
}