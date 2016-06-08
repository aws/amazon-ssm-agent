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

build:: build-linux build-windows build-linux-386 build-windows-386

prepack:: prepack-linux prepack-linux-386 prepack-windows prepack-windows-386

package:: create-package-folder package-linux package-windows

release:: clean checkstyle coverage build prepack package

ifneq ($(FINALIZE),)
	bgo-final
endif

clean:: remove-prepacked-folder
	rm -rf build/* bin/ pkg/ vendor/bin/ vendor/pkg/ .cover/
	find . -type f -name '*.log' -delete

.PHONY: pre-build
pre-build:
	go run $(BGO_SPACE)/agent/version/versiongenerator/version-gen.go
	for file in $(BGO_SPACE)/Tools/src/*.sh; do chmod 755 $$file; done
	@echo "Build amazon-ssm-agent"
	@echo "GOPATH=$(GOPATH)"
	rm -rf $(BGO_SPACE)/build/bin/ $(BGO_SPACE)/vendor/bin/ $(BGO_SPACE)/vendor/pkg/
	mkdir -p $(BGO_SPACE)/bin/
	cp $(BGO_SPACE)/Tools/src/PipelineRunTests.sh $(BGO_SPACE)/bin/
	cp $(BGO_SPACE)/LICENSE $(BGO_SPACE)/bin/
	cp $(BGO_SPACE)/amazon-ssm-agent.json.template $(BGO_SPACE)/bin/amazon-ssm-agent.json.template
	cp $(BGO_SPACE)/seelog_unix.xml $(BGO_SPACE)/bin/
	cp $(BGO_SPACE)/seelog_windows.xml.template $(BGO_SPACE)/bin/
	cp $(BGO_SPACE)/VERSION $(BGO_SPACE)/bin/
	cp $(BGO_SPACE)/agent/integration-cli/integration-cli.json $(BGO_SPACE)/bin/
	exit 0

.PHONY: build-linux
build-linux: checkstyle copy-src pre-build
	@echo "Build for linux agent"
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_amd64/amazon-ssm-agent -v \
	$(BGO_SPACE)/agent/agent.go $(BGO_SPACE)/agent/agent_unix.go $(BGO_SPACE)/agent/agent_parser.go
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_amd64/integration-cli -v \
	$(BGO_SPACE)/agent/integration-cli/agentconsole.go
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_amd64/updater -v \
	$(BGO_SPACE)/agent/update/updater/updater.go $(BGO_SPACE)/agent/update/updater/updater_unix.go

.PHONY: build-darwin
build-darwin: checkstyle copy-src pre-build
	@echo "Rebuild for darwin agent"
	GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/darwin_amd64/amazon-ssm-agent -v \
	$(BGO_SPACE)/agent/agent.go $(BGO_SPACE)/agent/agent_unix.go $(BGO_SPACE)/agent/agent_parser.go
	GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/darwin_amd64/integration-cli -v \
	$(BGO_SPACE)/agent/integration-cli/agentconsole.go
	GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/darwin_amd64/updater -v \
	$(BGO_SPACE)/agent/update/updater/updater.go $(BGO_SPACE)/agent/update/updater/updater_unix.go

.PHONY: build-windows
build-windows: checkstyle copy-src pre-build
	@echo "Rebuild for windows agent"
	GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/windows_amd64/amazon-ssm-agent.exe -v \
	$(BGO_SPACE)/agent/agent.go $(BGO_SPACE)/agent/agent_windows.go $(BGO_SPACE)/agent/agent_parser.go
	GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/windows_amd64/integration-cli.exe -v \
	$(BGO_SPACE)/agent/integration-cli/agentconsole.go
	GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/windows_amd64/updater.exe -v \
	$(BGO_SPACE)/agent/update/updater/updater.go $(BGO_SPACE)/agent/update/updater/updater_windows.go

.PHONY: build-linux-386
build-linux-386: checkstyle copy-src pre-build
	@echo "Build for linux agent"
	GOOS=linux GOARCH=386 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_386/amazon-ssm-agent -v \
	$(BGO_SPACE)/agent/agent.go $(BGO_SPACE)/agent/agent_unix.go $(BGO_SPACE)/agent/agent_parser.go
	GOOS=linux GOARCH=386 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_386/integration-cli -v \
	$(BGO_SPACE)/agent/integration-cli/agentconsole.go
	GOOS=linux GOARCH=386 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_386/updater -v \
	$(BGO_SPACE)/agent/update/updater/updater.go $(BGO_SPACE)/agent/update/updater/updater_unix.go

.PHONY: build-darwin-386
build-darwin-386: checkstyle copy-src pre-build
	@echo "Rebuild for darwin agent"
	GOOS=darwin GOARCH=386 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/darwin_386/amazon-ssm-agent -v \
	$(BGO_SPACE)/agent/agent.go $(BGO_SPACE)/agent/agent_unix.go $(BGO_SPACE)/agent/agent_parser.go
	GOOS=darwin GOARCH=386 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/darwin_386/integration-cli -v \
	$(BGO_SPACE)/agent/integration-cli/agentconsole.go
	GOOS=darwin GOARCH=386 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/darwin_386/updater -v \
	$(BGO_SPACE)/agent/update/updater/updater.go $(BGO_SPACE)/agent/update/updater/updater_unix.go

.PHONY: build-windows-386
build-windows-386: checkstyle copy-src pre-build
	@echo "Rebuild for windows agent"
	GOOS=windows GOARCH=386 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/windows_386/amazon-ssm-agent.exe -v \
	$(BGO_SPACE)/agent/agent.go $(BGO_SPACE)/agent/agent_windows.go $(BGO_SPACE)/agent/agent_parser.go
	GOOS=windows GOARCH=386 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/windows_386/integration-cli.exe -v \
	$(BGO_SPACE)/agent/integration-cli/agentconsole.go
	GOOS=windows GOARCH=386 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/windows_386/updater.exe -v \
	$(BGO_SPACE)/agent/update/updater/updater.go $(BGO_SPACE)/agent/update/updater/updater_windows.go

.PHONY: copy-src
copy-src:
ifneq ("$(wildcard $(BUILDFILE_PATH))","")
	rm -rf $(GOTEMPCOPYPATH)
	mkdir -p $(GOTEMPCOPYPATH)
	@echo "copying files to $(GOTEMPCOPYPATH)"
	cp -r $(BGO_SPACE)/agent $(GOTEMPCOPYPATH)
endif

.PHONY: remove-prepacked-folder
remove-prepacked-folder:
	rm -rf $(BGO_SPACE)/bin/prepacked

.PHONY: prepack-linux
prepack-linux:
	mkdir -p $(BGO_SPACE)/bin/prepacked/linux_amd64
	cp $(BGO_SPACE)/bin/linux_amd64/amazon-ssm-agent $(BGO_SPACE)/bin/prepacked/linux_amd64/amazon-ssm-agent
	cp $(BGO_SPACE)/bin/linux_amd64/integration-cli $(BGO_SPACE)/bin/prepacked/linux_amd64/integration-cli
	cp $(BGO_SPACE)/bin/linux_amd64/updater $(BGO_SPACE)/bin/prepacked/linux_amd64/updater
	cp $(BGO_SPACE)/bin/amazon-ssm-agent.json.template $(BGO_SPACE)/bin/prepacked/linux_amd64/amazon-ssm-agent.json.template
	cp $(BGO_SPACE)/bin/integration-cli.json $(BGO_SPACE)/bin/prepacked/linux_amd64/integration-cli.json
	cp $(BGO_SPACE)/bin/seelog_unix.xml $(BGO_SPACE)/bin/prepacked/linux_amd64/seelog.xml
	cp $(BGO_SPACE)/bin/LICENSE $(BGO_SPACE)/bin/prepacked/linux_amd64/LICENSE

.PHONY: prepack-darwin
prepack-darwin:
	mkdir -p $(BGO_SPACE)/bin/prepacked/darwin_amd64
	cp $(BGO_SPACE)/bin/darwin_amd64/amazon-ssm-agent $(BGO_SPACE)/bin/prepacked/darwin_amd64/amazon-ssm-agent
	cp $(BGO_SPACE)/bin/darwin_amd64/integration-cli $(BGO_SPACE)/bin/prepacked/darwin_amd64/integration-cli
	cp $(BGO_SPACE)/bin/darwin_amd64/updater $(BGO_SPACE)/bin/prepacked/darwin_amd64/updater
	cp $(BGO_SPACE)/bin/amazon-ssm-agent.json.template $(BGO_SPACE)/bin/prepacked/darwin_amd64/amazon-ssm-agent.json.template
	cp $(BGO_SPACE)/bin/integration-cli.json $(BGO_SPACE)/bin/prepacked/darwin_amd64/integration-cli.json
	cp $(BGO_SPACE)/bin/seelog_unix.xml $(BGO_SPACE)/bin/prepacked/darwin_amd64/seelog.xml
	cp $(BGO_SPACE)/bin/LICENSE $(BGO_SPACE)/bin/prepacked/darwin_amd64/LICENSE

.PHONY: prepack-windows
prepack-windows:
	mkdir -p $(BGO_SPACE)/bin/prepacked/windows_amd64
	cp $(BGO_SPACE)/bin/windows_amd64/amazon-ssm-agent.exe $(BGO_SPACE)/bin/prepacked/windows_amd64/amazon-ssm-agent.exe
	cp $(BGO_SPACE)/bin/windows_amd64/integration-cli.exe $(BGO_SPACE)/bin/prepacked/windows_amd64/integration-cli.exe
	cp $(BGO_SPACE)/bin/windows_amd64/updater.exe $(BGO_SPACE)/bin/prepacked/windows_amd64/updater.exe
	cp $(BGO_SPACE)/bin/amazon-ssm-agent.json.template $(BGO_SPACE)/bin/prepacked/windows_amd64/amazon-ssm-agent.json.template
	cp $(BGO_SPACE)/bin/integration-cli.json $(BGO_SPACE)/bin/prepacked/windows_amd64/integration-cli.json
	cp $(BGO_SPACE)/bin/seelog_windows.xml.template $(BGO_SPACE)/bin/prepacked/windows_amd64/seelog.xml.template
	cp $(BGO_SPACE)/bin/LICENSE $(BGO_SPACE)/bin/prepacked/windows_amd64/LICENSE

.PHONY: prepack-linux-386
prepack-linux-386:
	mkdir -p $(BGO_SPACE)/bin/prepacked/linux_386
	cp $(BGO_SPACE)/bin/linux_386/amazon-ssm-agent $(BGO_SPACE)/bin/prepacked/linux_386/amazon-ssm-agent
	cp $(BGO_SPACE)/bin/linux_386/integration-cli $(BGO_SPACE)/bin/prepacked/linux_386/integration-cli
	cp $(BGO_SPACE)/bin/linux_386/updater $(BGO_SPACE)/bin/prepacked/linux_386/updater
	cp $(BGO_SPACE)/bin/amazon-ssm-agent.json.template $(BGO_SPACE)/bin/prepacked/linux_386/amazon-ssm-agent.json.template
	cp $(BGO_SPACE)/bin/integration-cli.json $(BGO_SPACE)/bin/prepacked/linux_386/integration-cli.json
	cp $(BGO_SPACE)/bin/seelog_unix.xml $(BGO_SPACE)/bin/prepacked/linux_386/seelog.xml
	cp $(BGO_SPACE)/bin/LICENSE $(BGO_SPACE)/bin/prepacked/linux_386/LICENSE

.PHONY: prepack-darwin-386
prepack-darwin-386:
	mkdir -p $(BGO_SPACE)/bin/prepacked/darwin_386
	cp $(BGO_SPACE)/bin/darwin_386/amazon-ssm-agent $(BGO_SPACE)/bin/prepacked/darwin_386/amazon-ssm-agent
	cp $(BGO_SPACE)/bin/darwin_386/integration-cli $(BGO_SPACE)/bin/prepacked/darwin_386/integration-cli
	cp $(BGO_SPACE)/bin/darwin_386/updater $(BGO_SPACE)/bin/prepacked/darwin_386/updater
	cp $(BGO_SPACE)/bin/amazon-ssm-agent.json.template $(BGO_SPACE)/bin/prepacked/darwin_386/amazon-ssm-agent.json.template
	cp $(BGO_SPACE)/bin/integration-cli.json $(BGO_SPACE)/bin/prepacked/darwin_386/integration-cli.json
	cp $(BGO_SPACE)/bin/seelog_unix.xml $(BGO_SPACE)/bin/prepacked/darwin_386/seelog.xml
	cp $(BGO_SPACE)/bin/LICENSE $(BGO_SPACE)/bin/prepacked/darwin_386/LICENSE

.PHONY: prepack-windows-386
prepack-windows-386:
	mkdir -p $(BGO_SPACE)/bin/prepacked/windows_386
	cp $(BGO_SPACE)/bin/windows_386/amazon-ssm-agent.exe $(BGO_SPACE)/bin/prepacked/windows_386/amazon-ssm-agent.exe
	cp $(BGO_SPACE)/bin/windows_386/integration-cli.exe $(BGO_SPACE)/bin/prepacked/windows_386/integration-cli.exe
	cp $(BGO_SPACE)/bin/windows_386/updater.exe $(BGO_SPACE)/bin/prepacked/windows_386/updater.exe
	cp $(BGO_SPACE)/bin/amazon-ssm-agent.json.template $(BGO_SPACE)/bin/prepacked/windows_386/amazon-ssm-agent.json.template
	cp $(BGO_SPACE)/bin/integration-cli.json $(BGO_SPACE)/bin/prepacked/windows_386/integration-cli.json
	cp $(BGO_SPACE)/bin/seelog_windows.xml.template $(BGO_SPACE)/bin/prepacked/windows_386/seelog.xml.template
	cp $(BGO_SPACE)/bin/LICENSE $(BGO_SPACE)/bin/prepacked/windows_386/LICENSE

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
package-rpm: create-package-folder
	$(BGO_SPACE)/Tools/src/create_rpm.sh

.PHONY: package-deb
package-deb: create-package-folder
	$(BGO_SPACE)/Tools/src/create_deb.sh

.PHONY: package-win
package-win: create-package-folder
	$(BGO_SPACE)/Tools/src/create_win.sh

.PHONY: package-rpm-386
package-rpm-386: create-package-folder
	$(BGO_SPACE)/Tools/src/create_rpm_386.sh

.PHONY: package-deb-386
package-deb-386: create-package-folder
	$(BGO_SPACE)/Tools/src/create_deb_386.sh

.PHONY: package-win-386
package-win-386: create-package-folder
	$(BGO_SPACE)/Tools/src/create_win_386.sh

.PHONY: get-tools
get-tools:
	go get -u github.com/nsf/gocode
	go get -u golang.org/x/tools/cmd/oracle
	go get -u golang.org/x/tools/go/loader
	go get -u golang.org/x/tools/go/types

.PHONY: quick-integtest
quick-integtest:
	# if you want to restrict to some specific package, sample below
	# go test -v -gcflags "-N -l" -tags=integration github.com/aws/amazon-ssm-agent/agent/fileutil/...
	go test -gcflags "-N -l" -tags=integration github.com/aws/amazon-ssm-agent/agent/...

.PHONY: gen-report
gen-report:
	$(BGO_SPACE)/Tools/src/gen-report.sh
