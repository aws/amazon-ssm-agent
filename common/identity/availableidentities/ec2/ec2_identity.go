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
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/backoffconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/registration"
	"github.com/aws/amazon-ssm-agent/agent/ssm/authregister"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ec2roleprovider"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/sharedprovider"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ssmec2roleprovider"
	"github.com/aws/amazon-ssm-agent/common/identity/endpoint"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/cenkalti/backoff/v4"
)

var (
	newSharedCredentialsProvider = sharedprovider.NewCredentialsProvider
	newAuthRegisterService       = authregister.NewClient
	newImdsClient                = ec2metadata.New
	updateServerInfo             = registration.UpdateServerInfo
	getStoredInstanceId          = registration.InstanceID
	getStoredPrivateKey          = registration.PrivateKey
	getStoredPrivateKeyType      = registration.PrivateKeyType
	backoffRetry                 = backoff.Retry
	exponentialBackoffCfg        = backoffconfig.GetDefaultExponentialBackoff
	loadAppConfig                = appconfig.Config
)

// InstanceID returns the managed instance id
func (i *Identity) InstanceID() (string, error) {
	return i.Client.GetMetadata(ec2InstanceIDResource)
}

// Region returns the region of the ec2 instance
func (i *Identity) Region() (region string, err error) {
	if region, err = i.Client.Region(); err == nil {
		return
	}
	var document ec2metadata.EC2InstanceIdentityDocument
	if document, err = i.Client.GetInstanceIdentityDocument(); err == nil {
		region = document.Region
	}

	return
}

// AvailabilityZone returns the availabilityZone ec2 instance
func (i *Identity) AvailabilityZone() (string, error) {
	return i.Client.GetMetadata(ec2AvailabilityZoneResource)
}

// AvailabilityZoneId returns the availabilityZoneId ec2 instance
func (i *Identity) AvailabilityZoneId() (string, error) {
	return i.Client.GetMetadata(ec2AvailabilityZoneResourceId)
}

// InstanceType returns the instance type of the ec2 instance
func (i *Identity) InstanceType() (string, error) {
	return i.Client.GetMetadata(ec2InstanceTypeResource)
}

// Credentials initializes credentials for EC2 identity if none exists and returns credentials
// Since credentials expire in about 6 hours, setting the ExpiryWindow to 5 hours
// will trigger a refresh 5 hours before they actually expire. So the TTL of credentials
// is reduced to about 1 hour to match EC2 assume role frequency.
func (i *Identity) Credentials() *credentials.Credentials {
	i.shareLock.Lock()
	defer i.shareLock.Unlock()

	if i.credentials != nil {
		return i.credentials
	}

	// this condition is to make newer workers compatible with older agent
	// Older core agent does not populate ShareFile and ShareProfile for EC2.
	// Hence, we use IPR provider instead of Shared Provider when these values are blank
	if configVal, err := i.runtimeConfigClient.GetConfig(); err == nil {
		if strings.TrimSpace(configVal.ShareProfile) == "" || strings.TrimSpace(configVal.ShareFile) == "" {
			// in this case, inner provider will always return IPR
			return credentials.NewCredentials(i.credentialsProvider.GetInnerProvider())
		}
	}

	i.initSharedCreds()
	return i.credentials
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
	macs, err := i.Client.GetMetadata(ec2MacsResource)
	if err != nil {
		return map[string][]string{}, err
	}

	addresses := strings.Split(macs, "\n")
	ipv4 := make([]string, len(addresses))
	ipv6 := make([]string, len(addresses))

	for index, address := range addresses {
		ipv4[index], _ = i.Client.GetMetadata(ec2MacsResource + "/" + address + "/" + ec2VpcCidrBlockV4Resource)
		ipv6[index], _ = i.Client.GetMetadata(ec2MacsResource + "/" + address + "/" + ec2VpcCidrBlockV6Resource)
	}

	return map[string][]string{"ipv4": ipv4, "ipv6": ipv6}, nil
}

// CredentialProvider returns the initialized credentials provider
func (i *Identity) CredentialProvider() credentialproviders.IRemoteProvider {
	return i.credentialsProvider
}

// Register registers the EC2 identity with Systems Manager
func (i *Identity) Register() error {
	registrationInfo := i.loadRegistrationInfo()
	if registrationInfo != nil {
		i.Log.Debugf("registration info found for ec2 instance")
		i.registrationReadyChan <- registrationInfo
		return nil
	}

	i.Log.Infof("no registration info found for ec2 instance, attempting registration")
	publicKey, privateKey, keyType, err := registration.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("error generating signing keys. %w", err)
	}

	region, err := i.Region()
	if err != nil {
		return fmt.Errorf("unable to get region for identity %w", err)
	}

	instanceId, err := i.InstanceID()
	if err != nil {
		return fmt.Errorf("unable to get instance id for identity %w", err)
	}

	i.Log.Debug("checking write access before registering")
	err = updateServerInfo("", "", privateKey, keyType, IdentityType, registration.EC2RegistrationVaultKey)
	if err != nil {
		return fmt.Errorf("unable to save registration information. %w\nTry running as sudo/administrator.", err)
	}

	backoffConfig, err := exponentialBackoffCfg()
	if err != nil {
		return fmt.Errorf("unable to set up backoff config for registration. Aborting. %w", err)
	}

	_, err = i.authRegisterService.RegisterManagedInstance(publicKey, keyType, instanceId, "", "")
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == ssm.ErrCodeInstanceAlreadyRegistered {
				i.Log.Errorf("Instance appears to already be registered. Err: %v", aerr)
				close(i.registrationReadyChan)
				return nil
			}

		}

		return fmt.Errorf("error calling RegisterManagedInstance API: %w", err)
	}

	backoffConfig.Reset()
	err = backoffRetry(func() (err error) {
		return updateServerInfo(instanceId, region, privateKey, keyType, IdentityType, registration.EC2RegistrationVaultKey)
	}, backoffConfig)

	if err != nil {
		return fmt.Errorf("failed to update EC2 local registration info after successful registration. %w", err)
	}

	registrationInfo = &authregister.RegistrationInfo{
		PrivateKey: privateKey,
		KeyType:    keyType,
	}

	i.registrationReadyChan <- registrationInfo

	return nil
}

func (i *Identity) loadRegistrationInfo() *authregister.RegistrationInfo {
	instanceId := getStoredInstanceId(i.Log, IdentityType, registration.EC2RegistrationVaultKey)
	privateKey := getStoredPrivateKey(i.Log, IdentityType, registration.EC2RegistrationVaultKey)
	keyType := getStoredPrivateKeyType(i.Log, IdentityType, registration.EC2RegistrationVaultKey)

	if instanceId == "" || privateKey == "" || keyType == "" {
		return nil
	}

	return &authregister.RegistrationInfo{
		PrivateKey: privateKey,
		KeyType:    keyType,
	}
}

// NewEC2Identity initializes the ec2 identity
func NewEC2Identity(log log.T) *Identity {
	awsConfig := &aws.Config{}
	awsConfig = awsConfig.WithMaxRetries(3)
	sess, err := session.NewSession(awsConfig)
	if err != nil {
		log.Errorf("Failed to create session with aws config. Err: %v", err)
		return nil
	}

	config, err := loadAppConfig(true)
	if err != nil {
		log.Errorf("Failed to load app config for ec2 identity. Err: %v", err)
		return nil
	}

	log = log.WithContext("[EC2Identity]")
	identity := &Identity{
		Log:                   log,
		Config:                &config,
		registrationReadyChan: make(chan *authregister.RegistrationInfo, 1),
		shareLock:             &sync.RWMutex{},
		runtimeConfigClient:   runtimeconfig.NewIdentityRuntimeConfigClient(),
	}

	// Ensure IMDS client is initialized before attempting to get instance info
	identity.initIMDSClient(sess)
	instanceInfo, err := getInstanceInfo(identity)
	if err != nil {
		log.Error(err)
		return nil
	}

	endpointHelper := endpoint.NewEndpointHelper(log, config)
	identity.initAuthRegisterService(instanceInfo.Region)
	identity.initEc2RoleProvider(endpointHelper, instanceInfo)
	return identity
}

// initEc2RoleProvider initializes the role provider for the EC2 identity
func (i *Identity) initEc2RoleProvider(endpointHelper endpoint.IEndpointHelper, instanceInfo *ssmec2roleprovider.InstanceInfo) {
	if i.credentialsProvider != nil {
		return
	}

	ssmEC2RoleProvider := &ssmec2roleprovider.SSMEC2RoleProvider{
		ExpiryWindow:          time.Duration(0),
		Config:                i.Config,
		Log:                   i.Log.WithContext("[SSMEC2RoleProvider]"),
		IMDSClient:            i.Client,
		InstanceInfo:          instanceInfo,
		RegistrationReadyChan: i.registrationReadyChan,
	}

	iprRoleProvider := &ec2rolecreds.EC2RoleProvider{
		Client:       ec2metadata.New(session.New()),
		ExpiryWindow: time.Hour * 5, // Credentials marked as expired 5 hours before token invalid
	}

	innerProviders := &ec2roleprovider.EC2InnerProviders{
		IPRProvider:    iprRoleProvider,
		SsmEc2Provider: ssmEC2RoleProvider,
	}

	ec2RoleProvider := &ec2roleprovider.EC2RoleProvider{
		InnerProviders:         innerProviders,
		Log:                    i.Log.WithContext(ec2rolecreds.ProviderName),
		Config:                 i.Config,
		InstanceInfo:           instanceInfo,
		SsmEndpoint:            endpointHelper.GetServiceEndpoint("ssm", instanceInfo.Region),
		ShareFileLocation:      appconfig.DefaultEC2SharedCredentialsFilePath,
		CredentialProfile:      "default",
		ShouldShareCredentials: true,
	}

	i.credentialsProvider = ec2RoleProvider
}

// getInstanceInfo queries identity for instanceId and region
func getInstanceInfo(identity *Identity) (*ssmec2roleprovider.InstanceInfo, error) {
	instanceId, err := identity.InstanceID()
	if err != nil {
		err = fmt.Errorf("failed to get identity instance id. Error: %w", err)
		return nil, err
	}

	region, err := identity.Region()
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

// initSharedCreds initializes credentials using shared credentials provider that reads credentials from shared location, falls back to non-shared credentials provider for any failure
func (i *Identity) initSharedCreds() {
	if i.credentials != nil {
		return
	}

	if shareCredsProvider, err := newSharedCredentialsProvider(i.Log); err != nil {
		i.Log.Errorf("failed to initialize shared credentials provider, falling back to remote credentials provider: %v", err)
		i.initNonSharedCreds()
	} else {
		i.credentials = credentials.NewCredentials(shareCredsProvider)
	}
}

// initNonSharedCreds initializes credentials provider and credentials that do not share credentials via aws credentials file
func (i *Identity) initNonSharedCreds() {
	if i.credentials != nil {
		return
	}

	i.credentials = credentials.NewCredentials(i.credentialsProvider)
}

// initIMDSClient initializes the client used to make instance metadata service requests
func (i *Identity) initIMDSClient(sess *session.Session) {
	if i.Client != nil {
		return
	}

	i.Client = newImdsClient(sess)
}

// initAuthRegisterService initializes the client used to make requests to RegisterManagedInstance
func (i *Identity) initAuthRegisterService(region string) {
	if i.authRegisterService != nil {
		return
	}

	i.authRegisterService = newAuthRegisterService(i.Log.WithContext("[AuthRegisterService]"), region, i.Client)
}
