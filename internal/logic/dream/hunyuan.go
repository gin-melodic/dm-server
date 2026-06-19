package dream

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	hunyuan "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/hunyuan/v20230901"
)

type sHunyuan struct{}

var shareHunyuan = &sHunyuan{}

// analyzeDreamStream Use Tencent Hunyuan to analyze dream
func (s *sHunyuan) analyzeDreamStream(ctx context.Context, prompt, dreamContent string) (<-chan string, error) {
	// Get Tencent Hunyuan configuration
	hunyuanConfig, err := g.Cfg().Get(ctx, "hunyuan")
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to get Tencent Hunyuan configuration")
	}

	secretID := hunyuanConfig.MapStrStr()["secret_id"]
	secretKey := hunyuanConfig.MapStrStr()["secret_key"]
	region := hunyuanConfig.MapStrStr()["region"]
	model := hunyuanConfig.MapStrStr()["model"]

	// Set default values
	if region == "" {
		region = "ap-guangzhou"
	}
	if model == "" {
		model = "hy3-preview"
	}

	// If the configuration does not contain key information, try to get it from the environment variable
	if secretID == "" {
		secretID = os.Getenv("TENCENTCLOUD_SECRET_ID")
	}
	if secretKey == "" {
		secretKey = os.Getenv("TENCENTCLOUD_SECRET_KEY")
	}

	// Check if the key exists
	if secretID == "" || secretKey == "" {
		return nil, gerror.New("Tencent Hunyuan API key is not configured")
	}

	glog.Infof(ctx, "Region: %s", region)
	glog.Infof(ctx, "Model: %s", model)

	// Construct the complete prompt
	fullPrompt := fmt.Sprintf("%s\n\n用户的梦境内容如下：\n%s", prompt, dreamContent)
	glog.Infof(ctx, "Prompt: %s", previewRunes(fullPrompt, 100))

	// Use Tencent Cloud SDK
	credential := common.NewCredential(secretID, secretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "hunyuan.tencentcloudapi.com"

	// Create client
	client, err := hunyuan.NewClient(credential, region, cpf)
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to create Tencent Hunyuan client")
	}

	// Construct request
	request := hunyuan.NewChatCompletionsRequest()
	request.Model = &model
	request.Stream = common.BoolPtr(true)

	// Construct message
	systemMessage := &hunyuan.Message{
		Role:    common.StringPtr("system"),
		Content: common.StringPtr(prompt),
	}

	userMessage := &hunyuan.Message{
		Role:    common.StringPtr("user"),
		Content: common.StringPtr(fmt.Sprintf("用户的梦境内容如下：\n%s", dreamContent)),
	}

	request.Messages = []*hunyuan.Message{systemMessage, userMessage}

	// Set parameters
	temperature := float64(0.95)
	topP := float64(0.8)
	request.Temperature = &temperature
	request.TopP = &topP

	// Use exponential backoff retry mechanism to handle rate limiting
	maxRetries := 3
	retryDelay := time.Second * 2

	for attempt := 0; attempt <= maxRetries; attempt++ {
		glog.Debugf(ctx, "Attempt %d (max retries: %d)", attempt, maxRetries)

		// Send request
		response, err := client.ChatCompletions(request)
		if err != nil {
			// Check if it is a Tencent Cloud SDK error
			if sdkErr, ok := err.(*errors.TencentCloudSDKError); ok {
				// Check if it encountered rate limiting
				if sdkErr.Code == "RequestLimitExceeded" {
					if attempt < maxRetries {
						glog.Warningf(ctx, "Tencent Hunyuan API rate limit, retrying in %v", retryDelay)
						time.Sleep(retryDelay)
						// Exponentially increase delay time
						retryDelay *= 2
					} else {
						glog.Debugf(ctx, "Reached maximum retry attempts, returning rate limiting error")
						return nil, gerror.New("Tencent Hunyuan API call frequency exceeded, please try again later")
					}
				}
			}

			// Other errors are returned directly
			glog.Errorf(ctx, "API call failed: %v", err)
			return nil, gerror.Wrap(err, "Failed to call Tencent Hunyuan API")
		}

		// Create a channel for returning
		resultChan := make(chan string, 10)

		// Start a goroutine to read the streaming response
		go func() {
			defer close(resultChan)

			glog.Debugf(ctx, "Start reading response stream")

			// Stream response processing
			if response.Events != nil {
				for event := range response.Events {
					if event.Data != nil {
						// Parse response data
						var hunyuanResp sHunyuanResponse
						if err := json.Unmarshal(event.Data, &hunyuanResp); err != nil {
							glog.Debugf(ctx, "JSON parsing failed: %v, data: %s", err, string(event.Data))
							continue
						}

						// Send response content to result channel
						if len(hunyuanResp.Choices) > 0 {
							choice := hunyuanResp.Choices[0]
							content := choice.Message.Content
							if content == "" {
								content = choice.Delta.Content
							}
							if content != "" {
								// glog.Debugf(ctx, "Sending content to channel: %s", content)
								resultChan <- content
							}

							// Process finish reason
							if choice.FinishReason != "" && choice.FinishReason != "null" {
								glog.Debugf(ctx, "Received finish reason: %s", choice.FinishReason)

								// Special handling for different finish reasons
								switch choice.FinishReason {
								case "length":
									// Content length limit
									glog.Debugf(ctx, "Model reached maximum length limit, sending truncation warning")
									resultChan <- "[warning]由于内容长度限制，生成的内容可能不完整。如需完整分析，请尝试提供更短的梦境描述。"
								case "stop":
									// Normal completion
									glog.Debugf(ctx, "Model normally completed generation")
								case "sensitive":
									// 内容过滤
									glog.Warningf(ctx, "Forbidden content")
									resultChan <- "[warning]生成内容包含敏感内容，已被过滤。"
								default:
									// 其他未知原因
									glog.Debugf(ctx, "Unknown finish reason: %s", choice.FinishReason)
									resultChan <- fmt.Sprintf("[info]模型完成生成，原因: %s", choice.FinishReason)
								}

								glog.Info(ctx, "Model output completed")
								return
							}
						}
					}
				}
			}
		}()

		return resultChan, nil
	}

	return nil, gerror.New("Tencent Hunyuan API call failed, max retries reached")
}

// sHunyuanResponse represents the response structure from Hunyuan API
type sHunyuanResponse struct {
	Choices []struct {
		FinishReason string `json:"FinishReason"`
		Delta        struct {
			Content string `json:"Content"`
		} `json:"Delta"`
		Message struct {
			Content string `json:"Content"`
		} `json:"Message"`
	} `json:"Choices"`
	Created int64  `json:"Created"`
	ID      string `json:"Id"`
	Usage   struct {
		PromptTokens     int `json:"PromptTokens"`
		CompletionTokens int `json:"CompletionTokens"`
		TotalTokens      int `json:"TotalTokens"`
	} `json:"Usage"`
	Note string `json:"Note"`
}
