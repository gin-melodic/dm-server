// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// Dreams is the golang structure of table dreams for DAO operations like Where/Data.
type Dreams struct {
	g.Meta    `orm:"table:dreams, do:true"`
	Id        any         // Dream ID
	UserId    any         // Owner user ID
	Title     any         // Dream title
	Content   any         // Dream content
	DreamDate *gtime.Time // Date the dream occurred
	Tags      any         // Comma-separated tag list
	Symbols   any         // Comma-separated standard dream symbols
	CreatedAt *gtime.Time // Created at
	UpdatedAt *gtime.Time // Updated at
	DeletedAt *gtime.Time // Deleted at, NULL means not deleted
	Status    any         // Dream status: pending/processing/completed/error
}
