package dream

import (
	"context"
	"dm-server/internal/consts"
	"dm-server/internal/dao"
	"dm-server/internal/model/entity"
	"dm-server/internal/service"
	"dm-server/internal/utility/limiter"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/gogf/gf/v2/text/gregex"
	"github.com/gogf/gf/v2/text/gstr"
)

// AIServiceSelector Define AI service selector interface
type AIServiceSelector interface {
	AnalyzeDreamStream(ctx context.Context, prompt, content string) (<-chan string, error)
}

type sDream struct{}

func New() *sDream {
	return &sDream{}
}

func init() {
	service.RegisterDream(New())
}

// StreamDream Real-time streaming dream analysis
func (s *sDream) StreamDream(ctx context.Context, content string) (<-chan string, error) {
	ch := make(chan string, 10)

	// Try to acquire concurrency quota before starting worker goroutine
	if !limiter.Acquire(ctx) {
		// Immediately return a channel that pushes one error
		go func() {
			defer close(ch)
			ch <- "[error]Service busy, please try again later"
		}()
		return ch, nil
	}

	go func() {
		defer close(ch)
		// Ensure quota is released
		defer limiter.Release()

		// Check ctx before any time-consuming operation
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Get user openid from ctx
		userid := ctx.Value(consts.CtxUserId)
		if g.IsEmpty(userid) {
			glog.Async(true).Error(ctx, fmt.Errorf("failed to get user id"))
			ch <- "服务繁忙，请稍候再试"
			return
		}

		// Get user openid for rate limiting
		openidValue := ctx.Value(consts.CtxOpenId)
		if g.IsEmpty(openidValue) {
			// If openid is not in ctx, get it from database
			var user *entity.Users
			err := dao.Users.Ctx(ctx).Where("id", userid).Scan(&user)
			if err != nil || user == nil {
				glog.Async(true).Error(ctx, fmt.Errorf("failed to get user info: %v", err))
				ch <- "服务繁忙，请稍候再试"
				return
			}
			openidValue = user.Openid
		}

		// Check user analysis count limit
		if !limiter.CheckUserAnalysisLimit(ctx, openidValue.(string)) {
			glog.Async(true).Infof(ctx, "User %s exceeds analysis count limit", openidValue)
			ch <- "[error]您已达到小时内分析次数上限，请稍后再试"
			return
		}

		userIdStr := fmt.Sprintf("%v", userid)
		emotionTags := dreamEmotionTagsFromContext(ctx)

		// Try the L1-aware knowledge interpretation API first. A cache hit can answer without LLM.
		interpretResp, err := shareKnowledge.interpretDream(ctx, content, userIdStr, emotionTags)
		if err != nil {
			glog.Warningf(ctx, "知识库L1解析失败: %v", err)
		}
		if interpretResp != nil && interpretResp.isL1Hit() {
			glog.Infof(ctx, "知识库L1缓存命中: symbols=%v matched=%v", interpretResp.Data.SymbolsDetected, interpretResp.Data.SymbolsMatched)
			ch <- interpretResp.Data.Interpretation
			return
		}

		kw := interpretResp.toSearchResponse()
		if kw == nil {
			// Fall back to the legacy search endpoint for older knowledge-service deployments.
			kw, err = shareKnowledge.search(ctx, content)
			if err != nil {
				// If knowledge base search fails, it should not cause the entire analysis process to fail, just log it
				glog.Warningf(ctx, "知识库搜索失败: %v", err)
			}
		}

		// 3. Format knowledge base content
		var knowledgeContent string
		if shareKnowledge != nil {
			knowledgeContent = shareKnowledge.formatKnowledge(kw)
		}
		glog.Infof(ctx, "知识库内容: %s", knowledgeContent)

		// 4. Get dream analysis prompt
		prompt, err := g.Cfg().Get(ctx, "prompts.dream_analysis")
		if err != nil {
			ch <- "[error] Get davinci failed"
			return
		}
		fullPrompt := "``` \n# 梦境解析专家系统指令\n\n"

		// 5. If there is relevant knowledge, append it to the prompt
		if knowledgeContent != "" {
			fullPrompt = fmt.Sprintf("%s## 知识库调用指令：\n这些是通过分析用户梦境，知识库检索弗洛伊德《梦的解析》、荣格《红书解析》、《中国传统解梦》和《现代睡眠神经科学》返回的知识片段（不是所有书籍都有有效返回，返回内容可能不匹配），你可以结合片段进行分析。\n\n%s", fullPrompt, knowledgeContent)
		}

		fullPrompt += prompt.String()

		// 6. Call AI service to analyze dream (select service according to configuration)
		contentChan, err := s.analyzeDreamStream(ctx, fullPrompt, content)
		if err != nil {
			// Do not use Error to write logs when the request is canceled
			if errors.Is(err, context.Canceled) {
				glog.Async(true).Info(ctx, "stream canceled by client")
				return
			}
			glog.Async(true).Error(ctx, err)
			ch <- "[error] Call AI_Service to analyze dream failed"
			return
		}
		glog.Async(true).Infof(ctx, "AI_Service analysis started successfully")

		for {
			select {
			case <-ctx.Done():
				// Cancel: Stop consuming LLM output and exit
				glog.Async(true).Info(ctx, "stream canceled by client")
				return
			case piece, ok := <-contentChan:
				if !ok {
					return
				}
				if piece == "" {
					continue
				}
				if strings.HasPrefix(piece, "[error]") {
					glog.Async(true).Error(ctx, fmt.Errorf("LLM output error: %s", piece))
					ch <- "服务繁忙，请稍候再试"
					return
				}
				ch <- piece
			}
		}
	}()

	return ch, nil
}

func dreamEmotionTagsFromContext(ctx context.Context) []string {
	value := ctx.Value(consts.CtxDreamEmotionTags)
	switch tags := value.(type) {
	case []string:
		return filterEmotionTags(tags...)
	case string:
		return filterEmotionTags(tags)
	default:
		return nil
	}
}

func filterEmotionTags(tags ...string) []string {
	filtered := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		filtered = append(filtered, tag)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

// Clean text and extract title from text
func cleanText(text string) (string, string) {
	// Remove <think>.+</think> tags and the text inside them
	re1 := regexp.MustCompile(`(?s)\n?\s*<think>.*?</think>\s*\n?`)
	clearText := re1.ReplaceAllString(text, "")

	// Remove leading newlines
	clearText = gstr.Replace(clearText, "\n+", "")

	// Get the next line as title
	titleText := gstr.Split(clearText, "\n")[0]
	// Ensure it starts with '#' character
	if strings.HasPrefix(gstr.Trim(titleText), "#") {
		// Get text between 「」 as title
		match, err := gregex.MatchString(`「(.+?)」`, titleText)
		if err != nil {
			return "", clearText
		}
		if len(match) < 2 {
			return "", clearText
		}
		titleText = match[1]
	}
	return gstr.Trim(titleText), gstr.Trim(clearText)
}
