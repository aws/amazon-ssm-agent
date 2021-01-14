package creds

import (
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/defaults"
)

func GetRemoteCreds() *credentials.Credentials {
	cfg := defaults.Config()
	handlers := defaults.Handlers()
	remoteCreds := defaults.RemoteCredProvider(*cfg, handlers)

	return credentials.NewCredentials(remoteCreds)
}

func GetDefaultCreds() *credentials.Credentials {
	cfg := defaults.Config()
	handlers := defaults.Handlers()

	return defaults.CredChain(cfg, handlers)
}
