package consts

// ContextKey Define Context key type to avoid conflicts caused by using built-in string type
type ContextKey string

type CtxUser struct {
	Id          uint64 // User ID
	OpenID      string // User OpenID
	SupabaseUID string // Supabase Auth user UID
}

type DreamStreamMetadata struct {
	SymbolsDetected []string
	SymbolsMatched  []string
	InferenceLevel  string
}

type DreamRelatedContextItem struct {
	Id         uint64
	Date       string
	Summary    string
	Emotion    string
	Symbols    []string
	Similarity float64
}

const (
	CtxUserId              ContextKey = "userId"
	CtxOpenId              ContextKey = "openId"
	CtxSupabaseUid         ContextKey = "supabaseUid"
	CtxDreamEmotionTags    ContextKey = "dreamEmotionTags"
	CtxDreamStreamMetadata ContextKey = "dreamStreamMetadata"
	CtxDreamRelatedContext ContextKey = "dreamRelatedContext"
	CtxDreamResponseLocale ContextKey = "dreamResponseLocale"
)

const (
	PromptDreamAnalysis = "dream_analysis"
)
