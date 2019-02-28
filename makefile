BUILDFILE_PATH := ./build/private/bgo_exports.makefile
COPY := cp -p
GO_BUILD := go build -i
BRAZIL_BUILD := false

# Using the wildcard function to check if file exists
ifneq ("$(wildcard $(BUILDFILE_PATH))","")
	include $(BUILDFILE_PATH)
	BRAZIL_BUILD := true
endif

ifeq ($(BRAZIL_BUILD), true)
	GOTEMPPATH := $(BGO_SPACE)/build/private
	GOTEMPCOPYPATH := $(GOTEMPPATH)/src/github.com/aws/amazon-ssm-agent
	GOPATH := $(GOTEMPPATH):$(BGO_SPACE)/vendor:$(GOPATH)
	TEMPVERSIONPATH := $(GOTEMPCOPYPATH)/agent/version
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

ifneq ($(dir),)
	MOCKERYDIR := $(dir)
else
	MOCKERYDIR := NotSet
endif

export GOPATH
export BRAZIL_BUILD

checkstyle::
#   Run checkstyle script
	$(BGO_SPACE)/Tools/src/checkstyle.sh

coverage:: build-linux
	$(BGO_SPACE)/Tools/src/coverage.sh github.com/aws/amazon-ssm-agent/agent/...

build:: build-linux build-freebsd build-windows build-linux-386 build-windows-386 build-arm build-arm64 build-darwin

prepack:: cpy-plugins prepack-linux prepack-linux-arm64 prepack-linux-386 prepack-windows prepack-windows-386

package:: create-package-folder package-linux package-windows package-darwin

release:: clean quick-integtest checkstyle pre-release build prepack package build-tests

ifneq ($(FINALIZE),)
	bgo-final
endif

.PHONY: build-tests
build-tests: build-tests-linux build-tests-windows

.PHONY: dev-build-linux
dev-build-linux: clean quick-integtest checkstyle pre-release build-linux build-tests-linux
.PHONY: dev-build-freebsd
dev-build-freebsd: clean quick-integtest checkstyle pre-release build-freebsd
.PHONY: dev-build-windows
dev-build-windows: clean quick-integtest checkstyle pre-release build-windows build-tests-windows
.PHONY: dev-build-linux-386
dev-build-linux-386: clean quick-integtest checkstyle pre-release build-linux-386
.PHONY: dev-build-windows-386
dev-build-windows-386: clean quick-integtest checkstyle pre-release build-windows-386
.PHONY: dev-build-arm
dev-build-arm: clean quick-integtest checkstyle pre-release build-arm
.PHONY: dev-build-arm64
dev-build-arm64: clean quick-integtest checkstyle pre-release build-arm64
	
sources:: create-source-archive

clean:: remove-prepacked-folder
	rm -rf build/* bin/ pkg/ vendor/bin/ vendor/pkg/ .cover/
	find . -type f -name '*.log' -delete

.PHONY: mockgen
mockgen: clean checkstyle copy-src build-mock

.PHONY: update-plugins-binaries
update-plugins-binaries:
	$(BGO_SPACE)/Tools/src/release_dependencies.sh

.PHONY: cpy-plugins
cpy-plugins:
	$(BGO_SPACE)/Tools/src/copy_plugin_binaries.sh $(BRAZIL_BUILD)

.PHONY: quick-integtest
quick-integtest: copy-src pre-build pre-release --quick-integtest

.PHONY: quick-test
quick-test: copy-src pre-build pre-release --quick-test

.PHONY: pre-release
pre-release:
	@echo "SSM Agent release build"
	$(eval GO_BUILD := go build)
	rm -rf $(BGO_SPACE)/vendor/pkg

.PHONY: pre-build
pre-build:
	for file in $(BGO_SPACE)/Tools/src/*.sh; do chmod 755 $$file; done
	@echo "Build amazon-ssm-agent"
	@echo "GOPATH=$(GOPATH)"
	rm -rf $(BGO_SPACE)/build/bin/ $(BGO_SPACE)/vendor/bin/
	mkdir -p $(BGO_SPACE)/bin/
	$(COPY) $(BGO_SPACE)/Tools/src/PipelineRunTests.sh $(BGO_SPACE)/bin/
	$(COPY) $(BGO_SPACE)/LICENSE $(BGO_SPACE)/bin/
	$(COPY) $(BGO_SPACE)/amazon-ssm-agent.json.template $(BGO_SPACE)/bin/amazon-ssm-agent.json.template
	$(COPY) $(BGO_SPACE)/seelog_unix.xml $(BGO_SPACE)/bin/
	$(COPY) $(BGO_SPACE)/seelog_windows.xml.template $(BGO_SPACE)/bin/
	$(COPY) $(BGO_SPACE)/agent/integration-cli/integration-cli.json $(BGO_SPACE)/bin/

	@echo "Regenerate version file during pre-release"
	go run $(BGO_SPACE)/agent/version/versiongenerator/version-gen.go
	$(COPY) $(BGO_SPACE)/VERSION $(BGO_SPACE)/bin/

ifeq ($(BRAZIL_BUILD), true)
	@echo "Copying version files generated in pre-build"
	mkdir -p $(TEMPVERSIONPATH)
	$(COPY) $(BGO_SPACE)/VERSION $(GOTEMPCOPYPATH)
	$(COPY) $(BGO_SPACE)/agent/version/version.go $(TEMPVERSIONPATH)

	@echo "Update riputil file during pre-release"
	$(COPY) $(BGO_SPACE)/../../env/RIPStaticConfig-1.4/runtime/configuration/rip/rip_static_config.json $(BGO_SPACE)/agent/s3util
	go run $(BGO_SPACE)/agent/s3util/generator/riputil-gen.go
	$(COPY) $(BGO_SPACE)/agent/s3util/riputil.go $(GOTEMPCOPYPATH)/agent/s3util/riputil.go

	$(COPY) $(BGO_SPACE)/../../env/RIPStaticConfig-1.4/runtime/configuration/rip/rip_static_config.json $(BGO_SPACE)/agent/rip
	go run $(BGO_SPACE)/agent/rip/generator/rip-gen.go
endif

.PHONY: build-linux
build-linux: checkstyle copy-src pre-build
	@echo "Build for linux agent"
	GOOS=linux GOARCH=amd64 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_amd64/amazon-ssm-agent -v \
	$(BGO_SPACE)/agent/agent.go $(BGO_SPACE)/agent/agent_unix.go $(BGO_SPACE)/agent/agent_parser.go
	GOOS=linux GOARCH=amd64 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_amd64/updater -v \
	$(BGO_SPACE)/agent/update/updater/updater.go $(BGO_SPACE)/agent/update/updater/updater_unix.go
	GOOS=linux GOARCH=amd64 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_amd64/ssm-cli -v \
		$(BGO_SPACE)/agent/cli-main/cli-main.go
	GOOS=linux GOARCH=amd64 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_amd64/ssm-document-worker -v \
							$(BGO_SPACE)/agent/framework/processor/executer/outofproc/worker/main.go
	GOOS=linux GOARCH=amd64 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_amd64/ssm-session-logger -v \
                            $(BGO_SPACE)/agent/session/logging/main.go
	GOOS=linux GOARCH=amd64 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_amd64/ssm-session-worker -v \
    						$(BGO_SPACE)/agent/framework/processor/executer/outofproc/sessionworker/main.go

.PHONY: build-freebsd
build-freebsd: checkstyle copy-src pre-build
	@echo "Build for freebsd agent"
	GOOS=freebsd GOARCH=amd64 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/freebsd_amd64/amazon-ssm-agent -v \
	$(BGO_SPACE)/agent/agent.go $(BGO_SPACE)/agent/agent_unix.go $(BGO_SPACE)/agent/agent_parser.go
	GOOS=freebsd GOARCH=amd64 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/freebsd_amd64/ssm-cli -v \
			$(BGO_SPACE)/agent/cli-main/cli-main.go
	GOOS=freebsd GOARCH=amd64 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/freebsd_amd64/ssm-document-worker -v \
								$(BGO_SPACE)/agent/framework/processor/executer/outofproc/worker/main.go
	GOOS=freebsd GOARCH=amd64 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/freebsd_amd64/ssm-session-logger -v \
                                $(BGO_SPACE)/agent/session/logging/main.go
	GOOS=freebsd GOARCH=amd64 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/freebsd_amd64/ssm-session-worker -v \
    						    $(BGO_SPACE)/agent/framework/processor/executer/outofproc/sessionworker/main.go

.PHONY: build-darwin
build-darwin: checkstyle copy-src pre-build
	@echo "Build for darwin agent"
	GOOS=darwin GOARCH=amd64 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/darwin_amd64/amazon-ssm-agent -v \
	$(BGO_SPACE)/agent/agent.go $(BGO_SPACE)/agent/agent_unix.go $(BGO_SPACE)/agent/agent_parser.go
	GOOS=darwin GOARCH=amd64 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/darwin_amd64/updater -v \
	$(BGO_SPACE)/agent/update/updater/updater.go $(BGO_SPACE)/agent/update/updater/updater_unix.go
	GOOS=darwin GOARCH=amd64 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/darwin_amd64/ssm-cli -v \
		$(BGO_SPACE)/agent/cli-main/cli-main.go
	GOOS=darwin GOARCH=amd64 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/darwin_amd64/ssm-document-worker -v \
							$(BGO_SPACE)/agent/framework/processor/executer/outofproc/worker/main.go

.PHONY: build-windows
build-windows: checkstyle copy-src pre-build
	@echo "Rebuild for windows agent"
	GOOS=windows GOARCH=amd64 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/windows_amd64/amazon-ssm-agent.exe -v \
	$(BGO_SPACE)/agent/agent.go $(BGO_SPACE)/agent/agent_windows.go $(BGO_SPACE)/agent/agent_parser.go
	GOOS=windows GOARCH=amd64 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/windows_amd64/updater.exe -v \
	$(BGO_SPACE)/agent/update/updater/updater.go $(BGO_SPACE)/agent/update/updater/updater_windows.go
	GOOS=windows GOARCH=amd64 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/windows_amd64/ssm-cli.exe -v \
		$(BGO_SPACE)/agent/cli-main/cli-main.go
	GOOS=windows GOARCH=amd64 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/windows_amd64/ssm-document-worker.exe -v \
								$(BGO_SPACE)/agent/framework/processor/executer/outofproc/worker/main.go
	GOOS=windows GOARCH=amd64 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/windows_amd64/ssm-session-logger.exe -v \
        						$(BGO_SPACE)/agent/session/logging/main.go
	GOOS=windows GOARCH=amd64 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/windows_amd64/ssm-session-worker.exe -v \
								$(BGO_SPACE)/agent/framework/processor/executer/outofproc/sessionworker/main.go

.PHONY: build-linux-386
build-linux-386: checkstyle copy-src pre-build
	@echo "Build for linux agent"
	GOOS=linux GOARCH=386 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_386/amazon-ssm-agent -v \
	$(BGO_SPACE)/agent/agent.go $(BGO_SPACE)/agent/agent_unix.go $(BGO_SPACE)/agent/agent_parser.go
	GOOS=linux GOARCH=386 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_386/updater -v \
	$(BGO_SPACE)/agent/update/updater/updater.go $(BGO_SPACE)/agent/update/updater/updater_unix.go
	GOOS=linux GOARCH=386 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_386/ssm-cli -v \
		$(BGO_SPACE)/agent/cli-main/cli-main.go
	GOOS=linux GOARCH=386 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_386/ssm-document-worker -v \
								$(BGO_SPACE)/agent/framework/processor/executer/outofproc/worker/main.go
	GOOS=linux GOARCH=386 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_386/ssm-session-logger -v \
        						$(BGO_SPACE)/agent/session/logging/main.go
	GOOS=linux GOARCH=386 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_386/ssm-session-worker -v \
								$(BGO_SPACE)/agent/framework/processor/executer/outofproc/sessionworker/main.go

.PHONY: build-darwin-386
build-darwin-386: checkstyle copy-src pre-build
	@echo "Build for darwin agent"
	GOOS=darwin GOARCH=386 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/darwin_386/amazon-ssm-agent -v \
	$(BGO_SPACE)/agent/agent.go $(BGO_SPACE)/agent/agent_unix.go $(BGO_SPACE)/agent/agent_parser.go
	GOOS=darwin GOARCH=386 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/darwin_386/updater -v \
	$(BGO_SPACE)/agent/update/updater/updater.go $(BGO_SPACE)/agent/update/updater/updater_unix.go
	GOOS=darwin GOARCH=386 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/darwin_386/ssm-cli -v \
		$(BGO_SPACE)/agent/cli-main/cli-main.go
	GOOS=darwin GOARCH=386 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/darwin_386/ssm-document-worker -v \
								$(BGO_SPACE)/agent/framework/processor/executer/outofproc/worker/main.go

.PHONY: build-windows-386
build-windows-386: checkstyle copy-src pre-build
	@echo "Rebuild for windows agent"
	GOOS=windows GOARCH=386 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/windows_386/amazon-ssm-agent.exe -v \
	$(BGO_SPACE)/agent/agent.go $(BGO_SPACE)/agent/agent_windows.go $(BGO_SPACE)/agent/agent_parser.go
	GOOS=windows GOARCH=386 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/windows_386/updater.exe -v \
	$(BGO_SPACE)/agent/update/updater/updater.go $(BGO_SPACE)/agent/update/updater/updater_windows.go
	GOOS=windows GOARCH=386 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/windows_386/ssm-cli.exe -v \
		$(BGO_SPACE)/agent/cli-main/cli-main.go
	GOOS=windows GOARCH=386 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/windows_386/ssm-document-worker.exe -v \
								$(BGO_SPACE)/agent/framework/processor/executer/outofproc/worker/main.go
	GOOS=windows GOARCH=386 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/windows_386/ssm-session-logger.exe -v \
        						$(BGO_SPACE)/agent/session/logging/main.go
	GOOS=windows GOARCH=386 go build -ldflags "-s -w" -o $(BGO_SPACE)/bin/windows_386/ssm-session-worker.exe -v \
								$(BGO_SPACE)/agent/framework/processor/executer/outofproc/sessionworker/main.go

.PHONY: build-arm
build-arm: checkstyle copy-src pre-build
	@echo "Build for ARM platforms"
	GOOS=linux GOARCH=arm GOARM=6 $(GO_BUILD)  -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_arm/amazon-ssm-agent -v \
		$(BGO_SPACE)/agent/agent.go $(BGO_SPACE)/agent/agent_unix.go $(BGO_SPACE)/agent/agent_parser.go
	GOOS=linux GOARCH=arm GOARM=6 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_arm/updater -v \
		$(BGO_SPACE)/agent/update/updater/updater.go $(BGO_SPACE)/agent/update/updater/updater_unix.go
	GOOS=linux GOARCH=arm GOARM=6 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_arm/ssm-cli -v \
		$(BGO_SPACE)/agent/cli-main/cli-main.go
	GOOS=linux GOARCH=arm GOARM=6 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_arm/ssm-document-worker -v \
								$(BGO_SPACE)/agent/framework/processor/executer/outofproc/worker/main.go
	GOOS=linux GOARCH=arm GOARM=6 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_arm/ssm-session-logger -v \
        						$(BGO_SPACE)/agent/session/logging/main.go
	GOOS=linux GOARCH=arm GOARM=6 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_arm/ssm-session-worker -v \
								$(BGO_SPACE)/agent/framework/processor/executer/outofproc/sessionworker/main.go

.PHONY: build-arm64
build-arm64: checkstyle copy-src pre-build
	@echo "Build for ARM64 platforms"
	GOOS=linux GOARCH=arm64 $(GO_BUILD)  -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_arm64/amazon-ssm-agent -v \
		$(BGO_SPACE)/agent/agent.go $(BGO_SPACE)/agent/agent_unix.go $(BGO_SPACE)/agent/agent_parser.go
	GOOS=linux GOARCH=arm64 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_arm64/updater -v \
		$(BGO_SPACE)/agent/update/updater/updater.go $(BGO_SPACE)/agent/update/updater/updater_unix.go
	GOOS=linux GOARCH=arm64 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_arm64/ssm-cli -v \
		$(BGO_SPACE)/agent/cli-main/cli-main.go
	GOOS=linux GOARCH=arm64 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_arm64/ssm-document-worker -v \
								$(BGO_SPACE)/agent/framework/processor/executer/outofproc/worker/main.go
	GOOS=linux GOARCH=arm64 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_arm64/ssm-session-logger -v \
        						$(BGO_SPACE)/agent/session/logging/main.go
	GOOS=linux GOARCH=arm64 $(GO_BUILD) -ldflags "-s -w" -o $(BGO_SPACE)/bin/linux_arm64/ssm-session-worker -v \
								$(BGO_SPACE)/agent/framework/processor/executer/outofproc/sessionworker/main.go

.PHONY: copy-src
copy-src:
ifeq ($(BRAZIL_BUILD), true)
	rm -rf $(GOTEMPCOPYPATH)
	mkdir -p $(GOTEMPCOPYPATH)
	@echo "copying files to $(GOTEMPCOPYPATH)"
	$(COPY) -r $(BGO_SPACE)/agent $(GOTEMPCOPYPATH)
endif

.PHONY: build-tests-linux
build-tests-linux: copy-src copy-tests-src pre-build
	GOOS=linux GOARCH=amd64 go test -c -gcflags "-N -l" -tags=tests \
		github.com/aws/amazon-ssm-agent/internal/tests \
		-o bin/agent-tests/linux_amd64/agent-tests.test
	GOOS=linux GOARCH=arm64 go test -c -gcflags "-N -l" -tags=tests \
		github.com/aws/amazon-ssm-agent/internal/tests \
		-o bin/agent-tests/linux_arm64/agent-tests.test

.PHONY: build-tests-windows
build-tests-windows: copy-src copy-tests-src pre-build
	GOOS=windows GOARCH=amd64 go test -c -gcflags "-N -l" -tags=tests \
		github.com/aws/amazon-ssm-agent/internal/tests \
		-o bin/agent-tests/windows_amd64/agent-tests.test

.PHONY: copy-tests-src
copy-tests-src:
ifeq ($(BRAZIL_BUILD), true)
	@echo "copying test files to $(GOTEMPCOPYPATH)"
	$(COPY) -r $(BGO_SPACE)/internal $(GOTEMPCOPYPATH)
endif

.PHONY: remove-prepacked-folder
remove-prepacked-folder:
	rm -rf $(BGO_SPACE)/bin/prepacked

.PHONY: prepack-linux
prepack-linux:
	mkdir -p $(BGO_SPACE)/bin/prepacked/linux_amd64
	$(COPY) $(BGO_SPACE)/bin/linux_amd64/amazon-ssm-agent $(BGO_SPACE)/bin/prepacked/linux_amd64/amazon-ssm-agent
	$(COPY) $(BGO_SPACE)/bin/linux_amd64/updater $(BGO_SPACE)/bin/prepacked/linux_amd64/updater
	$(COPY) $(BGO_SPACE)/bin/linux_amd64/ssm-cli $(BGO_SPACE)/bin/prepacked/linux_amd64/ssm-cli
	$(COPY) $(BGO_SPACE)/bin/linux_amd64/ssm-document-worker $(BGO_SPACE)/bin/prepacked/linux_amd64/ssm-document-worker
	$(COPY) $(BGO_SPACE)/bin/linux_amd64/ssm-session-worker $(BGO_SPACE)/bin/prepacked/linux_amd64/ssm-session-worker
	$(COPY) $(BGO_SPACE)/bin/linux_amd64/ssm-session-logger $(BGO_SPACE)/bin/prepacked/linux_amd64/ssm-session-logger
	$(COPY) $(BGO_SPACE)/bin/amazon-ssm-agent.json.template $(BGO_SPACE)/bin/prepacked/linux_amd64/amazon-ssm-agent.json.template
	$(COPY) $(BGO_SPACE)/bin/seelog_unix.xml $(BGO_SPACE)/bin/prepacked/linux_amd64/seelog.xml.template
	$(COPY) $(BGO_SPACE)/bin/LICENSE $(BGO_SPACE)/bin/prepacked/linux_amd64/LICENSE

.PHONY: prepack-linux-arm64
prepack-linux-arm64:
	mkdir -p $(BGO_SPACE)/bin/prepacked/linux_arm64
	$(COPY) $(BGO_SPACE)/bin/linux_arm64/amazon-ssm-agent $(BGO_SPACE)/bin/prepacked/linux_arm64/amazon-ssm-agent
	$(COPY) $(BGO_SPACE)/bin/linux_arm64/updater $(BGO_SPACE)/bin/prepacked/linux_arm64/updater
	$(COPY) $(BGO_SPACE)/bin/linux_arm64/ssm-cli $(BGO_SPACE)/bin/prepacked/linux_arm64/ssm-cli
	$(COPY) $(BGO_SPACE)/bin/linux_arm64/ssm-document-worker $(BGO_SPACE)/bin/prepacked/linux_arm64/ssm-document-worker
	$(COPY) $(BGO_SPACE)/bin/linux_arm64/ssm-session-worker $(BGO_SPACE)/bin/prepacked/linux_arm64/ssm-session-worker
	$(COPY) $(BGO_SPACE)/bin/linux_arm64/ssm-session-logger $(BGO_SPACE)/bin/prepacked/linux_arm64/ssm-session-logger
	$(COPY) $(BGO_SPACE)/bin/amazon-ssm-agent.json.template $(BGO_SPACE)/bin/prepacked/linux_arm64/amazon-ssm-agent.json.template
	$(COPY) $(BGO_SPACE)/bin/seelog_unix.xml $(BGO_SPACE)/bin/prepacked/linux_arm64/seelog.xml.template
	$(COPY) $(BGO_SPACE)/bin/LICENSE $(BGO_SPACE)/bin/prepacked/linux_arm64/LICENSE

.PHONY: prepack-windows
prepack-windows:
	mkdir -p $(BGO_SPACE)/bin/prepacked/windows_amd64
	$(COPY) $(BGO_SPACE)/bin/windows_amd64/amazon-ssm-agent.exe $(BGO_SPACE)/bin/prepacked/windows_amd64/amazon-ssm-agent.exe
	$(COPY) $(BGO_SPACE)/bin/windows_amd64/updater.exe $(BGO_SPACE)/bin/prepacked/windows_amd64/updater.exe
	$(COPY) $(BGO_SPACE)/bin/windows_amd64/ssm-cli.exe $(BGO_SPACE)/bin/prepacked/windows_amd64/ssm-cli.exe
	$(COPY) $(BGO_SPACE)/bin/windows_amd64/ssm-document-worker.exe $(BGO_SPACE)/bin/prepacked/windows_amd64/ssm-document-worker.exe
	$(COPY) $(BGO_SPACE)/bin/windows_amd64/ssm-session-worker.exe $(BGO_SPACE)/bin/prepacked/windows_amd64/ssm-session-worker.exe
	$(COPY) $(BGO_SPACE)/bin/windows_amd64/ssm-session-logger.exe $(BGO_SPACE)/bin/prepacked/windows_amd64/ssm-session-logger.exe
	$(COPY) $(BGO_SPACE)/bin/amazon-ssm-agent.json.template $(BGO_SPACE)/bin/prepacked/windows_amd64/amazon-ssm-agent.json.template
	$(COPY) $(BGO_SPACE)/bin/seelog_windows.xml.template $(BGO_SPACE)/bin/prepacked/windows_amd64/seelog.xml.template
	$(COPY) $(BGO_SPACE)/bin/LICENSE $(BGO_SPACE)/bin/prepacked/windows_amd64/LICENSE

.PHONY: prepack-linux-386
prepack-linux-386:
	mkdir -p $(BGO_SPACE)/bin/prepacked/linux_386
	$(COPY) $(BGO_SPACE)/bin/linux_386/amazon-ssm-agent $(BGO_SPACE)/bin/prepacked/linux_386/amazon-ssm-agent
	$(COPY) $(BGO_SPACE)/bin/linux_386/updater $(BGO_SPACE)/bin/prepacked/linux_386/updater
	$(COPY) $(BGO_SPACE)/bin/linux_386/ssm-cli $(BGO_SPACE)/bin/prepacked/linux_386/ssm-cli
	$(COPY) $(BGO_SPACE)/bin/linux_386/ssm-document-worker $(BGO_SPACE)/bin/prepacked/linux_386/ssm-document-worker
	$(COPY) $(BGO_SPACE)/bin/linux_386/ssm-session-worker $(BGO_SPACE)/bin/prepacked/linux_386/ssm-session-worker
	$(COPY) $(BGO_SPACE)/bin/linux_386/ssm-session-logger $(BGO_SPACE)/bin/prepacked/linux_386/ssm-session-logger
	$(COPY) $(BGO_SPACE)/bin/amazon-ssm-agent.json.template $(BGO_SPACE)/bin/prepacked/linux_386/amazon-ssm-agent.json.template
	$(COPY) $(BGO_SPACE)/bin/seelog_unix.xml $(BGO_SPACE)/bin/prepacked/linux_386/seelog.xml.template
	$(COPY) $(BGO_SPACE)/bin/LICENSE $(BGO_SPACE)/bin/prepacked/linux_386/LICENSE

.PHONY: prepack-windows-386
prepack-windows-386:
	mkdir -p $(BGO_SPACE)/bin/prepacked/windows_386
	$(COPY) $(BGO_SPACE)/bin/windows_386/amazon-ssm-agent.exe $(BGO_SPACE)/bin/prepacked/windows_386/amazon-ssm-agent.exe
	$(COPY) $(BGO_SPACE)/bin/windows_386/updater.exe $(BGO_SPACE)/bin/prepacked/windows_386/updater.exe
	$(COPY) $(BGO_SPACE)/bin/windows_386/ssm-cli.exe $(BGO_SPACE)/bin/prepacked/windows_386/ssm-cli.exe
	$(COPY) $(BGO_SPACE)/bin/windows_386/ssm-document-worker.exe $(BGO_SPACE)/bin/prepacked/windows_386/ssm-document-worker.exe
	$(COPY) $(BGO_SPACE)/bin/windows_386/ssm-session-worker.exe $(BGO_SPACE)/bin/prepacked/windows_386/ssm-session-worker.exe
	$(COPY) $(BGO_SPACE)/bin/windows_386/ssm-session-logger.exe $(BGO_SPACE)/bin/prepacked/windows_386/ssm-session-logger.exe
	$(COPY) $(BGO_SPACE)/bin/amazon-ssm-agent.json.template $(BGO_SPACE)/bin/prepacked/windows_386/amazon-ssm-agent.json.template
	$(COPY) $(BGO_SPACE)/bin/seelog_windows.xml.template $(BGO_SPACE)/bin/prepacked/windows_386/seelog.xml.template
	$(COPY) $(BGO_SPACE)/bin/LICENSE $(BGO_SPACE)/bin/prepacked/windows_386/LICENSE

.PHONY: create-package-folder
create-package-folder:
	mkdir -p $(BGO_SPACE)/bin/updates/amazon-ssm-agent/`cat $(BGO_SPACE)/VERSION`/
	mkdir -p $(BGO_SPACE)/bin/updates/amazon-ssm-agent-updater/`cat $(BGO_SPACE)/VERSION`/

.PHONY: package-linux
package-linux: package-rpm-386 package-deb-386 package-rpm package-deb package-deb-arm package-deb-arm64 package-rpm-arm64
	$(BGO_SPACE)/Tools/src/create_linux_package.sh

.PHONY: package-windows
package-windows: package-win-386 package-win
	$(BGO_SPACE)/Tools/src/create_windows_package.sh
	$(BGO_SPACE)/Tools/src/create_windows_nano_package.sh

.PHONY: create-source-archive
create-source-archive:
	$(eval SOURCE_PACKAGE_NAME := amazon-ssm-agent-`cat $(BGO_SPACE)/VERSION`)
	git archive --prefix=$(SOURCE_PACKAGE_NAME)/ --format=tar HEAD | gzip -c > $(SOURCE_PACKAGE_NAME).tar.gz

.PHONY: package-rpm
package-rpm: create-package-folder
	$(BGO_SPACE)/Tools/src/create_rpm.sh

.PHONY: package-deb
package-deb: create-package-folder
	$(BGO_SPACE)/Tools/src/create_deb.sh

.PHONY: package-win
package-win: create-package-folder
	$(BGO_SPACE)/Tools/src/create_win.sh

.PHONY: package-darwin
package-darwin:
	$(BGO_SPACE)/Tools/src/create_darwin.sh

.PHONY: package-rpm-386
package-rpm-386: create-package-folder
	$(BGO_SPACE)/Tools/src/create_rpm_386.sh

.PHONY: package-deb-386
package-deb-386: create-package-folder
	$(BGO_SPACE)/Tools/src/create_deb_386.sh

.PHONY: package-win-386
package-win-386: create-package-folder
	$(BGO_SPACE)/Tools/src/create_win_386.sh

.PHONY: package-deb-arm
package-deb-arm: create-package-folder
	$(BGO_SPACE)/Tools/src/create_deb_arm.sh

.PHONY: package-deb-arm64
package-deb-arm64: create-package-folder
	$(BGO_SPACE)/Tools/src/create_deb_arm64.sh

.PHONY: package-rpm-arm64
package-rpm-arm64: create-package-folder
	$(BGO_SPACE)/Tools/src/create_rpm_arm64.sh

.PHONY: get-tools
get-tools:
	go get -u github.com/nsf/gocode
	go get -u golang.org/x/tools/cmd/oracle
	go get -u golang.org/x/tools/go/loader
	go get -u golang.org/x/tools/go/types

.PHONY: --quick-integtest
--quick-integtest:
	# if you want to restrict to some specific package, sample below
	# go test -v -gcflags "-N -l" -tags=integration github.com/aws/amazon-ssm-agent/agent/fileutil/...
	go test -gcflags "-N -l" -tags=integration github.com/aws/amazon-ssm-agent/agent/...

.PHONY: --quick-test
--quick-test:
	# if you want to test a specific package, you can add the package name instead of the dots. Sample below
	# go test -gcflags "-N -l" github.com/aws/amazon-ssm-agent/agent/task
	go test -gcflags "-N -l" github.com/aws/amazon-ssm-agent/agent/...


.PHONY: gen-report
gen-report:
	$(BGO_SPACE)/Tools/src/gen-report.sh


.PHONY: build-mock
build-mock:
	@echo "SSM Agent Mock Generation"
ifeq ($(MOCKERYDIR), NotSet)
	@echo "Please enter the directory name. e.g 'bb mockgen dir=agent/framework' or 'brazil-build mockgen dir=agent/health' "
	exit 1
else
	@echo "Start generating mocks in directory" $(MOCKERYDIR)
endif
	mockery -name="[A-Z]*" -dir=$(MOCKERYDIR) -output=mocks
	mv mocks $(MOCKERYDIR)/mocks