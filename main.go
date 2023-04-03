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
	OpenAIToken       = ""
	AzureOpenAIAPIVersion  = "2023-03-15-preview"
	AzureOpenAIEndpoint    = ""
	AzureOpenAIModelMapper = map[string]string{
		"gpt-3.5-turbo":      "gpt-35-turbo",
		"gpt-3.5-turbo-0301": "gpt-35-turbo-0301",
	}
	fallbackModelMapper = regexp.MustCompile(`[.:]`)
	v1_models = "https://gist.githubusercontent.com/wenxcs/e1ebd6a38427e707a4e5141195a854d3/raw/models.json"
	v1_credit_grants = "https://gist.githubusercontent.com/wenxcs/e1ebd6a38427e707a4e5141195a854d3/raw/credit_grants.json"
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
	if v := os.Getenv("OPENAI_TOKEN"); v != "" {
		OpenAIToken = v
		log.Printf("loading openai api token from env")
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
		originPath := fmt.Sprintf("%s", req.URL.Path)
		if strings.HasSuffix(originPath, "completions") || strings.HasSuffix(originPath, "embeddings") {
			remote, _ := url.Parse(AzureOpenAIEndpoint)
			body, _ := ioutil.ReadAll(req.Body)
			req.Body = ioutil.NopCloser(bytes.NewBuffer(body))
			model := gjson.GetBytes(body, "model").String()
			deployment := GetDeploymentByModel(model)

			// Replace the Bearer field in the Authorization header with api-key
			token := ""

			// use the token from the environment variable if it is set
			if AzureOpenAIToken != "" {
				token = AzureOpenAIToken
			} else {
				token := strings.ReplaceAll(req.Header.Get("Authorization"), "Bearer ", "")
			}

			req.Header.Set("api-key", token)
			req.Header.Del("Authorization")
			req.Host = remote.Host
			req.URL.Scheme = remote.Scheme
			req.URL.Host = remote.Host
			req.URL.Path = path.Join(fmt.Sprintf("/openai/deployments/%s", deployment), strings.Replace(req.URL.Path, "/v1/", "/", 1))
			req.URL.RawPath = req.URL.EscapedPath()

			// Add the api-version query parameter to the request URL
			query := req.URL.Query()
			query.Add("api-version", AzureOpenAIAPIVersion)
			req.URL.RawQuery = query.Encode()

			log.Printf("proxying request [%s] %s -> %s", model, originURL, req.URL.String())
		} else if strings.HasSuffix(originPath, "models") {
			remote, _ := url.Parse(v1_models)
			req.Host = remote.Host
			req.URL.Scheme = remote.Scheme
			req.URL.Host = remote.Host
			req.URL.Path = remote.URL.Path
			req.URL.RawPath = remote.URL.RawPath
		}  else if strings.HasSuffix(originPath, "credit_grants") {
			remote, _ := url.Parse(v1_credit_grants)
			req.Host = remote.Host
			req.URL.Scheme = remote.Scheme
			req.URL.Host = remote.Host
			req.URL.Path = remote.URL.Path
			req.URL.RawPath = remote.URL.RawPath
		} else {
			remote, _ := url.Parse("https://api.openai.com")
			req.Host = remote.Host
			req.URL.Scheme = remote.Scheme
			req.URL.Host = remote.Host

			token := ""
			if OpenAIToken != "" {
				token = OpenAIToken
			} else {
				token := strings.ReplaceAll(req.Header.Get("Authorization"), "Bearer ", "")
			}

			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
			log.Printf("proxying request %s -> %s", originURL, req.URL.String())
		}
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
	r.Any("*path", func(c *gin.Context) {
		// BUGFIX: fix options request, see https://github.com/diemus/azure-openai-proxy/issues/1
		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Status(200)
			return
		}

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
	})

	r.Run(Address)

}
