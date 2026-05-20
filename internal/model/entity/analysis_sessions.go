// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// AnalysisSessions is the golang structure for table analysis_sessions.
type AnalysisSessions struct {
	Id            uint64      `json:"id"            orm:"id"             description:"Session ID"`                                 // Session ID
	DreamId       uint64      `json:"dreamId"       orm:"dream_id"       description:"Associated dream ID"`                        // Associated dream ID
	SessionUuid   string      `json:"sessionUuid"   orm:"session_uuid"   description:"Analysis session UUID"`                      // Analysis session UUID
	AnalysisType  string      `json:"analysisType"  orm:"analysis_type"  description:"Analysis type, e.g. psychology"`             // Analysis type, e.g. psychology
	Status        string      `json:"status"        orm:"status"         description:"Status: pending/processing/completed/error"` // Status: pending/processing/completed/error
	Progress      uint        `json:"progress"      orm:"progress"       description:"Progress percentage"`                        // Progress percentage
	ResultSummary string      `json:"resultSummary" orm:"result_summary" description:"Analysis summary (stored upon completion)"`  // Analysis summary (stored upon completion)
	CreatedAt     *gtime.Time `json:"createdAt"     orm:"created_at"     description:"Created at"`                                 // Created at
	UpdatedAt     *gtime.Time `json:"updatedAt"     orm:"updated_at"     description:"Updated at"`                                 // Updated at
	DeletedAt     *gtime.Time `json:"deletedAt"     orm:"deleted_at"     description:"Deleted at, NULL means not deleted"`         // Deleted at, NULL means not deleted
}
