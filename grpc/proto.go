package grpc

import (
	"fmt"

	"google.golang.org/protobuf/proto"
)

func protoMarshalAppend(data []byte, v any) ([]byte, error) {
	msg, ok := v.(proto.Message)
	if !ok {
		return data, fmt.Errorf("proto: error marshalling data: expected proto.Message, got %T", v)
	}
	data, err := proto.MarshalOptions{}.MarshalAppend(data, msg)
	if err != nil {
		return data, fmt.Errorf("proto: error marshalling data: %w", err)
	}
	return data, nil
}

func protoUnmarshal(data []byte, v any) error {
	msg, ok := v.(proto.Message)
	if !ok {
		return fmt.Errorf("proto: error unmarshalling data: expected proto.Message, got %T", v)
	}
	if err := proto.Unmarshal(data, msg); err != nil {
		return fmt.Errorf("proto: error unmarshalling data: %w", err)
	}
	return nil
}
