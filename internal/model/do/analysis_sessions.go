// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// AnalysisSessions is the golang structure of table analysis_sessions for DAO operations like Where/Data.
type AnalysisSessions struct {
	g.Meta        `orm:"table:analysis_sessions, do:true"`
	Id            any         // Session ID
	DreamId       any         // Associated dream ID
	SessionUuid   any         // Analysis session UUID
	AnalysisType  any         // Analysis type, e.g. psychology
	Status        any         // Status: pending/processing/completed/error
	Progress      any         // Progress percentage
	ResultSummary any         // Analysis summary (stored upon completion)
	CreatedAt     *gtime.Time // Created at
	UpdatedAt     *gtime.Time // Updated at
	DeletedAt     *gtime.Time // Deleted at, NULL means not deleted
}
