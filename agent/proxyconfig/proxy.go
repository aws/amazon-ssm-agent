package proxyconfig

import "os"

// HTTP Proxy environment variables possible values
var ProxyEnvVariables = []string{
	"https_proxy",
	"http_proxy",
	"no_proxy",
}

func GetProxyConfig() (proxyValues map[string]string) {
	proxyValues = map[string]string{}
	for _, proxyVar := range ProxyEnvVariables {
		proxyValues[proxyVar] = os.Getenv(proxyVar)
	}
	return proxyValues
}
