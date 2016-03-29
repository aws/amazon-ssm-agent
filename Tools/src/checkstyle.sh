echo "Running checkstyle script"

# run gofmt
echo "Run 'gofmt'"
unformatted=$(gofmt -l `pwd`/agent/)
if [[ -n $unformatted ]]; then
	echo >&2 "Error: Found files not formatted by gofmt"
	for f in $unformatted; do
		echo >&2 $f
	done
	echo "Please run 'gofmt -w' for files listed."
	exit 1
fi

# run goimports
echo "Try update 'goimports'"
GOPATH=`pwd`/Tools go get golang.org/x/tools/cmd/goimports

echo "Run 'goimports'"
unformatted=$(Tools/bin/goimports -l `pwd`/agent/)
if [[ -n $unformatted ]]; then
	echo >&2 "Error: Found files not formatted by goimports"
	for f in $unformatted; do
		echo >&2 $f
	done
	echo "Please run 'goimports -w' for files listed."
	exit 1
fi

# run golint
echo "Try update 'golint'"
GOPATH=`pwd`/Tools go get -u github.com/golang/lint/golint

echo "Run 'golint'"
Tools/bin/golint `pwd`/agent

# run govet
echo "Try update 'go vet'"
GOPATH=`pwd`/Tools go get -u golang.org/x/tools/cmd/vet

echo "Run 'go vet'"
go tool vet `pwd`/agent
