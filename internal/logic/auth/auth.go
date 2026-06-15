package auth

import (
	"bytes"
	"context"
	"database/sql"
	v1 "dm-server/api/user/v1"
	"dm-server/internal/consts"
	"dm-server/internal/dao"
	"dm-server/internal/model"
	"dm-server/internal/model/entity"
	"dm-server/internal/service"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/golang-jwt/jwt/v5"
)

type sAuth struct{}

var (
	allowedPrivacyModes = map[string]struct{}{
		"private":    {},
		"cloud_sync": {},
	}
	allowedStorageModes = map[string]struct{}{
		"local_cache": {},
		"cloud_sync":  {},
	}
)

type psycheDreamRow struct {
	Id              uint64      `orm:"id"`
	Tags            string      `orm:"tags"`
	Emotion         string      `orm:"emotion"`
	ConfidenceScore float64     `orm:"confidence_score"`
	CreatedAt       *gtime.Time `orm:"created_at"`
}

type supabaseAuthUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type supabasePasswordAuthResult struct {
	AccessToken string            `json:"access_token"`
	User        *supabaseAuthUser `json:"user"`
}

type supabaseAuthErrorResponse struct {
	Error       string      `json:"error"`
	ErrorCode   string      `json:"error_code"`
	Message     string      `json:"message"`
	Msg         string      `json:"msg"`
	Description string      `json:"error_description"`
	Code        interface{} `json:"code"`
}

func New() *sAuth {
	return &sAuth{}
}

func init() {
	service.RegisterAuth(New())
}

// ─────────────────────────────────────────────────────────
// WeChat Login
// ─────────────────────────────────────────────────────────
func (s *sAuth) WechatLogin(ctx context.Context, req *v1.WechatAuthReq) (*v1.WechatAuthRes, error) {
	// get openid from code
	glog.Infof(ctx, "Get openid from code: %s", req.Code)
	openid, err := s.getOpenidFromWechat(ctx, req.Code)
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to get openid")
	}

	// find or create user (only openid)
	user, err := s.findOrCreateWechatUser(ctx, openid)
	if err != nil {
		return nil, gerror.Wrap(err, "Find or Create User failed")
	}

	// generate JWT token
	token, err := s.generateServerJWT(ctx, user.Id, "", openid, "wechat")
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to generate JWT token")
	}
	glog.Infof(ctx, "Generate JWT token: %s", token)

	return &v1.WechatAuthRes{
		OpenId:   openid,
		Token:    token,
		UserInfo: &v1.UserInfo{Id: user.Id, OpenId: user.Openid},
	}, nil
}

// ─────────────────────────────────────────────────────────
// Email Login via backend-managed Supabase password flow.
// ─────────────────────────────────────────────────────────
func (s *sAuth) EmailLogin(ctx context.Context, req *v1.EmailAuthReq) (*v1.EmailAuthRes, error) {
	return s.emailPasswordLogin(ctx, req)
}

func (s *sAuth) emailPasswordLogin(ctx context.Context, req *v1.EmailAuthReq) (*v1.EmailAuthRes, error) {
	email := strings.TrimSpace(req.Email)
	password := req.Password
	if email == "" || password == "" {
		return nil, gerror.New("email/password is required")
	}
	if !isLikelyEmail(email) {
		return nil, gerror.New("invalid email")
	}
	if len(password) < 6 {
		return nil, gerror.New("password must be at least 6 characters")
	}

	authResult, err := s.signInWithSupabasePassword(ctx, email, password)
	authFlow := "signedIn"
	if err != nil {
		if !isInvalidSupabasePasswordLogin(err) {
			return nil, err
		}
		authResult, err = s.signUpWithSupabasePassword(ctx, email, password)
		authFlow = "signedUp"
		if err != nil {
			if isSupabaseUserAlreadyRegistered(err) {
				return nil, gerror.New("invalid email or password")
			}
			return nil, err
		}
	}
	if strings.TrimSpace(authResult.AccessToken) == "" {
		return &v1.EmailAuthRes{
			AuthFlow:                  authFlow,
			EmailVerificationRequired: true,
			UserInfo:                  supabaseAuthUserInfo(authResult.User, email),
		}, nil
	}
	if authResult.User == nil || strings.TrimSpace(authResult.User.ID) == "" {
		return nil, gerror.New("Supabase auth response missing user")
	}

	userEmail := email
	if strings.TrimSpace(authResult.User.Email) != "" {
		userEmail = strings.TrimSpace(authResult.User.Email)
	}

	user, err := s.findOrCreateEmailUser(ctx, authResult.User.ID, userEmail)
	if err != nil {
		return nil, gerror.Wrap(err, "Find or Create User failed")
	}

	token, err := s.generateServerJWT(ctx, user.Id, authResult.User.ID, "", "email")
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to generate server JWT")
	}

	return &v1.EmailAuthRes{
		Token:    token,
		AuthFlow: authFlow,
		UserInfo: &v1.UserInfo{
			Id:    user.Id,
			Email: userEmail,
		},
	}, nil
}

// GetUserInfo Get user information
// Used after login to get detailed user info
func (s *sAuth) GetUserInfo(ctx context.Context, req *v1.GetUserInfoReq) (*v1.GetUserInfoRes, error) {
	userId := ctx.Value(consts.CtxUserId)
	if userId == nil {
		return nil, gerror.New("User not logged in")
	}
	userIdUint64, ok := userId.(uint64)
	if !ok {
		return nil, gerror.New("Invalid user ID type")
	}

	// find user by id
	var user *entity.Users
	err := dao.Users.Ctx(ctx).Where("id", userIdUint64).Where("deleted_at IS NULL").Scan(&user)
	if err != nil {
		return nil, gerror.Wrap(err, "Database error")
	}
	if user == nil {
		return nil, gerror.New("User does not exist")
	}
	return &v1.GetUserInfoRes{
		UserInfo: &v1.UserInfo{
			Id:       user.Id,
			OpenId:   user.Openid,
			Email:    user.Email,
			Nickname: user.Nickname,
			Avatar:   user.AvatarUrl,
		},
	}, nil
}

// UpdateUserInfo Update user information
func (s *sAuth) UpdateUserInfo(ctx context.Context, req *v1.UpdateUserInfoReq) (*v1.UpdateUserInfoRes, error) {
	userId := ctx.Value(consts.CtxUserId)
	if userId == nil {
		return nil, gerror.New("User not logged in")
	}
	userIdUint64, ok := userId.(uint64)
	if !ok {
		return nil, gerror.Newf("Invalid user ID type, %v", userId)
	}
	updateData := g.Map{}
	if requestJSONHasKey(ctx, "nickname") || req.Nickname != "" {
		updateData["nickname"] = req.Nickname
	}
	if requestJSONHasKey(ctx, "avatar_url") || req.Avatar != "" {
		updateData["avatar_url"] = req.Avatar
	}
	if len(updateData) > 0 {
		updateData["updated_at"] = gtime.Now().FormatTo("2006-01-02 15:04:05")
		_, err := dao.Users.Ctx(ctx).Data(updateData).Where("id", userIdUint64).Update()
		if err != nil {
			return nil, gerror.Wrap(err, "Failed to update user info")
		}
	}
	var user *entity.Users
	err := dao.Users.Ctx(ctx).Where("id", userIdUint64).Scan(&user)
	if err != nil {
		return nil, gerror.Wrap(err, "Database error")
	}
	if user == nil {
		return nil, gerror.New("User does not exist")
	}
	return &v1.UpdateUserInfoRes{
		UserInfo: &v1.UserInfo{
			Id:       user.Id,
			OpenId:   user.Openid,
			Email:    user.Email,
			Nickname: user.Nickname,
			Avatar:   user.AvatarUrl,
		},
	}, nil
}

func (s *sAuth) GetUserSettings(ctx context.Context, req *v1.GetUserSettingsReq) (*v1.GetUserSettingsRes, error) {
	userID, err := getAuthContextUserID(ctx)
	if err != nil {
		return nil, err
	}
	settings, err := loadUserSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	res := v1.GetUserSettingsRes(*settings)
	return &res, nil
}

func (s *sAuth) UpdateUserSettings(ctx context.Context, req *v1.UpdateUserSettingsReq) (*v1.UpdateUserSettingsRes, error) {
	userID, err := getAuthContextUserID(ctx)
	if err != nil {
		return nil, err
	}
	settings, err := loadUserSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	if req.Language != "" {
		if !isValidLanguageTag(req.Language) {
			return nil, gerror.New("invalid language")
		}
		settings.Language = req.Language
	}
	if req.PrivacyMode != "" {
		if !isAllowedUserSetting(req.PrivacyMode, allowedPrivacyModes) {
			return nil, gerror.New("invalid privacy mode")
		}
		settings.PrivacyMode = req.PrivacyMode
	}
	if req.DreamReminderEnabled != nil {
		settings.DreamReminderEnabled = req.DreamReminderEnabled
		if !*req.DreamReminderEnabled {
			settings.DreamReminderTime = ""
		}
	}
	if req.DreamReminderTime != "" {
		if !isValidReminderTime(req.DreamReminderTime) {
			return nil, gerror.New("invalid dream reminder time")
		}
		settings.DreamReminderTime = req.DreamReminderTime
	}
	if req.StorageMode != "" {
		if !isAllowedUserSetting(req.StorageMode, allowedStorageModes) {
			return nil, gerror.New("invalid storage mode")
		}
		settings.StorageMode = req.StorageMode
	}
	if err := saveUserSettings(ctx, userID, settings); err != nil {
		return nil, err
	}
	res := v1.UpdateUserSettingsRes(*settings)
	return &res, nil
}

func (s *sAuth) GetPsycheProfile(ctx context.Context, req *v1.GetPsycheProfileReq) (*v1.GetPsycheProfileRes, error) {
	userID, err := getAuthContextUserID(ctx)
	if err != nil {
		return nil, err
	}
	var dreams []psycheDreamRow
	err = g.DB().Model("dreams").
		Fields("id, tags, emotion, confidence_score, created_at").
		Where("user_id = ? AND deleted_at IS NULL AND status = ?", userID, "completed").
		OrderDesc("created_at").
		Scan(&dreams)
	if err != nil {
		return nil, gerror.Wrap(err, "failed to query psyche profile dreams")
	}
	score, level, description := psycheIntegration(len(dreams))
	archetypes, dominant := psycheArchetypes(dreams, score)
	profile := v1.GetPsycheProfileRes{
		IntegrationScore:       score,
		IntegrationLevel:       level,
		IntegrationDescription: description,
		Archetypes:             archetypes,
		DominantArchetype:      dominant,
		UpdatedAt:              gtime.Now().Format("Y-m-d H:i:s"),
	}
	return &profile, nil
}

// ─────────────────────────────────────────────────────────
// Internal helpers
// ─────────────────────────────────────────────────────────

func requestJSONHasKey(ctx context.Context, key string) bool {
	body := requestJSONBody(ctx)
	if len(body) == 0 {
		return false
	}
	_, ok := body[key]
	return ok
}

func requestJSONBody(ctx context.Context) map[string]json.RawMessage {
	r := g.RequestFromCtx(ctx)
	if r == nil {
		return nil
	}
	body := r.GetBody()
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	return payload
}

func (s *sAuth) signInWithSupabasePassword(ctx context.Context, email, password string) (*supabasePasswordAuthResult, error) {
	projectURL, apiKey, err := supabaseAuthConfig(ctx)
	if err != nil {
		return nil, err
	}
	return postSupabasePasswordAuth(ctx, projectURL+"/auth/v1/token?grant_type=password", apiKey, g.Map{
		"email":    email,
		"password": password,
	})
}

func (s *sAuth) signUpWithSupabasePassword(ctx context.Context, email, password string) (*supabasePasswordAuthResult, error) {
	projectURL, apiKey, err := supabaseAuthConfig(ctx)
	if err != nil {
		return nil, err
	}
	return postSupabasePasswordAuth(ctx, projectURL+"/auth/v1/signup", apiKey, g.Map{
		"email":    email,
		"password": password,
	})
}

func supabaseAuthConfig(ctx context.Context) (string, string, error) {
	projectURL := strings.TrimRight(g.Cfg().MustGet(ctx, "supabase.project_url").String(), "/")
	apiKey := strings.TrimSpace(g.Cfg().MustGet(ctx, "supabase.publishable_key").String())
	if apiKey == "" {
		apiKey = strings.TrimSpace(g.Cfg().MustGet(ctx, "supabase.secret_key").String())
	}
	if projectURL == "" || apiKey == "" {
		return "", "", gerror.New("Supabase auth is not configured")
	}
	return projectURL, apiKey, nil
}

func postSupabasePasswordAuth(ctx context.Context, url, apiKey string, payload g.Map) (*supabasePasswordAuthResult, error) {
	requestBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to encode Supabase auth request")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(requestBytes))
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to build Supabase auth request")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("apikey", apiKey)
	request.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to request Supabase auth")
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to read Supabase auth response")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, gerror.New(parseSupabaseAuthError(body, response.StatusCode))
	}

	var result supabasePasswordAuthResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, gerror.Wrap(err, "Failed to parse Supabase auth response")
	}
	return &result, nil
}

func parseSupabaseAuthError(body []byte, statusCode int) string {
	var errResp supabaseAuthErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil {
		for _, value := range []string{
			errResp.ErrorCode,
			errResp.Message,
			errResp.Msg,
			errResp.Description,
			errResp.Error,
		} {
			value = strings.TrimSpace(value)
			if value != "" {
				return value
			}
		}
	}
	return fmt.Sprintf("Supabase auth request failed with status %d", statusCode)
}

func isInvalidSupabasePasswordLogin(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "invalid login credentials") ||
		strings.Contains(message, "invalid_credentials")
}

func isSupabaseUserAlreadyRegistered(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "user already registered") ||
		strings.Contains(message, "user_already_exists") ||
		strings.Contains(message, "email_exists")
}

func supabaseAuthUserInfo(user *supabaseAuthUser, fallbackEmail string) *v1.UserInfo {
	email := strings.TrimSpace(fallbackEmail)
	if user != nil && strings.TrimSpace(user.Email) != "" {
		email = strings.TrimSpace(user.Email)
	}
	return &v1.UserInfo{Email: email}
}

func isLikelyEmail(value string) bool {
	matched, _ := regexp.MatchString(`^[^@\s]+@[^@\s]+\.[^@\s]+$`, strings.TrimSpace(value))
	return matched
}

func getAuthContextUserID(ctx context.Context) (uint64, error) {
	userId := ctx.Value(consts.CtxUserId)
	if userId == nil {
		return 0, gerror.New("User not logged in")
	}
	userIdUint64, ok := userId.(uint64)
	if !ok {
		return 0, gerror.New("Invalid user ID type")
	}
	return userIdUint64, nil
}

func defaultUserSettings() *v1.UserSettings {
	enabled := false
	return &v1.UserSettings{
		Language:             "zh-CN",
		PrivacyMode:          "private",
		DreamReminderEnabled: &enabled,
		DreamReminderTime:    "",
		StorageMode:          "local_cache",
	}
}

func loadUserSettings(ctx context.Context, userID uint64) (*v1.UserSettings, error) {
	settings := defaultUserSettings()
	var row struct {
		Language             string `orm:"language"`
		PrivacyMode          string `orm:"privacy_mode"`
		DreamReminderEnabled *bool  `orm:"dream_reminder_enabled"`
		DreamReminderTime    string `orm:"dream_reminder_time"`
		StorageMode          string `orm:"storage_mode"`
	}
	err := g.DB().Model("user_settings").Where("user_id", userID).Scan(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return settings, nil
		}
		return nil, gerror.Wrap(err, "failed to load user settings")
	}
	if row.Language == "" && row.PrivacyMode == "" && row.StorageMode == "" {
		return settings, nil
	}
	if row.Language != "" {
		settings.Language = row.Language
	}
	if row.PrivacyMode != "" {
		settings.PrivacyMode = row.PrivacyMode
	}
	if row.DreamReminderEnabled != nil {
		settings.DreamReminderEnabled = row.DreamReminderEnabled
	}
	settings.DreamReminderTime = row.DreamReminderTime
	if row.StorageMode != "" {
		settings.StorageMode = row.StorageMode
	}
	return settings, nil
}

func saveUserSettings(ctx context.Context, userID uint64, settings *v1.UserSettings) error {
	enabled := false
	if settings.DreamReminderEnabled != nil {
		enabled = *settings.DreamReminderEnabled
	}
	_, err := g.DB().Exec(ctx, `
		INSERT INTO user_settings
			(user_id, language, privacy_mode, dream_reminder_enabled, dream_reminder_time, storage_mode, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (user_id) DO UPDATE SET
			language = EXCLUDED.language,
			privacy_mode = EXCLUDED.privacy_mode,
			dream_reminder_enabled = EXCLUDED.dream_reminder_enabled,
			dream_reminder_time = EXCLUDED.dream_reminder_time,
			storage_mode = EXCLUDED.storage_mode,
			updated_at = EXCLUDED.updated_at
	`, userID, settings.Language, settings.PrivacyMode, enabled, settings.DreamReminderTime, settings.StorageMode, gtime.Now().Format("Y-m-d H:i:s"))
	if err != nil {
		return gerror.Wrap(err, "failed to save user settings")
	}
	return nil
}

func isAllowedUserSetting(value string, allowed map[string]struct{}) bool {
	_, ok := allowed[strings.TrimSpace(value)]
	return ok
}

func isValidLanguageTag(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	matched, _ := regexp.MatchString(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$`, value)
	return matched
}

func isValidReminderTime(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	matched, _ := regexp.MatchString(`^([01]\d|2[0-3]):[0-5]\d$`, value)
	return matched
}

func psycheIntegration(total int) (float64, string, string) {
	switch {
	case total == 0:
		return 20, "low", "Record dreams to begin building a stable psyche profile."
	case total < 3:
		return 35 + float64(total*7), "low", "Keep recording dreams to build a richer psyche profile."
	case total < 10:
		return 56 + float64(total-3)*4, "moderate", "Your recent dreams show emerging symbolic continuity and self-reflection."
	default:
		score := 84 + float64(total-10)
		if score > 96 {
			score = 96
		}
		return score, "high", "Your dream journal shows stable integration across recurring themes."
	}
}

func psycheArchetypes(dreams []psycheDreamRow, integration float64) ([]v1.ArchetypeProfileItem, string) {
	scores := map[string]float64{
		"self":    integration,
		"persona": 42,
		"shadow":  42,
		"anima":   38,
		"sage":    44,
	}
	for _, dream := range dreams {
		emotion := strings.ToLower(strings.TrimSpace(dream.Emotion))
		switch emotion {
		case "fear", "angry", "anxious", "sad":
			scores["shadow"] += 4
		case "peaceful", "happy":
			scores["self"] += 2
		case "confused":
			scores["sage"] += 2
		case "excited":
			scores["persona"] += 2
		}
		for _, tag := range strings.Split(strings.ToLower(dream.Tags), ",") {
			tag = strings.TrimSpace(tag)
			switch {
			case strings.Contains(tag, "work") || strings.Contains(tag, "社交") || strings.Contains(tag, "学校"):
				scores["persona"] += 3
			case strings.Contains(tag, "fear") || strings.Contains(tag, "dark") || strings.Contains(tag, "阴影"):
				scores["shadow"] += 3
			case strings.Contains(tag, "love") || strings.Contains(tag, "family") || strings.Contains(tag, "情感"):
				scores["anima"] += 3
			case strings.Contains(tag, "guide") || strings.Contains(tag, "teacher") || strings.Contains(tag, "智慧"):
				scores["sage"] += 3
			}
		}
	}

	items := []v1.ArchetypeProfileItem{
		{Type: "self", Name: "Self", Description: "Wholeness and inner alignment"},
		{Type: "persona", Name: "Persona", Description: "Social identity and outward roles"},
		{Type: "shadow", Name: "Shadow", Description: "Unintegrated emotions and avoided themes"},
		{Type: "anima", Name: "Anima", Description: "Inner feeling and symbolic imagination"},
		{Type: "sage", Name: "Sage", Description: "Guidance, insight, and meaning-making"},
	}
	for i := range items {
		score := scores[items[i].Type]
		if score > 99 {
			score = 99
		}
		items[i].Score = score
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Score == items[j].Score {
			return items[i].Type < items[j].Type
		}
		return items[i].Score > items[j].Score
	})
	dominant := "self"
	if len(items) > 0 {
		dominant = items[0].Type
	}
	return items, dominant
}

// findOrCreateUser Find or create user
func (s *sAuth) findOrCreateWechatUser(ctx context.Context, openid string) (*entity.Users, error) {
	var user *entity.Users
	err := dao.Users.Ctx(ctx).Where("openid", openid).Where("deleted_at IS NULL").Scan(&user)
	if err != nil {
		return nil, gerror.Wrap(err, "Database error")
	}
	if user != nil {
		return user, nil
	}
	user = &entity.Users{
		Openid:       openid,
		AuthProvider: "wechat",
	}
	lastId, err := dao.Users.Ctx(ctx).Data(g.Map{
		"openid":        openid,
		"auth_provider": "wechat",
	}).InsertAndGetId()
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to create user")
	}
	user.Id = uint64(lastId)
	return user, nil
}

func (s *sAuth) findOrCreateEmailUser(ctx context.Context, supabaseUID, email string) (*entity.Users, error) {
	var user *entity.Users
	glog.Infof(ctx, "Find or create email user: %s, %s", supabaseUID, email)
	err := dao.Users.Ctx(ctx).
		Where("supabase_uid", supabaseUID).
		Where("deleted_at IS NULL").
		Scan(&user)
	if err != nil {
		return nil, gerror.Wrap(err, "Database error")
	}
	if user != nil {
		// Sync email if it changed on Supabase side
		if email != "" && user.Email != email {
			_, _ = dao.Users.Ctx(ctx).
				Data(g.Map{"email": email, "updated_at": gtime.Now().FormatTo("2006-01-02 15:04:05")}).
				Where("id", user.Id).Update()
			user.Email = email
		}
		return user, nil
	}
	// Create new user
	user = &entity.Users{
		SupabaseUid:  supabaseUID,
		Email:        email,
		AuthProvider: "email",
	}
	lastId, err := dao.Users.Ctx(ctx).Data(g.Map{
		"supabase_uid":  supabaseUID,
		"email":         email,
		"auth_provider": "email",
	}).InsertAndGetId()
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to create user")
	}
	user.Id = uint64(lastId)
	return user, nil
}

// getOpenidFromWechat Get openid from WeChat
func (s *sAuth) getOpenidFromWechat(ctx context.Context, code string) (string, error) {
	// WeChat mini program config (should be read from config file)
	appID := g.Cfg().MustGet(ctx, "wechat.appId").String()
	appSecret := g.Cfg().MustGet(ctx, "wechat.appSecret").String()

	url := fmt.Sprintf("https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		appID, appSecret, code)

	client := g.Client()
	response, err := client.Get(ctx, url)
	if err != nil {
		return "", gerror.Wrap(err, "Failed to request WeChat API")
	}
	defer response.Close()

	var result struct {
		Openid     string `json:"openid"`
		SessionKey string `json:"session_key"`
		Unionid    string `json:"unionid"`
		Errcode    int    `json:"errcode"`
		Errmsg     string `json:"errmsg"`
	}

	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", gerror.Wrap(err, "Failed to parse WeChat response")
	}

	if result.Errcode != 0 {
		return "", gerror.Newf("WeChat API error: %d %s", result.Errcode, result.Errmsg)
	}

	if result.Openid == "" {
		return "", gerror.New("Failed to get openid")
	}

	return result.Openid, nil
}

// generateServerJWT issue server JWT after auth
func (s *sAuth) generateServerJWT(ctx context.Context, userID uint64, supabaseUID, openid, provider string) (string, error) {
	secret := g.Cfg().MustGet(ctx, "jwt.secret").String()
	timeout := g.Cfg().MustGet(ctx, "jwt.timeout").Int()
	exp := time.Now().Add(time.Duration(timeout) * time.Second).Unix()
	claims := jwt.MapClaims{
		"userId":      userID,
		"supabaseUid": supabaseUID,
		"openid":      openid,
		"provider":    provider,
		"exp":         exp,
		"iat":         time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// VerifyJWT Verify JWT token
func (s *sAuth) VerifyJWT(ctx context.Context, tokenString string) (*model.AuthClaims, error) {
	secret := g.Cfg().MustGet(ctx, "jwt.secret").String()
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, gerror.New("Invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, gerror.New("Invalid token format")
	}
	return &model.AuthClaims{
		ID:           gconv.Uint64(claims["userId"]),
		SupabaseUID:  gconv.String(claims["supabaseUid"]),
		OpenID:       gconv.String(claims["openid"]),
		AuthProvider: gconv.String(claims["provider"]),
	}, nil
}
