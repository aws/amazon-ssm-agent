// +build darwin

package main

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	logger "github.com/aws/amazon-ssm-agent/agent/log/ssmlog"
)

const (
	proxyCommand            = "scutil"
	proxyArgument           = "--proxy"
	httpProxyEnabledRegex   = "HTTPEnable:1"
	httpsProxyEnabledRegex  = "HTTPSEnable:1"
	bypassProxyEnabledRegex = "ExceptionsList:"
	httpHostNameRegexExp    = `HTTPProxy:(.*)`
	httpPortRegexExp        = `HTTPPort:(.*)`
	httpsHostNameRegexExp   = `HTTPSProxy:(.*)`
	httpsPortRegexExp       = `HTTPSPort:(.*)`
	bypassProxyRegexExp     = `ExceptionsList:.*\n(\d.*\n)*}`
)

func main() {
	// initialize logger
	log := logger.SSMLogger(true)
	defer log.Close()
	defer log.Flush()

	// parse input parameters
	parseFlags(log)

	setProxySettings(log)

	// run agent
	run(log)
}

// retrieve proxy info from 1) environmental variables, or 2) scutil output
func setProxySettings(log log.T) {
	httpProxy, httpsProxy, noProxy := getWebProxies(log)

	os.Setenv("http_proxy", httpProxy)
	log.Info("http_proxy set to: ", httpProxy)

	os.Setenv("https_proxy", httpsProxy)
	log.Info("https_proxy set to: ", httpsProxy)

	os.Setenv("no_proxy", noProxy)
	log.Info("no_proxy set to: ", noProxy)
}

// helper function to parse HTTP, HTTPS, and bypass proxy settings from "scutil --proxy" command on MacOS
func getWebProxies(log log.T) (string, string, string) {

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1) // command cancelled after 1 second
	defer cancel()

	output, err := exec.CommandContext(ctx, proxyCommand, proxyArgument).Output()
	if err != nil {
		log.Info("Could not run scutil to retrieve proxy information.")
		return "", "", ""
	}

	var proxyInfo = strings.ReplaceAll(string(output), " ", "") // remove white spaces

	httpProxyEnabled, _ := regexp.MatchString(httpProxyEnabledRegex, proxyInfo)
	httpsProxyEnabled, _ := regexp.MatchString(httpsProxyEnabledRegex, proxyInfo)
	bypassProxyEnabled, _ := regexp.MatchString(bypassProxyEnabledRegex, proxyInfo)

	httpProxy := ""
	httpsProxy := ""
	noProxy := ""

	// Parse http proxy
	if httpProxyEnabled == true {
		httpHostNameRegex, _ := regexp.Compile(httpHostNameRegexExp)
		httpHostName := httpHostNameRegex.FindStringSubmatch(proxyInfo)[1]

		httpPortRegex, _ := regexp.Compile(httpPortRegexExp)
		httpPort := httpPortRegex.FindStringSubmatch(proxyInfo)[1]

		httpProxy = httpHostName + ":" + httpPort
	}

	// Parse https proxy
	if httpsProxyEnabled == true {
		httpsHostNameRegex, _ := regexp.Compile(httpsHostNameRegexExp)
		httpsHostName := httpsHostNameRegex.FindStringSubmatch(proxyInfo)[1]

		httpsPortRegex, _ := regexp.Compile(httpsPortRegexExp)
		httpsPort := httpsPortRegex.FindStringSubmatch(proxyInfo)[1]

		httpsProxy = httpsHostName + ":" + httpsPort
	}

	// Parse bypass proxy settings
	if bypassProxyEnabled == true {
		bypassProxyRegex, _ := regexp.Compile(bypassProxyRegexExp)
		exceptionsList := bypassProxyRegex.FindStringSubmatch(proxyInfo)

		// Ignore first
		exceptionsList = strings.Split(exceptionsList[0], "\n")
		exceptionsList = exceptionsList[1 : len(exceptionsList)-1]
		for i := 0; i < len(exceptionsList); i++ {
			exceptionsList[i] = exceptionsList[i][2:]
		}

		noProxy = strings.Join(exceptionsList, ",")
	}

	return httpProxy, httpsProxy, noProxy
}
