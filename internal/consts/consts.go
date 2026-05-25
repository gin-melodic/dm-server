package consts

// ContextKey Define Context key type to avoid conflicts caused by using built-in string type
type ContextKey string

type CtxUser struct {
	Id          uint64 // User ID
	OpenID      string // User OpenID
	SupabaseUID string // Supabase Auth user UID
}

const (
	CtxUserId      ContextKey = "userId"
	CtxOpenId      ContextKey = "openId"
	CtxSupabaseUid ContextKey = "supabaseUid"
)

const (
	PromptDreamAnalysis = "dream_analysis"
)
