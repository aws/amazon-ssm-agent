BUILDFILE_PATH := ./build/private/bgo_exports.makefile

# Using the wildcard function to check if file exists
ifneq ("$(wildcard $(BUILDFILE_PATH))","")
	include $(BUILDFILE_PATH)
	GOTEMPPATH := $(BGO_SPACE)/build/private
	GOTEMPCOPYPATH := $(GOTEMPPATH)/src/github.com/aws/amazon-ssm-agent
	GOPATH := $(GOTEMPPATH):$(BGO_SPACE)/vendor:$(GOPATH)
	FINALIZE := $(shell command -v bgo-final 2>/dev/null)

else
#   Initailize workspace if it's empty
	ifeq ($(WORKSPACE),)
		WORKSPACE := $(shell pwd)/../../../../
	endif

#   Initailize BGO_SPACE
	export BGO_SPACE=$(shell pwd)
	path := $(BGO_SPACE)/vendor:$(WORKSPACE)
	ifneq ($(GOPATH),)
		GOPATH := $(path):$(GOPATH)
	else
		GOPATH := $(path)
	endif
endif
export GOPATH

checkstyle::
#   Run checkstyle script
	$(BGO_SPACE)/Tools/src/checkstyle.sh

coverage:: build-linux
	$(BGO_SPACE)/Tools/src/coverage.sh github.com/aws/amazon-ssm-agent/agent/...

build:: pre-build build-linux build-windows build-linux-386 build-windows-386
	@echo "Build amazon-ssm-agent"
	@echo "GOPATH=$(GOPATH)"
	rm -rf $(BGO_SPACE)/build/bin/ $(BGO_SPACE)/vendor/bin/ $(BGO_SPACE)/vendor/pkg/
	mkdir -p $(BGO_SPACE)/bin/
	cp $(BGO_SPACE)/Tools/src/PipelineRunTests.sh $(BGO_SPACE)/bin/
	cp $(BGO_SPACE)/amazon-ssm-agent.json $(BGO_SPACE)/bin/amazon-ssm-agent.json
	cp $(BGO_SPACE)/seelog.xml $(BGO_SPACE)/bin/
	cp $(BGO_SPACE)/VERSION $(BGO_SPACE)/bin/
	cp $(BGO_SPACE)/agent/integration-cli/integration-cli.json $(BGO_SPACE)/bin/
	exit 0

package:: create-package-folder package-linux package-windows

release:: clean checkstyle coverage build package
ifneq ($(FINALIZE),)
	bgo-final
endif

clean::
	rm -rf build/* bin/ pkg/ vendor/bin/ vendor/pkg/
	find . -type f -name '*.log' -delete

.PHONY: pre-build
pre-build:
	go run $(BGO_SPACE)/agent/version/versiongenerator/version-gen.go
	for file in $(BGO_SPACE)/Tools/src/*.sh; do chmod 755 $$file; done

.PHONY: build-linux
build-linux: checkstyle copy-src
	@echo "Build for linux agent"
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_amd64/amazon-ssm-agent -v $(BGO_SPACE)/agent/agent.go
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_amd64/integration-cli -v $(BGO_SPACE)/agent/integration-cli/agentconsole.go
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_amd64/updater -v $(BGO_SPACE)/agent/update/updater/updater.go

.PHONY: build-darwin
build-darwin: checkstyle copy-src
	@echo "Rebuild for darwin agent"
	GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/darwin_amd64/amazon-ssm-agent -v $(BGO_SPACE)/agent/agent.go
	GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/darwin_amd64/integration-cli -v $(BGO_SPACE)/agent/integration-cli/agentconsole.go
	GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/darwin_amd64/updater -v $(BGO_SPACE)/agent/update/updater/updater.go

.PHONY: build-windows
build-windows: checkstyle copy-src
	@echo "Rebuild for windows agent"
	GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/windows_amd64/amazon-ssm-agent.exe -v $(BGO_SPACE)/agent/agent.go
	GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/windows_amd64/integration-cli.exe -v $(BGO_SPACE)/agent/integration-cli/agentconsole.go
	GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/windows_amd64/updater.exe -v $(BGO_SPACE)/agent/update/updater/updater.go

.PHONY: build-linux-386
build-linux-386: checkstyle copy-src
	@echo "Build for linux agent"
	GOOS=linux GOARCH=386 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_386/amazon-ssm-agent -v $(BGO_SPACE)/agent/agent.go
	GOOS=linux GOARCH=386 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_386/integration-cli -v $(BGO_SPACE)/agent/integration-cli/agentconsole.go
	GOOS=linux GOARCH=386 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_386/updater -v $(BGO_SPACE)/agent/update/updater/updater.go

.PHONY: build-darwin-386
build-darwin-386: checkstyle copy-src
	@echo "Rebuild for darwin agent"
	GOOS=darwin GOARCH=386 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/darwin-386/amazon-ssm-agent -v $(BGO_SPACE)/agent/agent.go
	GOOS=darwin GOARCH=386 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/darwin-386/integration-cli -v $(BGO_SPACE)/agent/integration-cli/agentconsole.go
	GOOS=darwin GOARCH=386 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/darwin-386/updater -v $(BGO_SPACE)/agent/update/updater/updater.go

.PHONY: build-windows-386
build-windows-386: checkstyle copy-src
	@echo "Rebuild for windows agent"
	GOOS=windows GOARCH=386 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/windows_386/amazon-ssm-agent.exe -v $(BGO_SPACE)/agent/agent.go
	GOOS=windows GOARCH=386 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/windows_386/integration-cli.exe -v $(BGO_SPACE)/agent/integration-cli/agentconsole.go
	GOOS=windows GOARCH=386 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/windows_386/updater.exe -v $(BGO_SPACE)/agent/update/updater/updater.go

.PHONY: copy-src
copy-src:
ifneq ("$(wildcard $(BUILDFILE_PATH))","")
	rm -rf $(GOTEMPCOPYPATH)
	mkdir -p $(GOTEMPCOPYPATH)
	@echo "copying files to $(GOTEMPCOPYPATH)"
	cp -r $(BGO_SPACE)/agent $(GOTEMPCOPYPATH)
endif

.PHONY: create-package-folder
create-package-folder:
	mkdir -p $(BGO_SPACE)/bin/updates/amazon-ssm-agent/`cat $(BGO_SPACE)/VERSION`/
	mkdir -p $(BGO_SPACE)/bin/updates/amazon-ssm-agent-updater/`cat $(BGO_SPACE)/VERSION`/

.PHONY: package-linux
package-linux: package-rpm-386 package-deb-386 package-rpm package-deb
	$(BGO_SPACE)/Tools/src/create_linux_package.sh

.PHONY: package-windows
package-windows: package-win-386 package-win
	$(BGO_SPACE)/Tools/src/create_windows_package.sh

.PHONY: package-rpm
package-rpm:
	$(BGO_SPACE)/Tools/src/create_rpm.sh

.PHONY: package-deb
package-deb:
	$(BGO_SPACE)/Tools/src/create_deb.sh

.PHONY: package-win
package-win:
	$(BGO_SPACE)/Tools/src/create_win.sh

.PHONY: package-rpm-386
package-rpm-386:
	$(BGO_SPACE)/Tools/src/create_rpm_386.sh

.PHONY: package-deb-386
package-deb-386:
	$(BGO_SPACE)/Tools/src/create_deb_386.sh

.PHONY: package-win-386
package-win-386:
	$(BGO_SPACE)/Tools/src/create_win_386.sh

.PHONY: get-tools
get-tools:
	go get -u github.com/nsf/gocode
	go get -u golang.org/x/tools/cmd/oracle
	go get -u golang.org/x/tools/go/loader
	go get -u golang.org/x/tools/go/types

.PHONY: quick-integtest
quick-integtest: copy-src
	# if you want to restrict to some specific package, sample below
	# go test -v -gcflags "-N -l" -tags=integration github.com/aws/amazon-ssm-agent/agent/fileutil/...
	go test -gcflags "-N -l" -tags=integration github.com/aws/amazon-ssm-agent/agent/...

.PHONY: gen-report
gen-report:
	$(BGO_SPACE)/Tools/src/gen-report.sh
