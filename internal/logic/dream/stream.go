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

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/gogf/gf/v2/text/gregex"
	"github.com/gogf/gf/v2/text/gstr"
	"github.com/gogf/gf/v2/util/guid"
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

		// First, record the dream content
		dreamId, err := dao.Dreams.Ctx(ctx).Data(g.Map{
			"user_id":    userid,
			"content":    content,
			"dream_date": gtime.Now().Format("Y-m-d"),
		}).InsertAndGetId()
		if err != nil {
			glog.Async(true).Error(ctx, err)
			ch <- "[error]failed to insert dream"
			return
		}

		// TODO: Search for similar dreams to reduce LLM calls
		// TODO: If similar dreams are found, also simulate streaming output

		// Search for relevant knowledge
		kw, err := shareKnowledge.search(ctx, content)
		if err != nil {
			// If knowledge base search fails, it should not cause the entire analysis process to fail, just log it
			glog.Warningf(ctx, "知识库搜索失败: %v", err)
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

		// Create session record
		sessionUUID := guid.S()
		sessionId, err := dao.AnalysisSessions.Ctx(ctx).Data(g.Map{
			"dream_id":     dreamId,
			"session_uuid": sessionUUID,
			"status":       "processing",
		}).InsertAndGetId()
		if err != nil {
			glog.Async(true).Error(ctx, err)
			ch <- "[error] Create analysis session failed"
		}

		glog.Async(true).Infof(ctx, "Session created, session_id: %d", sessionId)
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

		// Summarize LLM output
		var contentSummery string

		for {
			select {
			case <-ctx.Done():
				// Cancel: Stop consuming LLM output and exit
				glog.Async(true).Info(ctx, "stream canceled by client")
				// Update session status to canceled
				_, _ = dao.AnalysisSessions.Ctx(ctx).Where("id", sessionId).Update(g.Map{
					"status":   "canceled",
					"progress": 0,
				})
				return
			case piece, ok := <-contentChan:
				if !ok {
					// Normal completion
					status := "completed"
					if contentSummery == "" {
						status = "empty"
					}
					title, content := cleanText(contentSummery)
					err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
						// Update dream table
						_, err := dao.Dreams.Ctx(ctx).Where("id", dreamId).Update(g.Map{
							"title":  title,
							"status": status,
						})
						if err != nil {
							return err
						}
						// Update analysis_sessions table
						_, err = dao.AnalysisSessions.Ctx(ctx).Where("id", sessionId).Update(g.Map{
							"status":         status,
							"progress":       100,
							"result_summary": content,
						})
						if err != nil {
							return err
						}
						return nil
					})
					if err != nil {
						glog.Async(true).Error(ctx, err)
						ch <- "[error] Update database failed"
						return
					}
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
				contentSummery += piece
			}
		}
	}()

	return ch, nil
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
