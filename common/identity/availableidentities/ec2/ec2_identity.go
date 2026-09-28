// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package ec2

import (
	"context"
	"errors"
	"fmt"

	"io"
	"strings"
	"sync"
	"time"

	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"github.com/aws/aws-sdk-go-v2/credentials/ec2rolecreds"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/backoffconfig"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/registration"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/ssm/authregister"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2/ec2detector"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ec2roleprovider"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/sharedprovider"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ssmec2roleprovider"
	"github.com/aws/amazon-ssm-agent/common/identity/endpoint"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/cenkalti/backoff/v4"
)

var (
	newSharedCredentialsProvider = sharedprovider.NewCredentialsProvider
	newAuthRegisterService       = authregister.NewClient
	newImdsClient                = imds.NewFromConfig
	updateServerInfo             = registration.UpdateServerInfo
	getStoredInstanceId          = registration.InstanceID
	getStoredRegion              = registration.Region
	getStoredPrivateKey          = registration.PrivateKey
	getStoredPublicKey           = registration.PublicKey
	getStoredPrivateKeyType      = registration.PrivateKeyType
	backoffRetry                 = backoff.Retry
	exponentialBackoffCfg        = backoffconfig.GetDefaultExponentialBackoff
	loadAppConfig                = appconfig.Config
)

// InstanceID returns the managed instance id
func (i *Identity) InstanceID() (string, error) {
	return i.InstanceIDWithContext(context.Background())
}

// InstanceIDWithContext returns the managed instance id
func (i *Identity) InstanceIDWithContext(ctx context.Context) (string, error) {
	metaData, err := i.Client.GetMetadata(ctx, &imds.GetMetadataInput{ec2InstanceIDResource})
	if err != nil {
		return "", err
	}

	rc := metaData.Content
	defer rc.Close()
	bytes, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}
	content := string(bytes)
	return content, nil
}

// Region returns the region of the ec2 instance
func (i *Identity) Region() (region string, err error) {
	return i.RegionWithContext(context.Background())
}

// RegionWithContext returns the region of the ec2 instance
func (i *Identity) RegionWithContext(ctx context.Context) (region string, err error) {
	regionOutput, err := i.Client.GetRegion(ctx, &imds.GetRegionInput{})
	if err == nil {
		region = regionOutput.Region
		return
	}
	var document *imds.GetInstanceIdentityDocumentOutput
	if document, err = i.Client.GetInstanceIdentityDocument(ctx, &imds.GetInstanceIdentityDocumentInput{}); err == nil {
		region = document.InstanceIdentityDocument.Region
	}

	return
}

func GetMetaDataContent(path string, i *Identity) (string, error) {
	mdInput := imds.GetMetadataInput{Path: path}
	metaData, err := i.Client.GetMetadata(context.TODO(), &mdInput)
	if err != nil {
		return "", err
	}
	rc := metaData.Content
	defer rc.Close()
	bytes, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}
	content := string(bytes)
	return content, nil
}

// AvailabilityZone returns the availabilityZone ec2 instance
func (i *Identity) AvailabilityZone() (string, error) {
	return GetMetaDataContent(ec2AvailabilityZoneResource, i)
}

// AvailabilityZoneId returns the availabilityZoneId ec2 instance
func (i *Identity) AvailabilityZoneId() (string, error) {
	return GetMetaDataContent(ec2AvailabilityZoneResourceId, i)
}

// InstanceType returns the instance type of the ec2 instance
func (i *Identity) InstanceType() (string, error) {
	return GetMetaDataContent(ec2InstanceTypeResource, i)
}

// CredentialsProvider returns CredentialsProvider for EC2 identity
func (i *Identity) CredentialsProvider() aws.CredentialsProvider {
	i.shareLock.Lock()
	defer i.shareLock.Unlock()

	return i.credentialsProvider

}

// IsIdentityEnvironment returns if instance is a ec2 instance
func (i *Identity) IsIdentityEnvironment() bool {
	_, err := i.InstanceID()
	return err == nil
}

// IdentityType returns the identity type of the ec2 instance
func (i *Identity) IdentityType() string { return IdentityType }

// VpcPrimaryCIDRBlock returns ipv4, ipv6 VPC CIDR block addresses if exists
func (i *Identity) VpcPrimaryCIDRBlock() (ip map[string][]string, err error) {
	macs, err := GetMetaDataContent(ec2MacsResource, i)
	if err != nil {
		return map[string][]string{}, err
	}

	addresses := strings.Split(macs, "\n")
	ipv4 := make([]string, len(addresses))
	ipv6 := make([]string, len(addresses))

	for index, address := range addresses {
		ipv4[index], _ = GetMetaDataContent(ec2MacsResource+"/"+address+"/"+ec2VpcCidrBlockV4Resource, i)
		ipv6[index], _ = GetMetaDataContent(ec2MacsResource+"/"+address+"/"+ec2VpcCidrBlockV6Resource, i)
	}

	return map[string][]string{"ipv4": ipv4, "ipv6": ipv6}, nil
}

// CredentialProvider returns the initialized credentials provider
func (i *Identity) CredentialProvider() credentialproviders.IRemoteProvider {
	return i.credentialsProvider
}

// Register registers the EC2 identity with Systems Manager
func (i *Identity) Register(ctx context.Context) error {
	region, err := i.RegionWithContext(ctx)
	if err != nil {
		return fmt.Errorf("unable to get region for identity %w", err)
	}

	instanceId, err := i.InstanceIDWithContext(ctx)
	if err != nil {
		return fmt.Errorf("unable to get instance id for identity %w", err)
	}

	i.Log.Info("Checking disk for registration info")
	registrationInfo := i.loadRegistrationInfo(instanceId)
	if registrationInfo.InstanceId != "" {
		i.Log.Info("Registration info found for ec2 instance")
		return nil
	}

	i.Log.Infof("No registration info found for ec2 instance, attempting registration")

	var publicKey, privateKey, keyType string
	if registrationInfo.PrivateKey != "" && registrationInfo.PublicKey != "" && registrationInfo.KeyType != "" {
		i.Log.Info("Found existing registration keypair")
		publicKey = registrationInfo.PublicKey
		privateKey = registrationInfo.PrivateKey
		keyType = registrationInfo.KeyType
	} else {
		i.Log.Info("Generating registration keypair")
		publicKey, privateKey, keyType, err = registration.GenerateKeyPair()
		if err != nil {
			return fmt.Errorf("error generating registration keypair. %w", err)
		}
	}

	i.Log.Info("Checking write access before registering")
	// TODO @khotia: do we need to register as ec2 for non-managed?
	err = updateServerInfo(instanceId, "", publicKey, privateKey, keyType, IdentityType, registration.EC2RegistrationVaultKey, "")
	if err != nil {
		return fmt.Errorf("unable to save registration information. %w\nTry running as sudo/administrator.", err)
	}

	backoffConfig, err := exponentialBackoffCfg()
	if err != nil {
		return fmt.Errorf("unable to set up backoff config for registration. Aborting. %w", err)
	}

	i.Log.Info("Registering EC2 instance with Systems Manager")
	_, err = i.AuthRegisterService.RegisterManagedInstance(ctx, publicKey, keyType, instanceId, "", "", "")
	if err != nil {
		var IARException *ssmtypes.InstanceAlreadyRegisteredException
		if errors.As(err, &IARException) {
			i.Log.Errorf("Instance appears to already be registered. Err: %v", err)
			return nil
		}

		return fmt.Errorf("error calling RegisterManagedInstance API: %w", err)
	}

	backoffConfig.Reset()
	err = backoffRetry(func() (err error) {
		return updateServerInfo(instanceId, region, publicKey, privateKey, keyType, IdentityType, registration.EC2RegistrationVaultKey, "")
	}, backoffConfig)
	if err != nil {
		return fmt.Errorf("failed to update EC2 local registration info after successful registration. %w", err)
	}

	registrationInfo = &authregister.RegistrationInfo{
		PrivateKey: privateKey,
		KeyType:    keyType,
		PublicKey:  publicKey,
		InstanceId: instanceId,
	}

	i.Log.Info("EC2 registration was successful.")
	return nil
}

func (i *Identity) loadRegistrationInfo(instanceId string) *authregister.RegistrationInfo {
	registrationInfo := &authregister.RegistrationInfo{
		InstanceId: getStoredInstanceId(i.Log, IdentityType, registration.EC2RegistrationVaultKey),
		PrivateKey: getStoredPrivateKey(i.Log, IdentityType, registration.EC2RegistrationVaultKey),
		KeyType:    getStoredPrivateKeyType(i.Log, IdentityType, registration.EC2RegistrationVaultKey),
		PublicKey:  getStoredPublicKey(i.Log, IdentityType, registration.EC2RegistrationVaultKey),
	}
	storedRegion := getStoredRegion(i.Log, IdentityType, registration.EC2RegistrationVaultKey)

	instanceIdMatch := registrationInfo.InstanceId == instanceId
	registrationConfirmed := storedRegion != ""
	hasKeypair := registrationInfo.PrivateKey != "" && registrationInfo.KeyType != ""

	if !instanceIdMatch {
		if registrationInfo.InstanceId != "" {
			i.Log.Infof("Stored instance id %s does not match current instance %s, re-initializing registration",
				registrationInfo.InstanceId, instanceId)
		}
		return &authregister.RegistrationInfo{}
	}

	if !registrationConfirmed || !hasKeypair {
		registrationInfo.InstanceId = ""
	}

	return registrationInfo
}

func NewEC2IdentityWithConfig(log log.T, imdsAwsConfig aws.Config) *Identity {
	config, err := loadAppConfig(true)
	if err != nil {
		log.Errorf("Failed to load app config for ec2 identity. Err: %v", err)
		return nil
	}

	log = log.WithContext("[EC2Identity]")
	identity := &Identity{
		Log:                 log,
		Config:              &config,
		shareLock:           &sync.RWMutex{},
		runtimeConfigClient: runtimeconfig.NewIdentityRuntimeConfigClient(),
		ec2Detector:         ec2detector.New(log),
	}

	// Ensure IMDS client is initialized before attempting to get instance info
	identity.initIMDSClient(imdsAwsConfig)
	instanceInfo, err := getInstanceInfo(context.Background(), identity)
	if err != nil {
		log.Errorf("Failed to get instance info from IMDS. Err: %v", err)
		return nil
	}

	endpointHelper := endpoint.NewEndpointHelper(log, config)
	identity.initAuthRegisterService(instanceInfo.Region)
	identity.initEc2RoleProvider(endpointHelper, instanceInfo)
	return identity
}

func (i *Identity) SourceId() (string, error) { return i.InstanceID() }

func (i *Identity) SourceType() string { return SourceType }

func (i *Identity) SourceLocation() (string, error) { return i.Region() }

func (i *Identity) ComputerName() (string, error) { return platform.Hostname(i.Log), nil }

// NewEC2Identity initializes the ec2 identity
func NewEC2Identity(log log.T) *Identity {
	awsConfig := aws.Config{}
	// IMDS ignores RetryMaxAttempts; it only honors a Retryer func from the
	// config. Set the max attempts via a Retryer so retries are actually bounded.
	awsConfig.Retryer = func() aws.Retryer {
		return retry.NewStandard(func(o *retry.StandardOptions) {
			o.MaxAttempts = 3
		})
	}
	return NewEC2IdentityWithConfig(log, awsConfig)
}

// initEc2RoleProvider initializes the role provider for the EC2 identity
func (i *Identity) initEc2RoleProvider(endpointHelper endpoint.IEndpointHelper, instanceInfo *ssmec2roleprovider.InstanceInfo) {
	if i.credentialsProvider != nil {
		return
	}

	ssmEC2RoleProvider := &ssmec2roleprovider.SSMEC2RoleProvider{
		ExpiryWindow: time.Duration(0),
		Config:       i.Config,
		Log:          i.Log.WithContext("[SSMEC2RoleProvider]"),
		IMDSClient:   i.Client,
		InstanceInfo: instanceInfo,
	}
	// Wrap SSMEC2RoleProvider in a caching adapter so a
	// RequestManagedInstanceRoleToken call isn't issued on every SDK attempt
	// during the DHMC-fallback path. SSMEC2RoleProvider does not implement
	// HandleFailToRefresh, so the underlying aws.CredentialsCache surfaces
	// a failed Retrieve immediately instead of extending stale credentials.
	cachedSsmEC2Provider := ec2roleprovider.NewCachedInnerProvider(ssmEC2RoleProvider)

	sharedCredentialsProvider := sharedprovider.NewCredentialsProvider(i.Log)

	innerProviders := &ec2roleprovider.EC2InnerProviders{
		IPRProvider:               i.createIPRProvider(),
		SsmEc2Provider:            cachedSsmEC2Provider,
		SharedCredentialsProvider: sharedCredentialsProvider,
	}

	runtimeConfigClient := runtimeconfig.NewIdentityRuntimeConfigClient()
	ssmEndpoint := endpointHelper.GetServiceEndpoint("ssm", instanceInfo.Region)
	ec2RoleProvider := ec2roleprovider.NewEC2RoleProvider(i.Log, innerProviders, instanceInfo, ssmEndpoint, runtimeConfigClient)

	i.credentialsProvider = ec2RoleProvider
}

func (i *Identity) createIPRProvider() *IEC2RoleProviderWrapper {
	i.Log.Infof("Creating IPR provider with IMDS client")
	ec2Provider := ec2rolecreds.New(func(o *ec2rolecreds.Options) {
		// Configure options here
		o.Client = i.Client
	})
	// Do not wrap ec2Provider in aws.NewCredentialsCache. ec2rolecreds.Provider
	// implements HandleFailToRefresh, which the CredentialsCache invokes on a
	// failed Retrieve — that hook extends the returned credentials' Expires
	// time by a random 5–15 minutes and returns the previous credentials
	// without error, so a detached instance profile would keep serving stale
	// IPR credentials to callers for up to ~1 hour. Leaving the provider raw
	// (consulted via IEC2RoleProviderWrapper.Retrieve on each refresh) makes
	// role revocation take effect promptly. If a future change reintroduces
	// caching here, wrap the provider with the noFailRefreshExtension shim
	// in common/identity/credentialproviders before passing it to
	// NewCredentialsCache so HandleFailToRefresh stays disabled.

	return &IEC2RoleProviderWrapper{
		provider: ec2Provider,
		expiry:   time.Now().Add(time.Hour), // Default expiry
	}
}

// getInstanceInfo queries identity for instanceId and region
func getInstanceInfo(ctx context.Context, identity *Identity) (*ssmec2roleprovider.InstanceInfo, error) {
	instanceId, err := identity.InstanceIDWithContext(ctx)
	if err != nil {
		err = fmt.Errorf("failed to get identity instance id. Error: %w", err)
		return nil, err
	}

	region, err := identity.RegionWithContext(ctx)
	if err != nil {
		err = fmt.Errorf("failed to get identity region. Error: %w", err)
		return nil, err
	}

	instanceInfo := &ssmec2roleprovider.InstanceInfo{
		InstanceId: instanceId,
		Region:     region,
	}
	return instanceInfo, nil
}

// initIMDSClient initializes the client used to make instance metadata service requests
func (i *Identity) initIMDSClient(cfg aws.Config) {
	if i.Client != nil {
		return
	}

	i.Client = newImdsClient(cfg)
}

// initAuthRegisterService initializes the client used to make requests to RegisterManagedInstance
func (i *Identity) initAuthRegisterService(region string) {
	if i.AuthRegisterService != nil {
		return
	}

	i.AuthRegisterService = newAuthRegisterService(i.Log.WithContext("[AuthRegisterService]"), region, i.Client)
}
