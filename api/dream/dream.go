// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package dream

import (
	"context"

	"dm-server/api/dream/v1"
)

type IDreamV1 interface {
	ChatWebSocket(ctx context.Context, req *v1.ChatWebSocketReq) (res *v1.ChatWebSocketRes, err error)
}
