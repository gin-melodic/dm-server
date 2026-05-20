// ================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// You can delete these comments if you wish manually maintain this interface file.
// ================================================================================

package service

import (
	"context"
)

type (
	IDream interface {
		// StreamDream Real-time streaming dream analysis
		StreamDream(ctx context.Context, content string) (<-chan string, error)
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
