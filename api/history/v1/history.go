package v1

import "github.com/gogf/gf/v2/frame/g"

// Fetch dream list in date range (pagination)
type FetchDreamListReq struct {
	g.Meta    `path:"/dream/list" method:"get" summary:"Get the list of dreams" tags:"Dream"`
	StartDate string `json:"startDate" form:"startDate" v:"required|date" example:"2022-01-01" dc:"Start date (inclusive)"`
	EndDate   string `json:"endDate" form:"endDate" v:"required|date" example:"2022-12-31" dc:"End date (inclusive)"`
	PageSize  int    `json:"pageSize" form:"pageSize" default:"10" example:"10" dc:"Page size"`
	Page      int    `json:"page" form:"page" default:"1" example:"1" dc:"Current page (1-based)"`
}

type DreamSummary struct {
	Id         uint64 `json:"id"`
	Title      string `json:"title"`
	Summary    string `json:"summary"`
	CreateTime string `json:"createTime"`
}

type FetchDreamListRes struct {
	Data     []DreamSummary `json:"data"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
	Total    int64          `json:"total"`
}

// Delete dream by id
type DeleteDreamReq struct {
	g.Meta `path:"/dream/delete" method:"post" summary:"Delete the dream" tags:"Dream"`
	Id     uint64 `json:"id" form:"id" v:"required" example:"1" dc:"The id of the dream"`
}

type DeleteDreamRes struct {
	Success bool `json:"success"`
}

// Get dream anaylyze result by id
type GetDreamAnalyzeResultReq struct {
	g.Meta `path:"/dream/analyze/result" method:"get" summary:"Get the analyze result of the dream" tags:"Dream"`
	Id     uint64 `json:"id" form:"id" v:"required" example:"1" dc:"The id of the dream"`
}

type GetDreamAnalyzeResultRes struct {
	Result string `json:"result"`
}
