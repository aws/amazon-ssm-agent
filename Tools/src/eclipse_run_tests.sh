. build/private/bgo_exports.sh

# run gofmt, golint, govet, insert license header where necessary and then run tests
# NOTE: this script has to be executed from the project root
PROJROOT=`pwd`
echo Project root $PROJROOT

# hack: to have tests compile in eclipse, we use _testt.go instead of _test.go as suffix file names
# when we want to actually run the tests, we have to rename the files and run go test
find $PROJROOT/src/aws-ssm-agent -name "*_testt.go*" | sed 's/\(.*\)_testt.go/mv \1_testt.go \1_test.go/' | while read line ; do echo $line; `$line`; done #| sh

# insert license header where needed
$PROJROOT/Tools/src/insert_license.sh

# run gofmt
gofmt -l -w $PROJROOT/src/aws-ssm-agent

# get golint if missing
if [ ! -f $PROJROOT/Tools/bin/golint ]
then
	echo Installing golint...
	GOPATH=$PROJROOT/Tools go get -u github.com/golang/lint/golint
fi

GOPATH=$PROJROOT/vendor:$PROJROOT

# run golint
$PROJROOT/Tools/bin/golint $PROJROOT/src/aws-ssm-agent/...

# run govet
go tool vet $PROJROOT/src/aws-ssm-agent

# run regular tests by default; add -v flag for verbose output
go test aws-ssm-agent/...
# Use this if you want to run integration tests as well
#go test -tags=integration aws-ssm-agent/...

