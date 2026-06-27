package v1

import "github.com/gogf/gf/v2/frame/g"

// Fetch dream list by date range or paginated latest records.
type FetchDreamListReq struct {
	g.Meta       `path:"/v1/dream/list" method:"get" summary:"Get the list of dreams" tags:"Dream"`
	StartDate    string `json:"startDate" form:"startDate" v:"date" example:"2022-01-01" dc:"Start date (inclusive)"`
	EndDate      string `json:"endDate" form:"endDate" v:"date" example:"2022-12-31" dc:"End date (inclusive)"`
	Page         int    `json:"page" form:"page" default:"1" example:"1" dc:"Current page (1-based)"`
	PageSize     int    `json:"pageSize" form:"pageSize" default:"10" example:"10" dc:"Page size"`
	Keyword      string `json:"keyword" form:"keyword" dc:"Search keyword for title, content, analysis, or keywords"`
	Emotion      string `json:"emotion" form:"emotion" dc:"Dream emotion"`
	FavoriteOnly bool   `json:"favoriteOnly" form:"favoriteOnly" dc:"Only return favorite dreams"`
}

type FetchDreamListRes struct {
	Items    []DreamRecord `json:"items"`
	Total    int64         `json:"total"`
	Page     int           `json:"page,omitempty"`
	PageSize int           `json:"pageSize,omitempty"`
	HasMore  bool          `json:"hasMore,omitempty"`
}

type DreamRecord struct {
	Id              uint64   `json:"id"`
	Title           string   `json:"title"`
	Content         string   `json:"content"`
	Interpretation  string   `json:"interpretation"`
	Emotion         string   `json:"emotion"`
	Keywords        []string `json:"keywords"`
	Symbols         []string `json:"symbols"`
	ConfidenceScore float64  `json:"confidenceScore"`
	IsFavorite      bool     `json:"isFavorite"`
	CreatedAt       string   `json:"createdAt"`
	UpdatedAt       string   `json:"updatedAt"`
}

// Get dream detail by id
type GetDreamReq struct {
	g.Meta `path:"/v1/dream/detail" method:"get" summary:"Get Dream Detail" tags:"Dream"`
	Id     uint64 `json:"id" form:"id" v:"required" example:"1" dc:"The id of the dream"`
	// TODO: Return one dream record by id.
}

type GetDreamRes struct {
	DreamRecord
	RelatedDreams []RelatedDream `json:"relatedDreams"`
	Insight       string         `json:"insight"`
}

// Update dream by id
type UpdateDreamReq struct {
	g.Meta     `path:"/v1/dream/update" method:"put" summary:"Update Dream" tags:"Dream"`
	Id         uint64 `json:"id" v:"required" example:"1" dc:"The id of the dream"`
	Title      string `json:"title" dc:"Dream title"`
	Content    string `json:"content" dc:"Dream content"`
	Emotion    string `json:"emotion" dc:"Dream emotion"`
	IsFavorite *bool  `json:"isFavorite" dc:"Whether this dream is marked as favorite"`
	// TODO: Update dream metadata and content.
}

type UpdateDreamRes = DreamRecord

// Delete dream by id
type DeleteDreamReq struct {
	g.Meta `path:"/v1/dream/delete" method:"post" summary:"Delete the dream" tags:"Dream"`
	Id     uint64 `json:"id" form:"id" v:"required" example:"1" dc:"The id of the dream"`
}

type DeleteDreamRes struct {
	Success bool `json:"success"`
}

// Get dream anaylyze result by id
type GetDreamAnalyzeResultReq struct {
	g.Meta `path:"/v1/dream/analyze/result" method:"get" summary:"Get the analyze result of the dream" tags:"Dream"`
	Id     uint64 `json:"id" form:"id" v:"required" example:"1" dc:"The id of the dream"`
}

type GetDreamAnalyzeResultRes struct {
	Result string `json:"result"`
}

// Create dream analysis
type CreateDreamAnalysisReq struct {
	g.Meta       `path:"/v1/dream/analyze" method:"post" summary:"Create Dream Analysis" tags:"Dream"`
	Content      string `json:"content" v:"required" dc:"Dream content to analyze"`
	Emotion      string `json:"emotion" dc:"Dream emotion"`
	Locale       string `json:"locale" dc:"Response locale"`
	AnalysisText string `json:"analysisText" dc:"Completed analysis text to persist without re-running the model"`
	// TODO: Analyze dream content and persist the dream plus analysis session.
}

type DreamAnalysisResult struct {
	Summary         string   `json:"summary"`
	Interpretation  string   `json:"interpretation"`
	Keywords        []string `json:"keywords"`
	Symbols         []string `json:"symbols"`
	ConfidenceScore float64  `json:"confidenceScore"`
	Locale          string   `json:"locale"`
}

type RelatedDream struct {
	Id          uint64   `json:"id"`
	Date        string   `json:"date"`
	Similarity  float64  `json:"similarity"`
	EmotionTags []string `json:"emotionTags"`
	Symbols     []string `json:"symbols"`
	Summary     string   `json:"summary"`
}

type DreamAnalysisStep struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type CreateDreamAnalysisRes struct {
	Dream         *DreamRecord         `json:"dream"`
	Analysis      *DreamAnalysisResult `json:"analysis"`
	Steps         []DreamAnalysisStep  `json:"steps"`
	RelatedDreams []RelatedDream       `json:"relatedDreams"`
	Insight       string               `json:"insight"`
}

// Set dream favorite status
type SetDreamFavoriteReq struct {
	g.Meta     `path:"/v1/dream/favorite" method:"patch" summary:"Set Dream Favorite" tags:"Dream"`
	Id         uint64 `json:"id" v:"required" example:"1" dc:"The id of the dream"`
	IsFavorite bool   `json:"isFavorite" dc:"Whether this dream is marked as favorite"`
	// TODO: Set a dream record's favorite status.
}

type SetDreamFavoriteRes = DreamRecord

// Get dream home data
type GetDreamHomeReq struct {
	g.Meta `path:"/v1/dream/home" method:"get" summary:"Get Dream Home" tags:"Dream"`
	// TODO: Return home screen dream stats and recent dreams.
}

type DreamRecommendation struct {
	Dream *DreamRecord `json:"dream"`
	Score float64      `json:"score"`
	Tier  string       `json:"tier"`
}

type DreamHome struct {
	TotalDreams       int                  `json:"totalDreams"`
	CurrentStreakDays int                  `json:"currentStreakDays"`
	Recommendation    *DreamRecommendation `json:"recommendation"`
	EmotionWaves      []EmotionWavePoint   `json:"emotionWaves"`
	RecentDreams      []DreamRecord        `json:"recentDreams"`
}

type EmotionWavePoint struct {
	Date    string `json:"date"`
	Emotion string `json:"emotion"`
	Count   int    `json:"count"`
}

type GetDreamHomeRes = DreamHome

// Get today's dream recommendation
type GetTodayDreamRecommendationReq struct {
	g.Meta `path:"/v1/dream/recommendation/today" method:"get" summary:"Get Today's Dream Recommendation" tags:"Dream"`
	// TODO: Return today's dream recommendation.
}

type GetTodayDreamRecommendationRes = DreamRecommendation
