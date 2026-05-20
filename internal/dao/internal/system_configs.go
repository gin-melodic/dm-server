// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SystemConfigsDao is the data access object for the table system_configs.
type SystemConfigsDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  SystemConfigsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// SystemConfigsColumns defines and stores column names for the table system_configs.
type SystemConfigsColumns struct {
	Key       string // Config key
	Value     string // Config value (JSON or plain text)
	UpdatedAt string // Updated at
}

// systemConfigsColumns holds the columns for the table system_configs.
var systemConfigsColumns = SystemConfigsColumns{
	Key:       "key",
	Value:     "value",
	UpdatedAt: "updated_at",
}

// NewSystemConfigsDao creates and returns a new DAO object for table data access.
func NewSystemConfigsDao(handlers ...gdb.ModelHandler) *SystemConfigsDao {
	return &SystemConfigsDao{
		group:    "default",
		table:    "system_configs",
		columns:  systemConfigsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SystemConfigsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SystemConfigsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SystemConfigsDao) Columns() SystemConfigsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SystemConfigsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SystemConfigsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SystemConfigsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
