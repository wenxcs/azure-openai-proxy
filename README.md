# Azure OpenAI Proxy

## Deployment

1. Set your DNS record' CAA to `letsencrypt.org`
2. Expose your 80, 443 to public access
3. Clone this repo, and config enviorment `ENDPOINT` as your Azure OpenAI endpoint, `HOST` as your public URL. Both are without http or https schema
4. Use `sudo docker compose up -d` to start the service

## Usage
In any client support original openai api, configure your `secret` as one of the three cases:   
 - `your password`   
 - `your password@customized endpoint`
 - `your password@customized endpoint@your model deployment`     

For example, "aaaa@bbbb.openai.azure.com@mygpt" is a valid secret which use token aaaa to communicate with bbbb.openai.azure.com using deployment mygpt.   
 - must use "bbbb.openai.azure.com", with *NO* "http://" or "https://".

By default, this proxy will use `gpt-35-turbo, gpt-35-turbo-0301` as deloyment names. If the deployment name is in your secret, this proxy will only use that.   
At last, change your proxy host url to your client's corresponding field.   

If you only include password in the secret, this proxy will redirect all request to your enviorment `ENDPOINT`.

## Status
### Opencat
 - Platform: iOS, iPadOS, MacOS
 - Link: https://opencat.app/   
 - Known issue: must include port 443 in host url in iOS opencat  

### AMA
 - Platform: Android
 - Link: https://play.google.com/store/apps/details?id=com.bytemyth.ama   
 - Known issue: must use `https://` in host url

## Credit

Many thanks to projects: 
- https://github.com/stulzq/azure-openai-proxy
- https://github.com/diemus/azure-openai-proxy
