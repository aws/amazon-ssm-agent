package utility

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/backoffconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/network"
	"github.com/cenkalti/backoff/v4"
)

const (
	SSMSetupCLIArtifactsFolderName = "ssm_setup_cli_artifacts"
	StableVersionString            = "stable"
	LatestVersionString            = "latest"
	VersionFile                    = "VERSION"
)

func HttpDownload(log log.T, fileURL string, destinationPath string) (string, error) {
	var localFilePath string
	var err error

	log.Debugf("attempting to download as http/https download from %v to %v", fileURL, destinationPath)
	urlHash := sha1.Sum([]byte(fileURL))
	destFile := filepath.Join(destinationPath, fmt.Sprintf("%x", urlHash))

	exponentialBackoff, err := backoffconfig.GetExponentialBackoff(200*time.Millisecond, 5)
	if err != nil {
		return "", err
	}

	download := func() (err error) {
		var httpClient http.Client
		var httpRequest *http.Request
		httpRequest, err = http.NewRequest("GET", fileURL, nil)
		if err != nil {
			return err
		}

		customTransport := network.GetDefaultTransport(log, appconfig.SsmagentConfig{})
		customTransport.TLSHandshakeTimeout = 20 * time.Second
		httpClient = http.Client{
			CheckRedirect: func(r *http.Request, via []*http.Request) error {
				r.URL.Opaque = r.URL.Path
				return nil
			},
			Transport: customTransport,
		}

		var resp *http.Response
		resp, err = httpClient.Do(httpRequest)
		if err != nil {
			log.Debugf("failed to download from http/https: %v", err)
			fileutil.DeleteFile(destFile)
			return
		}

		if resp.StatusCode != http.StatusOK {
			fileutil.DeleteFile(destFile)
			log.Debugf("failed to download from http/https: %v", err)
			err = fmt.Errorf("http request failed. status:%v statuscode:%v", resp.Status, resp.StatusCode)
			// skip backoff logic if permission denied to the URL
			if resp.StatusCode == http.StatusForbidden {
				return &backoff.PermanentError{Err: err}
			}
			return
		}
		defer resp.Body.Close()

		_, err = fileCopy(log, destFile, resp.Body)
		if err == nil {
			localFilePath = destFile
		} else {
			log.Errorf("failed to write destFile %v, %v ", destFile, err)
		}
		return
	}

	err = backoff.Retry(download, exponentialBackoff)
	return localFilePath, err
}

func HttpReadContent(stableVersionUrl string, client *http.Client) ([]byte, error) {
	resp, err := client.Get(stableVersionUrl)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("response code is nil")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unsuccessful request: response code: %v", resp.StatusCode)
	}
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return content, fmt.Errorf("failed to read response from %s: %v", stableVersionUrl, err)
	}
	return content, nil
}

// fileCopy copies the content from reader to destinationPath file
func fileCopy(log log.T, destinationPath string, src io.Reader) (written int64, err error) {
	var file *os.File
	file, err = os.Create(destinationPath)
	if err != nil {
		log.Errorf("failed to create file. %v", err)
		return
	}
	defer file.Close()
	var size int64
	size, err = io.Copy(file, src)
	log.Infof("%s with %v bytes downloaded", destinationPath, size)
	return
}

// FileExists checks whether the file is present on the instance
func FileExists(filePath string) (bool, error) {
	_, err := os.Stat(filePath)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// CleanupVersion is used cross package managers to remove additional characters from version
func CleanupVersion(version string) string {
	versionExtractor := regexp.MustCompile(`\d+.\d+.\d+.\d+`)
	return versionExtractor.FindString(version)
}

// ComputeCheckSum computes check sum of binaries
func ComputeCheckSum(filePath string) (hash string, err error) {
	var f *os.File
	f, err = os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err = io.Copy(hasher, f); err != nil {
		return "", err
	}
	hash = hex.EncodeToString(hasher.Sum(nil))

	return hash, nil
}
