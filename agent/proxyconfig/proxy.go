package proxyconfig

import "os"

const (
	PROXY_VAR_HTTPS  = "https_proxy"
	PROXY_VAR_HTTP   = "http_proxy"
	PROXY_VAR_BYPASS = "no_proxy"
)

// HTTP Proxy environment variables possible values
var ProxyEnvVariables = []string{
	PROXY_VAR_HTTPS,
	PROXY_VAR_HTTP,
	PROXY_VAR_BYPASS,
}

func GetProxyConfig() (proxyValues map[string]string) {
	proxyValues = map[string]string{}
	for _, proxyVar := range ProxyEnvVariables {
		proxyValues[proxyVar] = os.Getenv(proxyVar)
	}
	return proxyValues
}
