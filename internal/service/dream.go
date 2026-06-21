// ================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// You can delete these comments if you wish manually maintain this interface file.
// ================================================================================

package service

import (
	"context"
	"dm-server/internal/model"
)

type (
	IDream interface {
		ExtractDreamSymbols(ctx context.Context, content string, emotionTags []string) ([]string, error)
		SinkDreamSymbolCache(ctx context.Context, userId string, symbols []string, interpretation string, sourceDreamId string) error
		// StreamDream Real-time streaming dream analysis
		StreamDream(ctx context.Context, content string) (<-chan model.DreamStreamEvent, error)
	}
)

var (
	localDream IDream
)

func Dream() IDream {
	if localDream == nil {
		panic("implement not found for interface IDream, forgot register?")
	}
	return localDream
}

func RegisterDream(i IDream) {
	localDream = i
}
