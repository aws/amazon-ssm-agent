package stubs

import (
	"context"
	"errors"
	"fmt"

	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
)

func TestInnerProvider_RetrieveWithContext(t *testing.T) {
	now := time.Now()
	expectedExpiry := now.Add(1 * time.Hour)
	testCases := []struct {
		name           string
		provider       *InnerProvider
		ctx            context.Context
		expectedValue  aws.Credentials
		expectedError  error
		expectedExpiry time.Time
	}{
		{
			name: "Successful Retrieval",
			provider: &InnerProvider{
				ProviderName: "test-provider",
				Expiry:       expectedExpiry,
			},
			ctx: context.Background(),
			expectedValue: aws.Credentials{
				Source:  "test-provider",
				Expires: expectedExpiry,
			},
			expectedError: nil,
		},
		{
			name: "Retrieval with Error",
			provider: &InnerProvider{
				RetrieveErr: errors.New("retrieval failed"),
			},
			ctx:           context.Background(),
			expectedValue: aws.Credentials{},
			expectedError: errors.New("retrieval failed"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualValue, actualErr := tc.provider.Retrieve(tc.ctx)

			assert.Equal(t, tc.expectedValue, actualValue)

			if tc.expectedError != nil {
				assert.EqualError(t, actualErr, tc.expectedError.Error())
			} else {
				assert.NoError(t, actualErr)
			}
		})
	}
}

func TestInnerProvider_IsExpired(t *testing.T) {
	testCases := []struct {
		name           string
		retrieveErr    error
		expectedResult bool
	}{
		{
			name:           "No Error - Not Expired",
			retrieveErr:    nil,
			expectedResult: false,
		},
		{
			name:           "With Retrieve Error - Considered Expired",
			retrieveErr:    fmt.Errorf("retrieval error"),
			expectedResult: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			provider := &InnerProvider{
				RetrieveErr: tc.retrieveErr,
			}

			// Act
			result := provider.IsExpired()

			// Assert
			assert.Equal(t, tc.expectedResult, result,
				"IsExpired() should return %v when RetrieveErr is %v",
				tc.expectedResult, tc.retrieveErr)
		})
	}
}

func TestInnerProviderRetrieve(t *testing.T) {
	testCases := []struct {
		name           string
		mockProvider   *InnerProvider
		expectedValue  aws.Credentials
		expectedError  error
		expectedExpiry time.Time
	}{
		{
			name: "Successful Credential Retrieval",
			mockProvider: &InnerProvider{
				ProviderName: "TestProvider",
				Expiry:       time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
			},
			expectedValue: aws.Credentials{
				Source:  "TestProvider",
				Expires: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
			},
			expectedError: nil,
		},
		{
			name: "Retrieval Error",
			mockProvider: &InnerProvider{
				RetrieveErr: errors.New("retrieval failed"),
			},
			expectedValue: aws.Credentials{},
			expectedError: errors.New("retrieval failed"),
		},
		{
			name: "Expired Credentials",
			mockProvider: &InnerProvider{
				ProviderName: "",
				Expiry:       time.Now().Add(-1 * time.Hour),
				RetrieveErr:  errors.New("credentials expired"),
			},
			expectedValue: aws.Credentials{},
			expectedError: errors.New("credentials expired"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			actualValue, actualErr := tc.mockProvider.Retrieve(context.TODO())

			// Assert
			assert.Equal(t, tc.expectedValue, actualValue)

			if tc.expectedError != nil {
				assert.EqualError(t, actualErr, tc.expectedError.Error())
			} else {
				assert.NoError(t, actualErr)
			}
		})
	}
}

func TestInnerProviderSetExpiration(t *testing.T) {
	t.Run("Set Expiration with Window", func(t *testing.T) {
		provider := &InnerProvider{}

		// Choose a reference time
		referenceTime := time.Now().Add(2 * time.Hour)
		window := 15 * time.Minute

		// Set the expiration
		provider.SetExpiration(referenceTime, window)

		// Check that the expiry is set correctly (before the reference time by the window)
		expectedExpiry := referenceTime.Add(-window)
		assert.Equal(t, expectedExpiry, provider.Expiry,
			"SetExpiration should adjust the expiry time by the specified window")
	})
}

func TestInnerProviderExpiresAt(t *testing.T) {
	testCases := []struct {
		name           string
		setupProvider  func() *InnerProvider
		expectedResult time.Time
	}{
		{
			name: "Normal Expiration Time",
			setupProvider: func() *InnerProvider {
				futureTime := time.Now().Add(1 * time.Hour)
				return &InnerProvider{
					Expiry: futureTime,
				}
			},
			expectedResult: time.Now().Add(1 * time.Hour),
		},
		{
			name: "Past Expiration Time",
			setupProvider: func() *InnerProvider {
				pastTime := time.Now().Add(-1 * time.Hour)
				return &InnerProvider{
					Expiry: pastTime,
				}
			},
			expectedResult: time.Now().Add(-1 * time.Hour),
		},
		{
			name: "Zero Time Expiration",
			setupProvider: func() *InnerProvider {
				return &InnerProvider{
					Expiry: time.Time{},
				}
			},
			expectedResult: time.Time{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := tc.setupProvider()

			result := provider.ExpiresAt()

			// Use assert to compare times, allowing small time differences
			assert.WithinDuration(t, tc.expectedResult, result, time.Second,
				"ExpiresAt() should return the correct expiration time")
		})
	}
}
