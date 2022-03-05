COPY := cp -p
GO_BUILD := CGO_ENABLED=0 go build -ldflags "-s -w" -trimpath
GO_BUILD_PIE := go build -ldflags "-s -w -extldflags=-Wl,-z,now,-z,relro,-z,defs" -buildmode=pie -trimpath



# Initailize workspace if it's empty
ifeq ($(WORKSPACE),)
	WORKSPACE := $(shell pwd)/../../../../
endif

# Initailize GO_SPACE
export GO_SPACE=$(shell pwd)
GOTEMPPATH := $(GO_SPACE)/build/private
GOTEMPCOPYPATH := $(GOTEMPPATH)/src/github.com/aws/amazon-ssm-agent

path := $(GOTEMPPATH):$(GO_SPACE)/vendor
ifneq ($(GOPATH),)
	GOPATH := $(path):$(GOPATH)
else
	GOPATH := $(path)
endif


export GOPATH

checkstyle::
#   Run checkstyle script
	$(GO_SPACE)/Tools/src/checkstyle.sh

coverage:: build-linux
	$(GO_SPACE)/Tools/src/coverage.sh \
	  github.com/aws/amazon-ssm-agent/agent/... \
	  github.com/aws/amazon-ssm-agent/core/...

build:: build-linux build-freebsd build-windows build-linux-386 build-windows-386 build-arm build-arm64 build-darwin

prepack:: cpy-plugins copy-win-dep prepack-linux prepack-linux-arm64 prepack-linux-386 prepack-windows prepack-windows-386

package:: create-package-folder package-linux package-windows package-darwin

build-release:: clean quick-integtest checkstyle pre-release build prepack package finalize

release:: clean quick-integtest checkstyle pre-release cpy-plugins copy-win-dep finalize

package-src:: clean quick-integtest checkstyle pre-release cpy-plugins finalize

finalize:: copy-package-dep

.PHONY: dev-build-linux
dev-build-linux: clean quick-integtest checkstyle pre-release build-linux
.PHONY: dev-build-freebsd
dev-build-freebsd: clean quick-integtest checkstyle pre-release build-freebsd
.PHONY: dev-build-windows
dev-build-windows: clean quick-integtest checkstyle pre-release build-windows
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


.PHONY: cpy-plugins
cpy-plugins: copy-src pre-build
	$(GO_SPACE)/Tools/src/copy_plugin_binaries.sh

.PHONY: quick-integtest
quick-integtest: copy-src pre-build pre-release --quick-integtest --quick-integtest-core

.PHONY: quick-test
quick-test: copy-src pre-build pre-release --quick-test

.PHONY: quick-test-core
quick-test-core: copy-src pre-build pre-release --quick-test-core

.PHONY: quick-e2e
quick-e2e: copy-src pre-build pre-release --quick-e2e --quick-e2e-core

.PHONY: pre-release
pre-release:
	@echo "SSM Agent release build"
	rm -rf $(GO_SPACE)/vendor/pkg

.PHONY: copy-win-dep
copy-win-dep:
	@echo "Copying Windows packaging dependencies"
	$(COPY) -r $(GO_SPACE)/packaging/dependencies/* $(GO_SPACE)/bin

.PHONY: pre-build
pre-build:
	for file in $(GO_SPACE)/Tools/src/*.sh; do chmod 755 $$file; done
	@echo "Build amazon-ssm-agent"
	@echo "GOPATH=$(GOPATH)"
	rm -rf $(GO_SPACE)/build/bin/ $(GO_SPACE)/vendor/bin/
	mkdir -p $(GO_SPACE)/bin/
	$(COPY) $(GO_SPACE)/Tools/src/PipelineRunTests.sh $(GO_SPACE)/bin/
	$(COPY) $(GO_SPACE)/LICENSE $(GO_SPACE)/bin/
	$(COPY) $(GO_SPACE)/NOTICE.md $(GO_SPACE)/bin/
	$(COPY) $(GO_SPACE)/amazon-ssm-agent.json.template $(GO_SPACE)/bin/amazon-ssm-agent.json.template
	$(COPY) $(GO_SPACE)/seelog_unix.xml $(GO_SPACE)/bin/
	$(COPY) $(GO_SPACE)/seelog_windows.xml.template $(GO_SPACE)/bin/
	$(COPY) $(GO_SPACE)/agent/integration-cli/integration-cli.json $(GO_SPACE)/bin/

	@echo "Regenerate version file during pre-release"
	go run $(GO_SPACE)/agent/version/versiongenerator/version-gen.go
	$(COPY) $(GO_SPACE)/VERSION $(GO_SPACE)/bin/


.PHONY: build-linux
build-linux: checkstyle copy-src pre-build
	@echo "Build for linux agent"
	GOOS=linux GOARCH=amd64 $(GO_BUILD_PIE) -o $(GO_SPACE)/bin/linux_amd64/amazon-ssm-agent -v \
					$(GO_SPACE)/core/agent.go $(GO_SPACE)/core/agent_unix.go $(GO_SPACE)/core/agent_parser.go
	GOOS=linux GOARCH=amd64 $(GO_BUILD_PIE) -o $(GO_SPACE)/bin/linux_amd64/ssm-agent-worker -v \
					$(GO_SPACE)/agent/agent.go $(GO_SPACE)/agent/agent_unix.go $(GO_SPACE)/agent/agent_parser.go
	GOOS=linux GOARCH=amd64 $(GO_BUILD_PIE) -o $(GO_SPACE)/bin/linux_amd64/updater -v \
					$(GO_SPACE)/agent/update/updater/updater.go $(GO_SPACE)/agent/update/updater/updater_unix.go
	GOOS=linux GOARCH=amd64 $(GO_BUILD_PIE) -o $(GO_SPACE)/bin/linux_amd64/ssm-cli -v \
					$(GO_SPACE)/agent/cli-main/cli-main.go
	GOOS=linux GOARCH=amd64 $(GO_BUILD_PIE) -o $(GO_SPACE)/bin/linux_amd64/ssm-document-worker -v \
					$(GO_SPACE)/agent/framework/processor/executer/outofproc/worker/main.go
	GOOS=linux GOARCH=amd64 $(GO_BUILD_PIE) -o $(GO_SPACE)/bin/linux_amd64/ssm-session-logger -v \
					$(GO_SPACE)/agent/session/logging/main.go
	GOOS=linux GOARCH=amd64 $(GO_BUILD_PIE) -o $(GO_SPACE)/bin/linux_amd64/ssm-session-worker -v \
					$(GO_SPACE)/agent/framework/processor/executer/outofproc/sessionworker/main.go

.PHONY: build-freebsd
build-freebsd: checkstyle copy-src pre-build
	@echo "Build for freebsd agent"
	GOOS=freebsd GOARCH=amd64 $(GO_BUILD) -o $(GO_SPACE)/bin/freebsd_amd64/amazon-ssm-agent -v \
                        $(GO_SPACE)/core/agent.go $(GO_SPACE)/core/agent_unix.go $(GO_SPACE)/core/agent_parser.go
	GOOS=freebsd GOARCH=amd64 $(GO_BUILD) -o $(GO_SPACE)/bin/freebsd_amd64/ssm-agent-worker -v \
					$(GO_SPACE)/agent/agent.go $(GO_SPACE)/agent/agent_unix.go $(GO_SPACE)/agent/agent_parser.go
	GOOS=freebsd GOARCH=amd64 $(GO_BUILD) -o $(GO_SPACE)/bin/freebsd_amd64/ssm-cli -v \
					$(GO_SPACE)/agent/cli-main/cli-main.go
	GOOS=freebsd GOARCH=amd64 $(GO_BUILD) -o $(GO_SPACE)/bin/freebsd_amd64/ssm-document-worker -v \
					$(GO_SPACE)/agent/framework/processor/executer/outofproc/worker/main.go
	GOOS=freebsd GOARCH=amd64 $(GO_BUILD) -o $(GO_SPACE)/bin/freebsd_amd64/ssm-session-logger -v \
					$(GO_SPACE)/agent/session/logging/main.go
	GOOS=freebsd GOARCH=amd64 $(GO_BUILD) -o $(GO_SPACE)/bin/freebsd_amd64/ssm-session-worker -v \
					$(GO_SPACE)/agent/framework/processor/executer/outofproc/sessionworker/main.go

.PHONY: build-darwin
build-darwin: checkstyle copy-src pre-build
	@echo "Build for darwin agent"
	GOOS=darwin GOARCH=amd64 $(GO_BUILD) -o $(GO_SPACE)/bin/darwin_amd64/amazon-ssm-agent -v \
                    $(GO_SPACE)/core/agent.go $(GO_SPACE)/core/agent_unix.go $(GO_SPACE)/core/agent_parser.go
	GOOS=darwin GOARCH=amd64 $(GO_BUILD) -o $(GO_SPACE)/bin/darwin_amd64/ssm-agent-worker -v \
					$(GO_SPACE)/agent/agent.go $(GO_SPACE)/agent/agent_unix.go $(GO_SPACE)/agent/agent_parser.go
	GOOS=darwin GOARCH=amd64 $(GO_BUILD) -o $(GO_SPACE)/bin/darwin_amd64/updater -v \
					$(GO_SPACE)/agent/update/updater/updater.go $(GO_SPACE)/agent/update/updater/updater_unix.go
	GOOS=darwin GOARCH=amd64 $(GO_BUILD) -o $(GO_SPACE)/bin/darwin_amd64/ssm-cli -v \
					$(GO_SPACE)/agent/cli-main/cli-main.go
	GOOS=darwin GOARCH=amd64 $(GO_BUILD) -o $(GO_SPACE)/bin/darwin_amd64/ssm-document-worker -v \
					$(GO_SPACE)/agent/framework/processor/executer/outofproc/worker/main.go

.PHONY: build-windows
build-windows: checkstyle copy-src pre-build
	@echo "Rebuild for windows agent"
	GOOS=windows GOARCH=amd64 $(GO_BUILD) -o $(GO_SPACE)/bin/windows_amd64/amazon-ssm-agent.exe -v \
					$(GO_SPACE)/core/agent.go $(GO_SPACE)/core/agent_windows.go $(GO_SPACE)/core/agent_parser.go
	GOOS=windows GOARCH=amd64 $(GO_BUILD) -o $(GO_SPACE)/bin/windows_amd64/ssm-agent-worker.exe -v \
					$(GO_SPACE)/agent/agent.go $(GO_SPACE)/agent/agent_windows.go $(GO_SPACE)/agent/agent_parser.go
	GOOS=windows GOARCH=amd64 $(GO_BUILD) -o $(GO_SPACE)/bin/windows_amd64/updater.exe -v \
					$(GO_SPACE)/agent/update/updater/updater.go $(GO_SPACE)/agent/update/updater/updater_windows.go
	GOOS=windows GOARCH=amd64 $(GO_BUILD) -o $(GO_SPACE)/bin/windows_amd64/ssm-cli.exe -v \
					$(GO_SPACE)/agent/cli-main/cli-main.go
	GOOS=windows GOARCH=amd64 $(GO_BUILD) -o $(GO_SPACE)/bin/windows_amd64/ssm-document-worker.exe -v \
					$(GO_SPACE)/agent/framework/processor/executer/outofproc/worker/main.go
	GOOS=windows GOARCH=amd64 $(GO_BUILD) -o $(GO_SPACE)/bin/windows_amd64/ssm-session-logger.exe -v \
					$(GO_SPACE)/agent/session/logging/main.go
	GOOS=windows GOARCH=amd64 $(GO_BUILD) -o $(GO_SPACE)/bin/windows_amd64/ssm-session-worker.exe -v \
					$(GO_SPACE)/agent/framework/processor/executer/outofproc/sessionworker/main.go

.PHONY: build-linux-386
build-linux-386: checkstyle copy-src pre-build
	@echo "Build for linux agent"
	GOOS=linux GOARCH=386 $(GO_BUILD) -o $(GO_SPACE)/bin/linux_386/amazon-ssm-agent -v \
                        $(GO_SPACE)/core/agent.go $(GO_SPACE)/core/agent_unix.go $(GO_SPACE)/core/agent_parser.go
	GOOS=linux GOARCH=386 $(GO_BUILD) -o $(GO_SPACE)/bin/linux_386/ssm-agent-worker -v \
					$(GO_SPACE)/agent/agent.go $(GO_SPACE)/agent/agent_unix.go $(GO_SPACE)/agent/agent_parser.go
	GOOS=linux GOARCH=386 $(GO_BUILD) -o $(GO_SPACE)/bin/linux_386/updater -v \
					$(GO_SPACE)/agent/update/updater/updater.go $(GO_SPACE)/agent/update/updater/updater_unix.go
	GOOS=linux GOARCH=386 $(GO_BUILD) -o $(GO_SPACE)/bin/linux_386/ssm-cli -v \
					$(GO_SPACE)/agent/cli-main/cli-main.go
	GOOS=linux GOARCH=386 $(GO_BUILD) -o $(GO_SPACE)/bin/linux_386/ssm-document-worker -v \
					$(GO_SPACE)/agent/framework/processor/executer/outofproc/worker/main.go
	GOOS=linux GOARCH=386 $(GO_BUILD) -o $(GO_SPACE)/bin/linux_386/ssm-session-logger -v \
					$(GO_SPACE)/agent/session/logging/main.go
	GOOS=linux GOARCH=386 $(GO_BUILD) -o $(GO_SPACE)/bin/linux_386/ssm-session-worker -v \
					$(GO_SPACE)/agent/framework/processor/executer/outofproc/sessionworker/main.go

.PHONY: build-darwin-386
build-darwin-386: checkstyle copy-src pre-build
	@echo "Build for darwin agent"
	GOOS=darwin GOARCH=386 $(GO_BUILD) -o $(GO_SPACE)/bin/darwin_386/amazon-ssm-agent -v \
                    $(GO_SPACE)/core/agent.go $(GO_SPACE)/core/agent_unix.go $(GO_SPACE)/core/agent_parser.go
	GOOS=darwin GOARCH=386 $(GO_BUILD) -o $(GO_SPACE)/bin/darwin_386/ssm-agent-worker -v \
					$(GO_SPACE)/agent/agent.go $(GO_SPACE)/agent/agent_unix.go $(GO_SPACE)/agent/agent_parser.go
	GOOS=darwin GOARCH=386 $(GO_BUILD) -o $(GO_SPACE)/bin/darwin_386/updater -v \
					$(GO_SPACE)/agent/update/updater/updater.go $(GO_SPACE)/agent/update/updater/updater_unix.go
	GOOS=darwin GOARCH=386 $(GO_BUILD) -o $(GO_SPACE)/bin/darwin_386/ssm-cli -v \
					$(GO_SPACE)/agent/cli-main/cli-main.go
	GOOS=darwin GOARCH=386 $(GO_BUILD) -o $(GO_SPACE)/bin/darwin_386/ssm-document-worker -v \
					$(GO_SPACE)/agent/framework/processor/executer/outofproc/worker/main.go

.PHONY: build-windows-386
build-windows-386: checkstyle copy-src pre-build
	@echo "Rebuild for windows agent"
	GOOS=windows GOARCH=386 $(GO_BUILD) -o $(GO_SPACE)/bin/windows_386/amazon-ssm-agent.exe -v \
					$(GO_SPACE)/core/agent.go $(GO_SPACE)/core/agent_windows.go $(GO_SPACE)/core/agent_parser.go
	GOOS=windows GOARCH=386 $(GO_BUILD) -o $(GO_SPACE)/bin/windows_386/ssm-agent-worker.exe -v \
					$(GO_SPACE)/agent/agent.go $(GO_SPACE)/agent/agent_windows.go $(GO_SPACE)/agent/agent_parser.go
	GOOS=windows GOARCH=386 $(GO_BUILD) -o $(GO_SPACE)/bin/windows_386/updater.exe -v \
					$(GO_SPACE)/agent/update/updater/updater.go $(GO_SPACE)/agent/update/updater/updater_windows.go
	GOOS=windows GOARCH=386 $(GO_BUILD) -o $(GO_SPACE)/bin/windows_386/ssm-cli.exe -v \
					$(GO_SPACE)/agent/cli-main/cli-main.go
	GOOS=windows GOARCH=386 $(GO_BUILD) -o $(GO_SPACE)/bin/windows_386/ssm-document-worker.exe -v \
					$(GO_SPACE)/agent/framework/processor/executer/outofproc/worker/main.go
	GOOS=windows GOARCH=386 $(GO_BUILD) -o $(GO_SPACE)/bin/windows_386/ssm-session-logger.exe -v \
					$(GO_SPACE)/agent/session/logging/main.go
	GOOS=windows GOARCH=386 $(GO_BUILD) -o $(GO_SPACE)/bin/windows_386/ssm-session-worker.exe -v \
					$(GO_SPACE)/agent/framework/processor/executer/outofproc/sessionworker/main.go

.PHONY: build-arm
build-arm: checkstyle copy-src pre-build
	@echo "Build for ARM platforms"
	GOOS=linux GOARCH=arm GOARM=6 $(GO_BUILD) -o $(GO_SPACE)/bin/linux_arm/amazon-ssm-agent -v \
                        $(GO_SPACE)/core/agent.go $(GO_SPACE)/core/agent_unix.go $(GO_SPACE)/core/agent_parser.go
	GOOS=linux GOARCH=arm GOARM=6 $(GO_BUILD) -o $(GO_SPACE)/bin/linux_arm/ssm-agent-worker -v \
						$(GO_SPACE)/agent/agent.go $(GO_SPACE)/agent/agent_unix.go $(GO_SPACE)/agent/agent_parser.go
	GOOS=linux GOARCH=arm GOARM=6 $(GO_BUILD) -o $(GO_SPACE)/bin/linux_arm/updater -v \
						$(GO_SPACE)/agent/update/updater/updater.go $(GO_SPACE)/agent/update/updater/updater_unix.go
	GOOS=linux GOARCH=arm GOARM=6 $(GO_BUILD) -o $(GO_SPACE)/bin/linux_arm/ssm-cli -v \
						$(GO_SPACE)/agent/cli-main/cli-main.go
	GOOS=linux GOARCH=arm GOARM=6 $(GO_BUILD) -o $(GO_SPACE)/bin/linux_arm/ssm-document-worker -v \
						$(GO_SPACE)/agent/framework/processor/executer/outofproc/worker/main.go
	GOOS=linux GOARCH=arm GOARM=6 $(GO_BUILD) -o $(GO_SPACE)/bin/linux_arm/ssm-session-logger -v \
						$(GO_SPACE)/agent/session/logging/main.go
	GOOS=linux GOARCH=arm GOARM=6 $(GO_BUILD) -o $(GO_SPACE)/bin/linux_arm/ssm-session-worker -v \
						$(GO_SPACE)/agent/framework/processor/executer/outofproc/sessionworker/main.go

# Production binaries are built using GO_BUILD_PIE
.PHONY: build-arm64
build-arm64: checkstyle copy-src pre-build
	@echo "Build for ARM64 platforms"
	GOOS=linux GOARCH=arm64 $(GO_BUILD) -o $(GO_SPACE)/bin/linux_arm64/amazon-ssm-agent -v \
                    $(GO_SPACE)/core/agent.go $(GO_SPACE)/core/agent_unix.go $(GO_SPACE)/core/agent_parser.go
	GOOS=linux GOARCH=arm64 $(GO_BUILD) -o $(GO_SPACE)/bin/linux_arm64/ssm-agent-worker -v \
					$(GO_SPACE)/agent/agent.go $(GO_SPACE)/agent/agent_unix.go $(GO_SPACE)/agent/agent_parser.go
	GOOS=linux GOARCH=arm64 $(GO_BUILD) -o $(GO_SPACE)/bin/linux_arm64/updater -v \
					$(GO_SPACE)/agent/update/updater/updater.go $(GO_SPACE)/agent/update/updater/updater_unix.go
	GOOS=linux GOARCH=arm64 $(GO_BUILD) -o $(GO_SPACE)/bin/linux_arm64/ssm-cli -v \
					$(GO_SPACE)/agent/cli-main/cli-main.go
	GOOS=linux GOARCH=arm64 $(GO_BUILD) -o $(GO_SPACE)/bin/linux_arm64/ssm-document-worker -v \
					$(GO_SPACE)/agent/framework/processor/executer/outofproc/worker/main.go
	GOOS=linux GOARCH=arm64 $(GO_BUILD) -o $(GO_SPACE)/bin/linux_arm64/ssm-session-logger -v \
					$(GO_SPACE)/agent/session/logging/main.go
	GOOS=linux GOARCH=arm64 $(GO_BUILD) -o $(GO_SPACE)/bin/linux_arm64/ssm-session-worker -v \
					$(GO_SPACE)/agent/framework/processor/executer/outofproc/sessionworker/main.go

.PHONY: copy-src
copy-src:
	rm -rf $(GOTEMPCOPYPATH)
	mkdir -p $(GOTEMPCOPYPATH)
	@echo "copying files to $(GOTEMPCOPYPATH)"
	$(COPY) -r $(GO_SPACE)/agent $(GOTEMPCOPYPATH)
	$(COPY) -r $(GO_SPACE)/core $(GOTEMPCOPYPATH)
	$(COPY) -r $(GO_SPACE)/common $(GOTEMPCOPYPATH)

.PHONY: copy-tests-src
copy-tests-src:
ifeq ($(BRAZIL_BUILD), true)
	@echo "copying test files to $(GOTEMPCOPYPATH)"
	$(COPY) -r $(GO_SPACE)/internal $(GOTEMPCOPYPATH)
endif

.PHONY: copy-package-dep
copy-package-dep: copy-src pre-build
	@echo "Copying packaging dependencies to $(GO_SPACE)/bin/package_dep"
	mkdir -p $(GO_SPACE)/bin/package_dep

	$(COPY) -r $(GO_SPACE)/Tools $(GO_SPACE)/bin/package_dep/
	$(COPY) -r $(GO_SPACE)/packaging $(GO_SPACE)/bin/package_dep/

	$(COPY) -r $(GO_SPACE)/amazon-ssm-agent.json.template $(GO_SPACE)/bin/package_dep/
	$(COPY) -r $(GO_SPACE)/amazon-ssm-agent.spec $(GO_SPACE)/bin/package_dep/

	$(COPY) -r $(GO_SPACE)/seelog_unix.xml $(GO_SPACE)/bin/package_dep/
	$(COPY) -r $(GO_SPACE)/seelog_windows.xml.template $(GO_SPACE)/bin/package_dep/

	$(COPY) -r $(GO_SPACE)/NOTICE.md $(GO_SPACE)/bin/package_dep/
	$(COPY) -r $(GO_SPACE)/RELEASENOTES.md $(GO_SPACE)/bin/package_dep/
	$(COPY) -r $(GO_SPACE)/README.md $(GO_SPACE)/bin/package_dep/
	$(COPY) -r $(GO_SPACE)/LICENSE $(GO_SPACE)/bin/package_dep/
	$(COPY) -r $(GO_SPACE)/VERSION $(GO_SPACE)/bin/package_dep/

	cd $(GO_SPACE) && zip -q -y -r $(GO_SPACE)/bin/gosrc.zip agent common core vendor && cd -

.PHONY: remove-prepacked-folder
remove-prepacked-folder:
	rm -rf $(GO_SPACE)/bin/prepacked

.PHONY: prepack-linux
prepack-linux:
	mkdir -p $(GO_SPACE)/bin/prepacked/linux_amd64
	$(COPY) $(GO_SPACE)/bin/linux_amd64/amazon-ssm-agent $(GO_SPACE)/bin/prepacked/linux_amd64/amazon-ssm-agent
	$(COPY) $(GO_SPACE)/bin/linux_amd64/ssm-agent-worker $(GO_SPACE)/bin/prepacked/linux_amd64/ssm-agent-worker
	$(COPY) $(GO_SPACE)/bin/linux_amd64/updater $(GO_SPACE)/bin/prepacked/linux_amd64/updater
	$(COPY) $(GO_SPACE)/bin/linux_amd64/ssm-cli $(GO_SPACE)/bin/prepacked/linux_amd64/ssm-cli
	$(COPY) $(GO_SPACE)/bin/linux_amd64/ssm-document-worker $(GO_SPACE)/bin/prepacked/linux_amd64/ssm-document-worker
	$(COPY) $(GO_SPACE)/bin/linux_amd64/ssm-session-worker $(GO_SPACE)/bin/prepacked/linux_amd64/ssm-session-worker
	$(COPY) $(GO_SPACE)/bin/linux_amd64/ssm-session-logger $(GO_SPACE)/bin/prepacked/linux_amd64/ssm-session-logger
	$(COPY) $(GO_SPACE)/bin/amazon-ssm-agent.json.template $(GO_SPACE)/bin/prepacked/linux_amd64/amazon-ssm-agent.json.template
	$(COPY) $(GO_SPACE)/bin/seelog_unix.xml $(GO_SPACE)/bin/prepacked/linux_amd64/seelog.xml.template
	$(COPY) $(GO_SPACE)/bin/LICENSE $(GO_SPACE)/bin/prepacked/linux_amd64/LICENSE
	$(COPY) $(GO_SPACE)/bin/NOTICE.md $(GO_SPACE)/bin/prepacked/linux_amd64/NOTICE.md

.PHONY: prepack-linux-arm64
prepack-linux-arm64:
	mkdir -p $(GO_SPACE)/bin/prepacked/linux_arm64
	$(COPY) $(GO_SPACE)/bin/linux_arm64/amazon-ssm-agent $(GO_SPACE)/bin/prepacked/linux_arm64/amazon-ssm-agent
	$(COPY) $(GO_SPACE)/bin/linux_arm64/ssm-agent-worker $(GO_SPACE)/bin/prepacked/linux_arm64/ssm-agent-worker
	$(COPY) $(GO_SPACE)/bin/linux_arm64/updater $(GO_SPACE)/bin/prepacked/linux_arm64/updater
	$(COPY) $(GO_SPACE)/bin/linux_arm64/ssm-cli $(GO_SPACE)/bin/prepacked/linux_arm64/ssm-cli
	$(COPY) $(GO_SPACE)/bin/linux_arm64/ssm-document-worker $(GO_SPACE)/bin/prepacked/linux_arm64/ssm-document-worker
	$(COPY) $(GO_SPACE)/bin/linux_arm64/ssm-session-worker $(GO_SPACE)/bin/prepacked/linux_arm64/ssm-session-worker
	$(COPY) $(GO_SPACE)/bin/linux_arm64/ssm-session-logger $(GO_SPACE)/bin/prepacked/linux_arm64/ssm-session-logger
	$(COPY) $(GO_SPACE)/bin/amazon-ssm-agent.json.template $(GO_SPACE)/bin/prepacked/linux_arm64/amazon-ssm-agent.json.template
	$(COPY) $(GO_SPACE)/bin/seelog_unix.xml $(GO_SPACE)/bin/prepacked/linux_arm64/seelog.xml.template
	$(COPY) $(GO_SPACE)/bin/LICENSE $(GO_SPACE)/bin/prepacked/linux_arm64/LICENSE
	$(COPY) $(GO_SPACE)/bin/NOTICE.md $(GO_SPACE)/bin/prepacked/linux_arm64/NOTICE.md

.PHONY: prepack-windows
prepack-windows:
	mkdir -p $(GO_SPACE)/bin/prepacked/windows_amd64
	$(COPY) $(GO_SPACE)/bin/windows_amd64/amazon-ssm-agent.exe $(GO_SPACE)/bin/prepacked/windows_amd64/amazon-ssm-agent.exe
	$(COPY) $(GO_SPACE)/bin/windows_amd64/ssm-agent-worker.exe $(GO_SPACE)/bin/prepacked/windows_amd64/ssm-agent-worker.exe
	$(COPY) $(GO_SPACE)/bin/windows_amd64/updater.exe $(GO_SPACE)/bin/prepacked/windows_amd64/updater.exe
	$(COPY) $(GO_SPACE)/bin/windows_amd64/ssm-cli.exe $(GO_SPACE)/bin/prepacked/windows_amd64/ssm-cli.exe
	$(COPY) $(GO_SPACE)/bin/windows_amd64/ssm-document-worker.exe $(GO_SPACE)/bin/prepacked/windows_amd64/ssm-document-worker.exe
	$(COPY) $(GO_SPACE)/bin/windows_amd64/ssm-session-worker.exe $(GO_SPACE)/bin/prepacked/windows_amd64/ssm-session-worker.exe
	$(COPY) $(GO_SPACE)/bin/windows_amd64/ssm-session-logger.exe $(GO_SPACE)/bin/prepacked/windows_amd64/ssm-session-logger.exe
	$(COPY) $(GO_SPACE)/bin/amazon-ssm-agent.json.template $(GO_SPACE)/bin/prepacked/windows_amd64/amazon-ssm-agent.json.template
	$(COPY) $(GO_SPACE)/bin/seelog_windows.xml.template $(GO_SPACE)/bin/prepacked/windows_amd64/seelog.xml.template
	$(COPY) $(GO_SPACE)/bin/LICENSE $(GO_SPACE)/bin/prepacked/windows_amd64/LICENSE
	$(COPY) $(GO_SPACE)/bin/NOTICE.md $(GO_SPACE)/bin/prepacked/windows_amd64/NOTICE.md

.PHONY: prepack-linux-386
prepack-linux-386:
	mkdir -p $(GO_SPACE)/bin/prepacked/linux_386
	$(COPY) $(GO_SPACE)/bin/linux_386/amazon-ssm-agent $(GO_SPACE)/bin/prepacked/linux_386/amazon-ssm-agent
	$(COPY) $(GO_SPACE)/bin/linux_386/ssm-agent-worker $(GO_SPACE)/bin/prepacked/linux_386/ssm-agent-worker
	$(COPY) $(GO_SPACE)/bin/linux_386/updater $(GO_SPACE)/bin/prepacked/linux_386/updater
	$(COPY) $(GO_SPACE)/bin/linux_386/ssm-cli $(GO_SPACE)/bin/prepacked/linux_386/ssm-cli
	$(COPY) $(GO_SPACE)/bin/linux_386/ssm-document-worker $(GO_SPACE)/bin/prepacked/linux_386/ssm-document-worker
	$(COPY) $(GO_SPACE)/bin/linux_386/ssm-session-worker $(GO_SPACE)/bin/prepacked/linux_386/ssm-session-worker
	$(COPY) $(GO_SPACE)/bin/linux_386/ssm-session-logger $(GO_SPACE)/bin/prepacked/linux_386/ssm-session-logger
	$(COPY) $(GO_SPACE)/bin/amazon-ssm-agent.json.template $(GO_SPACE)/bin/prepacked/linux_386/amazon-ssm-agent.json.template
	$(COPY) $(GO_SPACE)/bin/seelog_unix.xml $(GO_SPACE)/bin/prepacked/linux_386/seelog.xml.template
	$(COPY) $(GO_SPACE)/bin/LICENSE $(GO_SPACE)/bin/prepacked/linux_386/LICENSE
	$(COPY) $(GO_SPACE)/bin/NOTICE.md $(GO_SPACE)/bin/prepacked/linux_386/NOTICE.md

.PHONY: prepack-windows-386
prepack-windows-386:
	mkdir -p $(GO_SPACE)/bin/prepacked/windows_386
	$(COPY) $(GO_SPACE)/bin/windows_386/amazon-ssm-agent.exe $(GO_SPACE)/bin/prepacked/windows_386/amazon-ssm-agent.exe
	$(COPY) $(GO_SPACE)/bin/windows_386/ssm-agent-worker.exe $(GO_SPACE)/bin/prepacked/windows_386/ssm-agent-worker.exe
	$(COPY) $(GO_SPACE)/bin/windows_386/updater.exe $(GO_SPACE)/bin/prepacked/windows_386/updater.exe
	$(COPY) $(GO_SPACE)/bin/windows_386/ssm-cli.exe $(GO_SPACE)/bin/prepacked/windows_386/ssm-cli.exe
	$(COPY) $(GO_SPACE)/bin/windows_386/ssm-document-worker.exe $(GO_SPACE)/bin/prepacked/windows_386/ssm-document-worker.exe
	$(COPY) $(GO_SPACE)/bin/windows_386/ssm-session-worker.exe $(GO_SPACE)/bin/prepacked/windows_386/ssm-session-worker.exe
	$(COPY) $(GO_SPACE)/bin/windows_386/ssm-session-logger.exe $(GO_SPACE)/bin/prepacked/windows_386/ssm-session-logger.exe
	$(COPY) $(GO_SPACE)/bin/amazon-ssm-agent.json.template $(GO_SPACE)/bin/prepacked/windows_386/amazon-ssm-agent.json.template
	$(COPY) $(GO_SPACE)/bin/seelog_windows.xml.template $(GO_SPACE)/bin/prepacked/windows_386/seelog.xml.template
	$(COPY) $(GO_SPACE)/bin/LICENSE $(GO_SPACE)/bin/prepacked/windows_386/LICENSE
	$(COPY) $(GO_SPACE)/bin/NOTICE.md $(GO_SPACE)/bin/prepacked/windows_386/NOTICE.md

.PHONY: create-package-folder
create-package-folder:
	mkdir -p $(GO_SPACE)/bin/updates/amazon-ssm-agent/`cat $(GO_SPACE)/VERSION`/
	mkdir -p $(GO_SPACE)/bin/updates/amazon-ssm-agent-updater/`cat $(GO_SPACE)/VERSION`/

.PHONY: package-linux
package-linux: package-rpm-386 package-deb-386 package-rpm package-deb package-deb-arm package-deb-arm64 package-rpm-arm64 package-binaries-linux-amd64 package-binaries-linux-arm64
	$(GO_SPACE)/Tools/src/create_linux_package.sh

.PHONY: package-windows
package-windows: package-win-386 package-win
	$(GO_SPACE)/Tools/src/create_windows_package.sh
	$(GO_SPACE)/Tools/src/create_windows_nano_package.sh

.PHONY: create-source-archive
create-source-archive:
	$(eval SOURCE_PACKAGE_NAME := amazon-ssm-agent-`cat $(GO_SPACE)/VERSION`)
	git archive --prefix=$(SOURCE_PACKAGE_NAME)/ --format=tar HEAD | gzip -c > $(SOURCE_PACKAGE_NAME).tar.gz

.PHONY: package-rpm
package-rpm: create-package-folder
	$(GO_SPACE)/Tools/src/create_rpm.sh linux_amd64

.PHONY: package-deb
package-deb: create-package-folder
	$(GO_SPACE)/Tools/src/create_deb.sh amd64

.PHONY: package-win
package-win: create-package-folder
	$(GO_SPACE)/Tools/src/create_win.sh

.PHONY: package-darwin
package-darwin:
	$(GO_SPACE)/Tools/src/create_darwin.sh

.PHONY: package-rpm-386
package-rpm-386: create-package-folder
	$(GO_SPACE)/Tools/src/create_rpm.sh linux_386

.PHONY: package-deb-386
package-deb-386: create-package-folder
	$(GO_SPACE)/Tools/src/create_deb.sh 386

.PHONY: package-win-386
package-win-386: create-package-folder
	$(GO_SPACE)/Tools/src/create_win_386.sh

.PHONY: package-deb-arm
package-deb-arm: create-package-folder
	$(GO_SPACE)/Tools/src/create_deb.sh arm

.PHONY: package-deb-arm64
package-deb-arm64: create-package-folder
	$(GO_SPACE)/Tools/src/create_deb.sh arm64

.PHONY: package-rpm-arm64
package-rpm-arm64: create-package-folder
	$(GO_SPACE)/Tools/src/create_rpm.sh linux_arm64

.PHONY: package-binaries-linux-amd64
package-binaries-linux-amd64: create-package-folder
	$(GO_SPACE)/Tools/src/create_binaries_tar.sh linux_amd64

.PHONY: package-binaries-linux-arm64
package-binaries-linux-arm64: create-package-folder
	$(GO_SPACE)/Tools/src/create_binaries_tar.sh linux_arm64

.PHONY: get-tools
get-tools:
	go get -u github.com/nsf/gocode
	go get -u golang.org/x/tools/cmd/oracle
	go get -u golang.org/x/tools/go/loader
	go get -u golang.org/x/tools/go/types

.PHONY: --quick-integtest
--quick-integtest:
	# if you want to restrict to some specific package, sample below
	# go test -v -gcflags "-N -l" -timeout 20m -tags=integration github.com/aws/amazon-ssm-agent/agent/fileutil/...
	go test -gcflags "-N -l" -timeout 20m -tags=integration github.com/aws/amazon-ssm-agent/agent/...

.PHONY: --quick-integtest-core
--quick-integtest-core:
	# if you want to restrict to some specific package, sample below
	# go test -v -gcflags "-N -l" -tags=integration github.com/aws/amazon-ssm-agent/agent/fileutil/...
	go test -gcflags "-N -l" -tags=integration github.com/aws/amazon-ssm-agent/core/...


.PHONY: --quick-test
--quick-test:
	# if you want to test a specific package, you can add the package name instead of the dots. Sample below
	# go test -gcflags "-N -l" -timeout 20m github.com/aws/amazon-ssm-agent/agent/task
	go test -gcflags "-N -l" -timeout 20m github.com/aws/amazon-ssm-agent/agent/...

--quick-test-core:
	# if you want to test a specific package, you can add the package name instead of the dots. Sample below
	# go test -gcflags "-N -l" github.com/aws/amazon-ssm-agent/agent/task
	go test -gcflags "-N -l" github.com/aws/amazon-ssm-agent/core/...

.PHONY: --quick-e2e
--quick-e2e:
	# if you want to restrict to some specific package, sample below
	# go test -v -gcflags "-N -l" -timeout 20m -tags=integration github.com/aws/amazon-ssm-agent/agent/fileutil/...
	go test -gcflags "-N -l" -timeout 20m -tags=e2e github.com/aws/amazon-ssm-agent/agent/...

--quick-e2e-core:
	# if you want to restrict to some specific package, sample below
	# go test -v -gcflags "-N -l" -tags=integration github.com/aws/amazon-ssm-agent/agent/fileutil/...
	go test -gcflags "-N -l" -tags=e2e github.com/aws/amazon-ssm-agent/core/...

.PHONY: gen-report
gen-report:
	$(GO_SPACE)/Tools/src/gen-report.sh
