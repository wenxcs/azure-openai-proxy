# Azure OpenAI Proxy

## Deployment

1. Set your DNS record' CAA to `letsencrypt.org`
2. Expose your 80, 443 to public access
3. Clone this repo, and config `ENDPOINT` as your Azure OpenAI endpoint, `HOST` as your public URL
4. Use `sudo docker compose up -d` to start the service

## Usage
In any client support original openai api, configure your `secret` as `your passoword` or `your password@customized endpoint` or `your password@customized endpoint@your model deployment`. And your deployment host url to your client's corresponding field.

By default, this proxy will use `gpt-35-turbo, gpt-35-turbo-0301` as deloyment names. If the deployment name is in your secret, this proxy will only use that.

## Status
Checked - iOS, iPadOS, MacOS: OpenCat: https://opencat.app/  
Checked - Linux, Windows, MacOS: chatbox: https://github.com/Bin-Huang/chatbox

