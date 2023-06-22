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
	"fmt"
	"strings"
	"sync"
	"time"

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
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/cenkalti/backoff/v4"
)

var (
	newSharedCredentialsProvider = sharedprovider.NewCredentialsProvider
	newAuthRegisterService       = authregister.NewClient
	newImdsClient                = ec2metadata.New
	updateServerInfo             = registration.UpdateServerInfo
	getStoredInstanceId          = registration.InstanceID
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
	return i.Client.GetMetadataWithContext(ctx, ec2InstanceIDResource)
}

// Region returns the region of the ec2 instance
func (i *Identity) Region() (region string, err error) {
	return i.RegionWithContext(context.Background())
}

// RegionWithContext returns the region of the ec2 instance
func (i *Identity) RegionWithContext(ctx context.Context) (region string, err error) {
	if region, err = i.Client.RegionWithContext(ctx); err == nil {
		return
	}
	var document ec2metadata.EC2InstanceIdentityDocument
	if document, err = i.Client.GetInstanceIdentityDocumentWithContext(ctx); err == nil {
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

	if i.credentials == nil {
		i.credentials = credentials.NewCredentials(i.credentialsProvider)
	}

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
		i.Log.Info("Found registration keys")
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
	err = updateServerInfo("", "", publicKey, privateKey, keyType, IdentityType, registration.EC2RegistrationVaultKey)
	if err != nil {
		return fmt.Errorf("unable to save registration information. %w\nTry running as sudo/administrator.", err)
	}

	backoffConfig, err := exponentialBackoffCfg()
	if err != nil {
		return fmt.Errorf("unable to set up backoff config for registration. Aborting. %w", err)
	}

	i.Log.Info("Registering EC2 instance with Systems Manager")
	_, err = i.AuthRegisterService.RegisterManagedInstanceWithContext(ctx, publicKey, keyType, instanceId, "", "")
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == ssm.ErrCodeInstanceAlreadyRegistered {
				i.Log.Errorf("Instance appears to already be registered. Err: %v", aerr)
				return nil
			}
		}

		return fmt.Errorf("error calling RegisterManagedInstance API: %w", err)
	}

	backoffConfig.Reset()
	err = backoffRetry(func() (err error) {
		return updateServerInfo(instanceId, region, publicKey, privateKey, keyType, IdentityType, registration.EC2RegistrationVaultKey)
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

	if registrationInfo.InstanceId == "" || registrationInfo.PrivateKey == "" ||
		registrationInfo.KeyType == "" || registrationInfo.InstanceId != instanceId {
		registrationInfo.InstanceId = "" // setting it as blank to try registration
	}

	return registrationInfo
}

func NewEC2IdentityWithConfig(log log.T, imdsAwsConfig *aws.Config) *Identity {
	sess, err := session.NewSession(imdsAwsConfig)
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
		Log:                 log,
		Config:              &config,
		shareLock:           &sync.RWMutex{},
		runtimeConfigClient: runtimeconfig.NewIdentityRuntimeConfigClient(),
	}

	// Ensure IMDS client is initialized before attempting to get instance info
	identity.initIMDSClient(sess)
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

// NewEC2Identity initializes the ec2 identity
func NewEC2Identity(log log.T) *Identity {
	awsConfig := &aws.Config{}
	awsConfig = awsConfig.WithMaxRetries(3).WithEC2MetadataEnableFallback(false)
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

	iprRoleProvider := &ec2rolecreds.EC2RoleProvider{
		Client: ec2metadata.New(session.New()),
	}

	sharedCredentialsProvider := sharedprovider.NewCredentialsProvider(i.Log)

	innerProviders := &ec2roleprovider.EC2InnerProviders{
		IPRProvider:               iprRoleProvider,
		SsmEc2Provider:            ssmEC2RoleProvider,
		SharedCredentialsProvider: sharedCredentialsProvider,
	}

	runtimeConfigClient := runtimeconfig.NewIdentityRuntimeConfigClient()
	ssmEndpoint := endpointHelper.GetServiceEndpoint("ssm", instanceInfo.Region)
	ec2RoleProvider := ec2roleprovider.NewEC2RoleProvider(i.Log, i.Config, innerProviders, instanceInfo, ssmEndpoint, runtimeConfigClient)

	i.credentialsProvider = ec2RoleProvider
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
func (i *Identity) initIMDSClient(sess *session.Session) {
	if i.Client != nil {
		return
	}

	i.Client = newImdsClient(sess)
}

// initAuthRegisterService initializes the client used to make requests to RegisterManagedInstance
func (i *Identity) initAuthRegisterService(region string) {
	if i.AuthRegisterService != nil {
		return
	}

	i.AuthRegisterService = newAuthRegisterService(i.Log.WithContext("[AuthRegisterService]"), region, i.Client)
}
