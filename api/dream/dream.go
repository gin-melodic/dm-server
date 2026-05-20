// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package dream

import (
	"context"

	v1 "dm-server/api/dream/v1"
)

type IDreamV1 interface {
	ChatWs(ctx context.Context, req *v1.ChatWebSocketReq) (res *v1.ChatWebSocketRes, err error)
}
