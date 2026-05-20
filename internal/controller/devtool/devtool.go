package devtool

import (
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/gclient"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gtime"
)

type Controller struct{}

func (c *Controller) Index(r *ghttp.Request) {
	view := g.View()
	// Explicitly add template path for safety in dev env
	_ = view.AddPath("resource/template")
	err := r.Response.WriteTpl("dev_api_test.html", g.Map{
		"title": "Dream Master API Developer Test Tool",
	})
	if err != nil {
		r.Response.Write("Template rendering failed: " + err.Error())
	}
}

type ProxyReq struct {
	Method  string            `json:"method"`
	Url     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

type ProxyRes struct {
	Status     int               `json:"status"`
	StatusText string            `json:"statusText"`
	TimeMs     int64             `json:"timeMs"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	Error      string            `json:"error,omitempty"`
}

func (c *Controller) Proxy(r *ghttp.Request) {
	var req *ProxyReq
	if err := r.Parse(&req); err != nil {
		r.Response.WriteJson(g.Map{"error": "Invalid proxy request parameter: " + err.Error()})
		return
	}

	if req.Url == "" {
		r.Response.WriteJson(g.Map{"error": "URL cannot be empty"})
		return
	}

	client := g.Client()
	// Set custom headers
	for k, v := range req.Headers {
		client.SetHeader(k, v)
	}

	// Append X-Dev-Token to request header
	client.SetHeader("X-Dev-Token", "nowled2_token")

	startTime := gtime.TimestampMilli()

	// Handle body and method
	method := strings.ToUpper(req.Method)
	var resp *gclient.Response
	var err error

	switch method {
	case "GET":
		resp, err = client.Get(r.Context(), req.Url)
	case "POST":
		resp, err = client.Post(r.Context(), req.Url, req.Body)
	case "PUT":
		resp, err = client.Put(r.Context(), req.Url, req.Body)
	case "DELETE":
		resp, err = client.Delete(r.Context(), req.Url, req.Body)
	case "PATCH":
		resp, err = client.Patch(r.Context(), req.Url, req.Body)
	case "HEAD":
		resp, err = client.Head(r.Context(), req.Url)
	case "OPTIONS":
		resp, err = client.Options(r.Context(), req.Url)
	default:
		r.Response.WriteJson(g.Map{"error": "Unsupported HTTP method: " + req.Method})
		return
	}

	if err != nil {
		r.Response.WriteJson(ProxyRes{
			Error: err.Error(),
		})
		return
	}
	defer resp.Close()

	endTime := gtime.TimestampMilli()
	duration := endTime - startTime

	// Read response body
	respBody := resp.ReadAllString()

	// Read response headers
	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			respHeaders[k] = v[0]
		}
	}

	r.Response.WriteJson(ProxyRes{
		Status:     resp.StatusCode,
		StatusText: resp.Status,
		TimeMs:     duration,
		Headers:    respHeaders,
		Body:       respBody,
	})
}
