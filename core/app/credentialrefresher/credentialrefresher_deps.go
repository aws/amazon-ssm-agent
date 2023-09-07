package credentialrefresher

import (
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/service/ssm"

	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/onprem"
)

const (
	ErrCodeAccessDeniedException          = "AccessDeniedException"
	ErrCodeInvalidInstanceId              = ssm.ErrCodeInvalidInstanceId
	ErrCodeMachineFingerprintDoesNotMatch = ssm.ErrCodeMachineFingerprintDoesNotMatch
	ErrCodeInternalFailure                = "InternalFailure"
	ErrCodeIncompleteSignature            = "IncompleteSignature"
	ErrCodeInvalidAction                  = "InvalidAction"
	ErrCodeInvalidClientTokenId           = "InvalidClientTokenId"
	ErrCodeNotAuthorized                  = "NotAuthorized"
	ErrCodeOptInRequired                  = "OptInRequired"
	ErrCodeServiceUnavailable             = "ServiceUnavailable"
	ErrCodeThrottlingException            = "ThrottlingException"
	ErrCodeValidationError                = "ValidationError"
	ErrCodeRateExceeded                   = "RateExceeded"
)

type getBackoffDurationFunc func(attemptNumber int) time.Duration

var identityGetDurationMaps = map[string]map[string]getBackoffDurationFunc{
	ec2.IdentityType:    ec2ErrorCodeGetBackoffDurationMap,
	onprem.IdentityType: onPremErrorCodeGetBackoffDurationMap,
}

// Sleep up to 30 minutes to retry access denied exceptions on EC2
var ec2ErrorCodeGetBackoffDurationMap = map[string]getBackoffDurationFunc{
	ErrCodeAccessDeniedException: getEc2LongSleepDuration,
	ErrCodeInvalidInstanceId:     getEc2LongSleepDuration,
}

// Check if error is a non-retryable error if fingerprint changes or response is access denied exception
var onPremErrorCodeGetBackoffDurationMap = map[string]getBackoffDurationFunc{
	ErrCodeAccessDeniedException:          getLongSleepDuration,
	ErrCodeMachineFingerprintDoesNotMatch: getLongSleepDuration,
	ErrCodeInvalidInstanceId:              getDefaultBackoffRetryJitterSleepDuration,
}

// Non-SSM specific errors
// https://docs.aws.amazon.com/awssupport/latest/APIReference/CommonErrors.html
var defaultErrorCodeGetDurationMap = map[string]getBackoffDurationFunc{
	ErrCodeAccessDeniedException: getLongSleepDuration,
	ErrCodeIncompleteSignature:   getLongSleepDuration,
	ErrCodeInternalFailure:       getDefaultBackoffRetryJitterSleepDuration,
	ErrCodeInvalidAction:         getLongSleepDuration,
	ErrCodeInvalidClientTokenId:  getLongSleepDuration,
	ErrCodeNotAuthorized:         getLongSleepDuration,
	ErrCodeOptInRequired:         getLongSleepDuration,
	ErrCodeServiceUnavailable:    getLongSleepDuration,
	ErrCodeThrottlingException:   getDefaultBackoffRetryJitterSleepDuration,
	ErrCodeValidationError:       getLongSleepDuration,
	ErrCodeRateExceeded:          getDefaultBackoffRetryJitterSleepDuration,
}

// Map of known http responses when requesting credentials from systems manager
var httpStatusCodeGetDurationMap = map[int]getBackoffDurationFunc{
	http.StatusInternalServerError: getDefaultBackoffRetryJitterSleepDuration,
	http.StatusTooManyRequests:     getDefaultBackoffRetryJitterSleepDuration,
	http.StatusBadRequest:          getLongSleepDuration,
	http.StatusUnauthorized:        getLongSleepDuration,
	http.StatusForbidden:           getLongSleepDuration,
	http.StatusMethodNotAllowed:    getLongSleepDuration,
	http.StatusNotFound:            getLongSleepDuration,
}

func getDefaultBackoffRetryJitterSleepDuration(retryCount int) time.Duration {
	expBackoff := math.Pow(2, float64(retryCount))
	return time.Duration(int(expBackoff)+rand.Intn(int(math.Ceil(expBackoff*0.2)))) * time.Second
}

func getLongSleepDuration(_ int) time.Duration {
	// Sleep 24 hours with random jitter of up to 2 hour if error is
	// non-retryable to make sure we spread retries for large de-registered fleets
	jitter := time.Second * time.Duration(rand.Intn(7200))
	return 24*time.Hour + jitter
}

func getEc2LongSleepDuration(_ int) time.Duration {
	// Sleep 25 minutes with random jitter of up to 5 minutes on AuthN/AuthZ failures
	// to make sure we spread role token requests from instances not yet onboarded to Default Host Management
	jitter := time.Second * time.Duration(rand.Intn(300))
	return 25*time.Minute + jitter
}
