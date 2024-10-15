package credentialrefresher

import (
	"math"
	"math/rand"
	"time"

	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/onprem"
	"github.com/aws/aws-sdk-go/service/ssm"
)

const (
	ErrCodeAccessDeniedException          = "AccessDeniedException"
	ErrCodeInvalidInstanceId              = ssm.ErrCodeInvalidInstanceId
	ErrCodeMachineFingerprintDoesNotMatch = ssm.ErrCodeMachineFingerprintDoesNotMatch
	ErrAllOtherAWSErrors                  = "ErrAllOtherAWSErrors"
	ErrAllOtherNonAWSErrors               = "ErrAllOtherNonAWSErrors"
	maxSleepDurationSeconds               = 1800 // 30 minutes cap on sleep duration.
	maxSleepDurationJitterSeconds         = 300  // 5 minutes jitter on max sleep
)

type getBackoffDurationFunc func(attemptNumber int) time.Duration

var identityGetDurationMaps = map[string]map[string]getBackoffDurationFunc{
	ec2.IdentityType:    ec2ErrorCodeGetBackoffDurationMap,
	onprem.IdentityType: onPremErrorCodeGetBackoffDurationMap,
}

// Sleep up to 30 minutes to retry access denied exceptions on EC2
var ec2ErrorCodeGetBackoffDurationMap = map[string]getBackoffDurationFunc{
	ErrCodeAccessDeniedException:          getLongSleepDuration,
	ErrCodeMachineFingerprintDoesNotMatch: getLongSleepDuration,
	ErrAllOtherAWSErrors:                  getEC2DefaultSSMSleepDuration,
	ErrAllOtherNonAWSErrors:               getEC2DefaultSSMSleepDuration,
}

// Check if error is a non-retryable error if fingerprint changes or response is access denied exception
var onPremErrorCodeGetBackoffDurationMap = map[string]getBackoffDurationFunc{
	ErrCodeAccessDeniedException:          getLongSleepDuration,
	ErrCodeMachineFingerprintDoesNotMatch: getLongSleepDuration,
	ErrAllOtherAWSErrors:                  getMediumBackoffRetryJitterSleepDuration,
	ErrAllOtherNonAWSErrors:               getDefaultBackoffRetryJitterSleepDuration,
}

// getDefaultBackoffRetryJitterSleepDuration returns sleep duration with 2^retry_count formula with 20% of it value as jitter
func getDefaultBackoffRetryJitterSleepDuration(retryCount int) time.Duration {
	expBackoff := math.Pow(2, float64(retryCount))
	sleepTimeInSeconds := int(expBackoff) + rand.Intn(int(math.Ceil(expBackoff*0.2)))
	if sleepTimeInSeconds > maxSleepDurationSeconds {
		sleepTimeInSeconds = maxSleepDurationSeconds - rand.Intn(maxSleepDurationJitterSeconds)
	}
	return time.Duration(sleepTimeInSeconds) * time.Second
}

// getMediumBackoffRetryJitterSleepDuration returns sleep duration with 2^retry_count formula with 20% of it value as jitter
// with additional 10-20 seconds added to it.
func getMediumBackoffRetryJitterSleepDuration(retryCount int) time.Duration {
	expBackoff := math.Pow(2, float64(retryCount))
	sleepTimeInSeconds := int(expBackoff) + rand.Intn(int(math.Ceil(expBackoff*0.2))) + 10 + rand.Intn(10)
	if sleepTimeInSeconds > maxSleepDurationSeconds {
		sleepTimeInSeconds = maxSleepDurationSeconds - rand.Intn(maxSleepDurationJitterSeconds)
	}
	return time.Duration(sleepTimeInSeconds) * time.Second
}

// getLongSleepDuration returns sleep duration between 25 minutes and 30 minutes
func getLongSleepDuration(retryCount int) time.Duration {
	// Sleep 25 minutes with random jitter of up to 5 minutes on AuthN/AuthZ failures
	// to make sure we spread role token requests from instances not yet onboarded to Default Host Management
	jitter := time.Second * time.Duration(rand.Intn(300))
	return 25*time.Minute + jitter
}

// getEC2DefaultSSMSleepDuration returns the duration similar to existing retry logic in hibernation module in agent.
// Min Value will be 5 minutes and max value will be 1 hr
func getEC2DefaultSSMSleepDuration(retryCount int) time.Duration {
	// These values are picked from hibernation go module in our agent package
	defaultHealthPingRate := 5 * 60 // 5 minutes
	maxInterval64 := float64(3600)

	// return 5 minutes if retry count is equal to zero
	if retryCount == 0 {
		return time.Duration(defaultHealthPingRate) * time.Second
	}

	// set maximum value when retry count > 16 to reduce computation
	if retryCount > 16 {
		return getSleepWithRandomJitter(maxInterval64)
	}

	retryCountFloat64 := float64(retryCount)
	multiplierFloat64 := float64(2)
	backOffRateFloat64 := float64(3)

	retryVal := int(math.Ceil(retryCountFloat64 / backOffRateFloat64))
	retryVal = int(math.Pow(2, float64(retryVal-1)))

	sleepTime := defaultHealthPingRate * int(multiplierFloat64) * retryVal
	sleepDuration := getSleepWithRandomJitter(float64(sleepTime))

	// if sleep duration is greater than 3600, then set it to 1 hour
	if sleepDuration.Seconds() >= maxInterval64 {
		sleepDuration = getSleepWithRandomJitter(maxInterval64)
	}
	return sleepDuration
}

func getSleepWithRandomJitter(interval float64) time.Duration {
	jitterDuration := 0
	randJitter := int(math.Ceil(interval * 0.1)) // jitter duration will be 10% of the above formula
	maxInterval := int(interval) - randJitter
	if randJitter != 0 {
		jitterDuration = rand.Intn(randJitter)
	}
	return time.Duration(maxInterval+jitterDuration) * time.Second
}
