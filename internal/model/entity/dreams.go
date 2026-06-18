// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// Dreams is the golang structure for table dreams.
type Dreams struct {
	Id        uint64      `json:"id"        orm:"id"         description:"Dream ID"`                                         // Dream ID
	UserId    uint64      `json:"userId"    orm:"user_id"    description:"Owner user ID"`                                    // Owner user ID
	Title     string      `json:"title"     orm:"title"      description:"Dream title"`                                      // Dream title
	Content   string      `json:"content"   orm:"content"    description:"Dream content"`                                    // Dream content
	DreamDate *gtime.Time `json:"dreamDate" orm:"dream_date" description:"Date the dream occurred"`                          // Date the dream occurred
	Tags      string      `json:"tags"      orm:"tags"       description:"Comma-separated tag list"`                         // Comma-separated tag list
	Symbols   string      `json:"symbols"   orm:"symbols"    description:"Comma-separated standard dream symbols"`           // Comma-separated standard dream symbols
	CreatedAt *gtime.Time `json:"createdAt" orm:"created_at" description:"Created at"`                                       // Created at
	UpdatedAt *gtime.Time `json:"updatedAt" orm:"updated_at" description:"Updated at"`                                       // Updated at
	DeletedAt *gtime.Time `json:"deletedAt" orm:"deleted_at" description:"Deleted at, NULL means not deleted"`               // Deleted at, NULL means not deleted
	Status    string      `json:"status"    orm:"status"     description:"Dream status: pending/processing/completed/error"` // Dream status: pending/processing/completed/error
}
