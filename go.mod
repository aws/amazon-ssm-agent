module github.com/aws/amazon-ssm-agent

go 1.26

replace github.com/aws/aws-sdk-go-v2 => ./extra/aws-sdk-go-v2

replace github.com/aws/aws-sdk-go-v2/service/ssm => ./extra/aws-sdk-go-v2/service/ssm

replace github.com/nightlyone/lockfile => ./extra/lockfile

godebug containermaxprocs=0 // disable consideration of cgroup limits

require (
	github.com/Jeffail/gabs v1.0.0
	github.com/Workiva/go-datastructures v1.0.53
	github.com/carlescere/scheduler v0.0.0-20150615230211-9b78eac89dfb
	github.com/cenkalti/backoff/v4 v4.0.2
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575
	github.com/coreos/go-semver v0.2.0
	github.com/creack/pty v1.1.11
	github.com/digitalocean/go-smbios v0.0.0-20180907143718-390a4f403a8e
	github.com/fsnotify/fsnotify v1.5.1
	github.com/go-git/go-git/v5 v5.19.2
	github.com/google/go-github/v61 v61.0.0
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/google/uuid v1.6.0
	github.com/gorhill/cronexpr v0.0.0-20180427100037-88b0669f7d75
	github.com/gorilla/websocket v1.5.3
	github.com/hectane/go-acl v0.0.0-20151103031024-7f56832555fc // Don't update -- breaks
	github.com/mitchellh/go-ps v1.0.0
	github.com/nightlyone/lockfile v0.0.0
	github.com/stretchr/testify v1.11.1
	github.com/xtaci/smux v1.5.15
	github.com/yusufpapurcu/wmi v1.2.4
	go.nanomsg.org/mangos/v3 v3.3.0
	golang.org/x/crypto v0.56.0
	golang.org/x/net v0.57.0
	golang.org/x/oauth2 v0.28.0
	golang.org/x/sync v0.21.0
	golang.org/x/sys v0.47.0
	gopkg.in/ini.v1 v1.62.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/ProtonMail/go-crypto v1.4.1
	github.com/aws/aws-sdk-go-v2 v1.41.5
	github.com/aws/aws-sdk-go-v2/config v1.31.7
	github.com/aws/aws-sdk-go-v2/credentials v1.18.11
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.7
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.19.5
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.51.1
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.65.0
	github.com/aws/aws-sdk-go-v2/service/kms v1.45.6
	github.com/aws/aws-sdk-go-v2/service/s3 v1.97.3
	github.com/aws/aws-sdk-go-v2/service/ssm v1.60.0
	github.com/aws/aws-sdk-go-v2/service/sts v1.38.3
	github.com/aws/smithy-go v1.24.2
)

require (
	dario.cat/mergo v1.0.2 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.8 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.21 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.21 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.22 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.21 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.21 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.29.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.34.3 // indirect
	github.com/cloudflare/circl v1.6.3 // indirect
	github.com/cyphar/filepath-securejoin v0.6.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.9.0 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/kevinburke/ssh_config v1.6.0 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/pjbgf/sha1cd v0.6.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sergi/go-diff v1.4.0 // indirect
	github.com/skeema/knownhosts v1.3.2 // indirect
	github.com/smartystreets/goconvey v1.8.1 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
