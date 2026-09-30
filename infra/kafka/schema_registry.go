package infrakafka

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/riferrei/srclient"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type SchemaRegistryClient struct {
	client *srclient.SchemaRegistryClient
}

func NewSchemaRegistryClient(url string) (*SchemaRegistryClient, error) {
	client := srclient.NewSchemaRegistryClient(url)
	return &SchemaRegistryClient{
		client: client,
	}, nil
}

func (s *SchemaRegistryClient) RegisterOrGetSchema(topic string, protoSchemaText string) (*srclient.Schema, error) {
	subject := topic + "-value"

	schema, err := s.client.CreateSchema(subject, protoSchemaText, srclient.Protobuf)
	if err != nil {
		return nil, fmt.Errorf("register schema for %s: %w", subject, err)
	}

	return schema, nil
}

func (s *SchemaRegistryClient) Serialize(ctx context.Context, schemaID int, message proto.Message) ([]byte, error) {
	data, err := proto.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("marshal protobuf: %w", err)
	}

	msgIndexes := s.getMessageIndexes(message)

	payload := make([]byte, 1+4+len(msgIndexes)+len(data))
	payload[0] = 0
	binary.BigEndian.PutUint32(payload[1:5], uint32(schemaID))
	copy(payload[5:5+len(msgIndexes)], msgIndexes)
	copy(payload[5+len(msgIndexes):], data)

	return payload, nil
}

func (s *SchemaRegistryClient) getMessageIndexes(message proto.Message) []byte {
	desc := message.ProtoReflect().Descriptor()

	var indexes []int
	for cur := desc; cur != nil; {
		indexes = append([]int{cur.Index()}, indexes...)
		if parent, ok := cur.Parent().(protoreflect.MessageDescriptor); ok {
			cur = parent
		} else {
			break
		}
	}

	if len(indexes) == 1 && indexes[0] == 0 {
		return []byte{0}
	}

	var buf []byte
	var tmp [binary.MaxVarintLen64]byte

	n := binary.PutVarint(tmp[:], int64(len(indexes)))
	buf = append(buf, tmp[:n]...)

	for _, idx := range indexes {
		n = binary.PutVarint(tmp[:], int64(idx))
		buf = append(buf, tmp[:n]...)
	}

	return buf
}

func (s *SchemaRegistryClient) Deserialize(ctx context.Context, payload []byte, dest proto.Message) error {
	if len(payload) < 5 {
		return fmt.Errorf("payload too short (min 5 bytes)")
	}

	if payload[0] != 0 {
		return fmt.Errorf("invalid magic byte: %d (expected 0)", payload[0])
	}

	schemaID := binary.BigEndian.Uint32(payload[1:5])

	_, err := s.client.GetSchema(int(schemaID))
	if err != nil {
		return fmt.Errorf("failed to fetch/verify schema with ID %d: %w", schemaID, err)
	}

	indexOffset, err := s.readMessageIndexesOffset(payload[5:])
	if err != nil {
		return fmt.Errorf("parse message indexes: %w", err)
	}

	protoDataOffset := 5 + indexOffset
	if protoDataOffset > len(payload) {
		return fmt.Errorf("invalid payload bounds")
	}

	if err := proto.Unmarshal(payload[protoDataOffset:], dest); err != nil {
		return fmt.Errorf("unmarshal protobuf data: %w", err)
	}

	return nil
}

func (s *SchemaRegistryClient) readMessageIndexesOffset(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, fmt.Errorf("empty data")
	}

	val, n := binary.Varint(data)
	if n <= 0 {
		return 0, fmt.Errorf("failed to read index length varint")
	}

	if val == 0 {
		return n, nil
	}

	offset := n
	for i := 0; i < int(val); i++ {
		if offset >= len(data) {
			return 0, fmt.Errorf("unexpected EOF reading indexes")
		}
		_, n := binary.Varint(data[offset:])
		if n <= 0 {
			return 0, fmt.Errorf("failed to read index varint at offset %d", offset)
		}
		offset += n
	}

	return offset, nil
}

func (s *SchemaRegistryClient) Close() {}

func (s *SchemaRegistryClient) Ping(ctx context.Context) error {
	_, err := s.client.GetSubjects()
	if err != nil {
		return fmt.Errorf("ping schema registry: %w", err)
	}
	return nil
}
