package azure

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"encoding/json"
	"os"
	"path"
	"regexp"
	"strings"
	"github.com/wenxcs/azure-openai-proxy/pkg/openai"
	"github.com/tidwall/gjson"
)

var (
	AzureOpenAIToken       = ""
	OpenAIToken       = ""
	AzureOpenAIAPIVersion  = "2023-03-15-preview"
	AzureOpenAIEndpoint    = ""
	AzureOpenAIModelMapper = map[string]string{
		"gpt-3.5-turbo":      "gpt-35-turbo",
		"gpt-3.5-turbo-0301": "gpt-35-turbo-0301",
	}
	fallbackModelMapper = regexp.MustCompile(`[.:]`)
)

func init() {
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

type Tokens struct {
	azure string `json:"azure"`
	openai string `json:"openai"`
}

func NewOpenAIReverseProxy() *httputil.ReverseProxy {
	remote, err := url.Parse(AzureOpenAIEndpoint)
	if err != nil {
		log.Printf("error parse endpoint: %s\n", AzureOpenAIEndpoint)
		os.Exit(1)
	}
	director := func(req *http.Request) {
		// Set the Host, Scheme, Path, and RawPath of the request to the remote host and path
		originURL := req.URL.String()
		if strings.HasSuffix(originURL, "completions") || strings.HasSuffix(originURL, "embeddings") {
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
				var tokens Tokens
				token_body = strings.ReplaceAll(req.Header.Get("Authorization"), "Bearer ", "")
				json.Unmarshal([]byte(token_body), &tokens)
				token = tokens.azure
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
		} else {
			remote, _ := url.Parse("https://api.openai.com")
			req.Host = remote.Host
			req.URL.Scheme = remote.Scheme
			req.URL.Host = remote.Host

			token := ""
			if OpenAIToken != "" {
				token = OpenAIToken
			} else {
				var tokens Tokens
				token_body = strings.ReplaceAll(req.Header.Get("Authorization"), "Bearer ", "")
				json.Unmarshal([]byte(token_body), &tokens)
				token = tokens.openai
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
