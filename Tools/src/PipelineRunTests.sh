#!/usr/bin/env bash
echo "Running Integration Tests"

mkdir -p src/github.com/aws/amazon-ssm-agent
cp -r agent src/github.com/aws/amazon-ssm-agent

GOROOT=`pwd`/test-runtime/lib CGO_ENABLED=0  GOPATH=`pwd`:`pwd`/vendor:`pwd`/test-runtime/lib `pwd`/test-runtime/lib/bin/go test -v --tags=integration github.com/aws/amazon-ssm-agent/agent/...
exitCode=$?

if [[ $exitCode == 0 ]]
then
		echo "TEST PASSED."
else
		echo "TEST FAILED."
fi

rm -rf src/github.com

exit $exitCode
