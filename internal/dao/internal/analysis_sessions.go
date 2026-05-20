// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// AnalysisSessionsDao is the data access object for the table analysis_sessions.
type AnalysisSessionsDao struct {
	table    string                  // table is the underlying table name of the DAO.
	group    string                  // group is the database configuration group name of the current DAO.
	columns  AnalysisSessionsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler      // handlers for customized model modification.
}

// AnalysisSessionsColumns defines and stores column names for the table analysis_sessions.
type AnalysisSessionsColumns struct {
	Id            string // Session ID
	DreamId       string // Associated dream ID
	SessionUuid   string // Analysis session UUID
	AnalysisType  string // Analysis type, e.g. psychology
	Status        string // Status: pending/processing/completed/error
	Progress      string // Progress percentage
	ResultSummary string // Analysis summary (stored upon completion)
	CreatedAt     string // Created at
	UpdatedAt     string // Updated at
	DeletedAt     string // Deleted at, NULL means not deleted
}

// analysisSessionsColumns holds the columns for the table analysis_sessions.
var analysisSessionsColumns = AnalysisSessionsColumns{
	Id:            "id",
	DreamId:       "dream_id",
	SessionUuid:   "session_uuid",
	AnalysisType:  "analysis_type",
	Status:        "status",
	Progress:      "progress",
	ResultSummary: "result_summary",
	CreatedAt:     "created_at",
	UpdatedAt:     "updated_at",
	DeletedAt:     "deleted_at",
}

// NewAnalysisSessionsDao creates and returns a new DAO object for table data access.
func NewAnalysisSessionsDao(handlers ...gdb.ModelHandler) *AnalysisSessionsDao {
	return &AnalysisSessionsDao{
		group:    "default",
		table:    "analysis_sessions",
		columns:  analysisSessionsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *AnalysisSessionsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *AnalysisSessionsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *AnalysisSessionsDao) Columns() AnalysisSessionsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *AnalysisSessionsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *AnalysisSessionsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *AnalysisSessionsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
