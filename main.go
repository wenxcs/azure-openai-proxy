package main

import (
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http/httputil"
	"net/url"
	"path"
	"regexp"
	"strings"
	"github.com/tidwall/gjson"
)

var (
	Address   = "0.0.0.0:8080"
	AzureOpenAIToken       = ""
	AzureOpenAIAPIVersion  = "2023-03-15-preview"
	AzureOpenAIEndpoint    = ""
	AzureOpenAIModelMapper = map[string]string{
		"gpt-3.5-turbo":      "gpt-35-turbo",
		"gpt-3.5-turbo-0301": "gpt-35-turbo-0301",
	}
	fallbackModelMapper = regexp.MustCompile(`[.:]`)
	response_json_credit_grants = `{
		"object": "credit_summary",
		"total_granted": 18.0,
		"total_used": 0,
		"total_available": 18.0,
		"grants": {
		  "object": "list",
		  "data": [
			{
			  "object": "credit_grant",
			  "id": "",
			  "grant_amount": 18.0,
			  "used_amount": 0.0,
			  "effective_at": 1675900800.0,
			  "expires_at": 1685577600.0
			}
		  ]
		}
	  }`
	response_json_models = `{
		"object": "list",
		"data": [
			{
				"id": "gpt-3.5-turbo-0301",
				"object": "model",
				"created": 1677649963,
				"owned_by": "openai",
				"permission": [
				  {
					"id": "modelperm-vrvwsIOWpZCbya4ceX3Kj4qw",
					"object": "model_permission",
					"created": 1679602087,
					"allow_create_engine": false,
					"allow_sampling": true,
					"allow_logprobs": true,
					"allow_search_indices": false,
					"allow_view": true,
					"allow_fine_tuning": false,
					"organization": "*",
					"group": null,
					"is_blocking": false
				  }
				],
				"root": "gpt-3.5-turbo-0301",
				"parent": null
			},
			{
				"id": "gpt-3.5-turbo",
				"object": "model",
				"created": 1677610602,
				"owned_by": "openai",
				"permission": [
				  {
					"id": "modelperm-M56FXnG1AsIr3SXq8BYPvXJA",
					"object": "model_permission",
					"created": 1679602088,
					"allow_create_engine": false,
					"allow_sampling": true,
					"allow_logprobs": true,
					"allow_search_indices": false,
					"allow_view": true,
					"allow_fine_tuning": false,
					"organization": "*",
					"group": null,
					"is_blocking": false
				  }
				],
				"root": "gpt-3.5-turbo",
				"parent": null
			}
		]
	}`
)

func init() {
	gin.SetMode(gin.ReleaseMode)
	if v := os.Getenv("AZURE_OPENAI_PROXY_ADDRESS"); v != "" {
		Address = v
	}
	log.Printf("loading azure openai proxy address: %s", Address)

	if v := os.Getenv("AZURE_OPENAI_APIVERSION"); v != "" {
		AzureOpenAIAPIVersion = v
	}
	if v := os.Getenv("AZURE_OPENAI_ENDPOINT"); v != "" {
		AzureOpenAIEndpoint = v
	}
	if v := os.Getenv("AZURE_OPENAI_MODEL_MAPPER"); v != "" {
		for _, pair := range strings.Split(v, ",") {
			info := strings.Split(pair, "=")
			if len(info) != 2 {
				log.Printf("error parsing AZURE_OPENAI_MODEL_MAPPER, invalid value %s", pair)
				os.Exit(1)
			}
			AzureOpenAIModelMapper[info[0]] = info[1]
		}
	}
	if v := os.Getenv("AZURE_OPENAI_TOKEN"); v != "" {
		AzureOpenAIToken = v
		log.Printf("loading azure api token from env")
	}
	log.Printf("loading azure api endpoint: %s", AzureOpenAIEndpoint)
	log.Printf("loading azure api version: %s", AzureOpenAIAPIVersion)
	for k, v := range AzureOpenAIModelMapper {
		log.Printf("loading azure model mapper: %s -> %s", k, v)
	}
}

func NewOpenAIReverseProxy() *httputil.ReverseProxy {
	director := func(req *http.Request) {
		// Set the Host, Scheme, Path, and RawPath of the request to the remote host and path
		originURL := req.URL.String()
		remote, _ := url.Parse(AzureOpenAIEndpoint)
		body, _ := ioutil.ReadAll(req.Body)
		req.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		model := gjson.GetBytes(body, "model").String()
		deployment := GetDeploymentByModel(model)

		// Replace the Bearer field in the Authorization header with api-key
		token := ""
		host := remote.Host

		// use the token from the environment variable if it is set
		if AzureOpenAIToken != "" {
			token = AzureOpenAIToken
		} else {
			token = strings.ReplaceAll(req.Header.Get("Authorization"), "Bearer ", "")
			if strings.Contains(token, "@") {
				token_split := strings.Split(token, "@")
				token = token_split[0]
				host = token_split[1]

				if len(token_split) > 2 {
					deployment = token_split[2]
				}
			}
		}

		req.Header.Set("api-key", token)
		req.Header.Del("Authorization")
		req.Host = host
		req.URL.Scheme = remote.Scheme
		req.URL.Host = host
		req.URL.Path = path.Join(fmt.Sprintf("/openai/deployments/%s", deployment), strings.Replace(req.URL.Path, "/v1/", "/", 1))
		req.URL.RawPath = req.URL.EscapedPath()

		// Add the api-version query parameter to the request URL
		query := req.URL.Query()
		query.Add("api-version", AzureOpenAIAPIVersion)
		req.URL.RawQuery = query.Encode()

		log.Printf("proxying request [%s] %s -> %s", model, originURL, req.URL.String())
	}
	return &httputil.ReverseProxy{Director: director}
}

func GetDeploymentByModel(model string) string {
	if v, ok := AzureOpenAIModelMapper[model]; ok {
		return v
	}
	// This is a fallback strategy in case the model is not found in the AzureOpenAIModelMapper
	return fallbackModelMapper.ReplaceAllString(model, "")
}

func main() {
	r := gin.Default()
	r.Any("/", func(c *gin.Context) {
		c.Status(200)
	})
	r.Any("/health", func(c *gin.Context) {
		c.Status(200)
	})
	r.Any("/v1/*path", func(c *gin.Context) {
		// BUGFIX: fix options request, see https://github.com/diemus/azure-openai-proxy/issues/1
		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Status(200)
			return
		}

		originPath := fmt.Sprintf("%s", c.Request.URL.Path)
		if strings.HasSuffix(originPath, "completions") || strings.HasSuffix(originPath, "embeddings") {
			server := NewOpenAIReverseProxy()
			server.ServeHTTP(c.Writer, c.Request)
			//BUGFIX: try to fix the difference between azure and openai
			//Azure's response is missing a \n at the end of the stream
			//see https://github.com/Chanzhaoyu/chatgpt-web/issues/831
			if c.Writer.Header().Get("Content-Type") == "text/event-stream" {
				if _, err := c.Writer.Write([]byte("\n")); err != nil {
					log.Printf("rewrite azure response error: %v", err)
				}
			}
		} else if strings.HasSuffix(originPath, "credit_grants") {
			c.String(200, response_json_credit_grants)
		} else if strings.HasSuffix(originPath, "models") {
			c.String(200, response_json_models)
		} else {
			c.Status(200)
		}
	})
	r.Run(Address)
}
