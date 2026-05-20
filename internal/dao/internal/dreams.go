// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// DreamsDao is the data access object for the table dreams.
type DreamsDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  DreamsColumns      // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// DreamsColumns defines and stores column names for the table dreams.
type DreamsColumns struct {
	Id        string // Dream ID
	UserId    string // Owner user ID
	Title     string // Dream title
	Content   string // Dream content
	DreamDate string // Date the dream occurred
	Tags      string // Comma-separated tag list
	CreatedAt string // Created at
	UpdatedAt string // Updated at
	DeletedAt string // Deleted at, NULL means not deleted
	Status    string // Dream status: pending/processing/completed/error
}

// dreamsColumns holds the columns for the table dreams.
var dreamsColumns = DreamsColumns{
	Id:        "id",
	UserId:    "user_id",
	Title:     "title",
	Content:   "content",
	DreamDate: "dream_date",
	Tags:      "tags",
	CreatedAt: "created_at",
	UpdatedAt: "updated_at",
	DeletedAt: "deleted_at",
	Status:    "status",
}

// NewDreamsDao creates and returns a new DAO object for table data access.
func NewDreamsDao(handlers ...gdb.ModelHandler) *DreamsDao {
	return &DreamsDao{
		group:    "default",
		table:    "dreams",
		columns:  dreamsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *DreamsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *DreamsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *DreamsDao) Columns() DreamsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *DreamsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *DreamsDao) Ctx(ctx context.Context) *gdb.Model {
	model := dao.DB().Model(dao.table)
	for _, handler := range dao.handlers {
		model = handler(model)
	}
	return model.Safe().Ctx(ctx)
}

// Transaction wraps the transaction logic using function f.
// It rolls back the transaction and returns the error if function f returns a non-nil error.
// It commits the transaction and returns nil if function f returns nil.
//
// Note: Do not commit or roll back the transaction in function f,
// as it is automatically handled by this function.
func (dao *DreamsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
