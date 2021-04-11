COPY := cp -p
GO_BUILD_NOPIE := CGO_ENABLED=0 go build -ldflags "-s -w" -trimpath
GO_BUILD_PIE := go build -ldflags "-s -w -extldflags=-Wl,-z,now,-z,relro,-z,defs" -buildmode=pie -trimpath

# Default build configuration, can be overridden at build time.
GOARCH?=$(shell go env GOARCH)
GOOS?=$(shell go env GOOS)
GO_CORE_SRC_TYPE?=unix
GO_WORKER_SRC_TYPE?=unix
GO_BUILD?=$(GO_BUILD_NOPIE)

GO_SPACE?=$(CURDIR)
GOTEMPPATH?=$(GO_SPACE)/build/private
GOTEMPCOPYPATH?=$(GOTEMPPATH)/src/github.com/aws/amazon-ssm-agent
GOPATH:=$(GOTEMPPATH):$(GO_SPACE)/vendor:$(GOPATH)
export GOPATH
export GO_SPACE

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

release:: clean quick-integtest checkstyle pre-release build-darwin cpy-plugins copy-win-dep finalize

package-src:: clean quick-integtest checkstyle pre-release cpy-plugins finalize

finalize:: build-tests copy-package-dep

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
quick-integtest: copy-src pre-build pre-release --quick-integtest --quick-integtest-core --quick-integtest-common

.PHONY: quick-test
quick-test: copy-src pre-build pre-release --quick-test --quick-test-core --quick-test-common

.PHONY: quick-test-core
quick-test-core: copy-src pre-build pre-release --quick-test-core

.PHONY: quick-test-common
quick-test-common: copy-src pre-build pre-release --quick-test-common

.PHONY: quick-e2e
quick-e2e: copy-src pre-build pre-release --quick-e2e --quick-e2e-core --quick-e2e-common

.PHONY: test-all
test-all: copy-src pre-build pre-release checkstyle --quick-integtest --quick-integtest-core --quick-integtest-common --quick-test --quick-test-core --quick-test-common --quick-e2e --quick-e2e-core --quick-e2e-common

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
# Remove overrides for GOARCH and GOOS when invoking 'go run', since we want the local host's settings
	GOARCH= GOOS= go run $(GO_SPACE)/agent/version/versiongenerator/version-gen.go
	$(COPY) $(GO_SPACE)/VERSION $(GO_SPACE)/bin/

# General build recipe. Defaults to generating a linux/amd64 non-PIE build, but can be overriden
# by setting appropriate variables.
.PHONY: build-any-%
build-any-%: checkstyle copy-src pre-build
	@echo "Build for $(GOARCH) $(GOOS) agent"
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO_BUILD) -o $(GO_SPACE)/bin/$(GOOS)_$(GOARCH)/amazon-ssm-agent$(EXE_EXT) -v \
	    $(GO_SPACE)/core/agent.go $(GO_SPACE)/core/agent_$(GO_CORE_SRC_TYPE).go $(GO_SPACE)/core/agent_parser.go
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO_BUILD) -o $(GO_SPACE)/bin/$(GOOS)_$(GOARCH)/ssm-agent-worker$(EXE_EXT) -v \
	    $(GO_SPACE)/agent/agent.go $(GO_SPACE)/agent/agent_$(GO_WORKER_SRC_TYPE).go $(GO_SPACE)/agent/agent_parser.go
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO_BUILD) -o $(GO_SPACE)/bin/$(GOOS)_$(GOARCH)/updater$(EXE_EXT) -v \
	    $(GO_SPACE)/agent/update/updater/updater.go $(GO_SPACE)/agent/update/updater/updater_$(GO_WORKER_SRC_TYPE).go
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO_BUILD) -o $(GO_SPACE)/bin/$(GOOS)_$(GOARCH)/ssm-cli$(EXE_EXT) -v \
	    $(GO_SPACE)/agent/cli-main/cli-main.go
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO_BUILD) -o $(GO_SPACE)/bin/$(GOOS)_$(GOARCH)/ssm-document-worker$(EXE_EXT) -v \
	    $(GO_SPACE)/agent/framework/processor/executer/outofproc/worker/main.go
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO_BUILD) -o $(GO_SPACE)/bin/$(GOOS)_$(GOARCH)/ssm-session-logger$(EXE_EXT) -v \
	    $(GO_SPACE)/agent/session/logging/main.go
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO_BUILD) -o $(GO_SPACE)/bin/$(GOOS)_$(GOARCH)/ssm-session-worker$(EXE_EXT) -v \
	    $(GO_SPACE)/agent/framework/processor/executer/outofproc/sessionworker/main.go
	@echo "Finished building $(GOARCH) $(GOOS) agent"

# Pre-defined recipes for various supported builds:

# Production binaries are built using GO_BUILD_PIE
.PHONY: build-linux
build-linux: GOARCH=amd64
build-linux: GOOS=linux
build-linux: GO_BUILD=$(GO_BUILD_PIE)
build-linux: build-any-amd64-linux

.PHONY: build-freebsd
build-freebsd: GOARCH=amd64
build-freebsd: GOOS=freebsd
build-freebsd: GO_BUILD=$(GO_BUILD_NOPIE)
build-freebsd: build-any-amd64-freebsd

.PHONY: build-darwin
build-darwin: GOARCH=amd64
build-darwin: GOOS=darwin
build-darwin: GO_BUILD=$(GO_BUILD_NOPIE)
build-darwin: GO_CORE_SRC_TYPE=darwin
build-darwin: build-any-darwin-amd64

.PHONY: build-windows
build-windows: GOOS=windows
build-windows: GOARCH=amd64
build-windows: GO_BUILD=$(GO_BUILD_NOPIE)
build-windows: EXE_EXT=.exe
build-windows: GO_CORE_SRC_TYPE=windows
build-windows: GO_WORKER_SRC_TYPE=windows
build-windows: build-any-windows-amd64

.PHONY: build-linux-386
build-linux-386: GOOS=linux
build-linux-386: GOARCH=386
build-linux-386: GO_BUILD=$(GO_BUILD_NOPIE)
build-linux-386: build-any-linux-386

.PHONY: build-windows-386
build-windows-386: GOOS=windows
build-windows-386: GOARCH=386
build-windows-386: GO_BUILD=$(GO_BUILD_NOPIE)
build-windows-386: EXE_EXT=.exe
build-windows-386: GO_CORE_SRC_TYPE=windows
build-windows-386: GO_WORKER_SRC_TYPE=windows
build-windows-386: build-any-windows-386

.PHONY: build-arm
build-arm: GOOS=linux
build-arm: GOARCH=arm
build-arm: GO_BUILD=GOARM=6 $(GO_BUILD_NOPIE)
build-arm: build-any-arm

.PHONY: build-arm64
build-arm64: GOOS=linux
build-arm64: GOARCH=arm64
build-arm64: GO_BUILD=$(GO_BUILD_NOPIE)
build-arm64: build-any-linux-arm64

.PHONY: copy-src
copy-src:
	rm -rf $(GOTEMPCOPYPATH)
	mkdir -p $(GOTEMPCOPYPATH)
	@echo "copying files to $(GOTEMPCOPYPATH)"
	$(COPY) -r $(GO_SPACE)/agent $(GOTEMPCOPYPATH)
	$(COPY) -r $(GO_SPACE)/core $(GOTEMPCOPYPATH)
	$(COPY) -r $(GO_SPACE)/common $(GOTEMPCOPYPATH)

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

# General prepack recipe. Details can be overridden by setting appropriate variables.
.PHONY: prepack-any-%
prepack-any-%: GOOS?=linux
prepack-any-%: GOARCH?=amd64
prepack-any-%:
	mkdir -p $(GO_SPACE)/bin/prepacked/$(GOOS)_$(GOARCH)
	$(COPY) $(GO_SPACE)/bin/$(GOOS)_$(GOARCH)/amazon-ssm-agent$(EXE_EXT) \
	    $(GO_SPACE)/bin/prepacked/$(GOOS)_$(GOARCH)/amazon-ssm-agent$(EXE_EXT)
	$(COPY) $(GO_SPACE)/bin/$(GOOS)_$(GOARCH)/ssm-agent-worker$(EXE_EXT) \
	    $(GO_SPACE)/bin/prepacked/$(GOOS)_$(GOARCH)/ssm-agent-worker$(EXE_EXT)
	$(COPY) $(GO_SPACE)/bin/$(GOOS)_$(GOARCH)/updater$(EXE_EXT) \
	    $(GO_SPACE)/bin/prepacked/$(GOOS)_$(GOARCH)/updater$(EXE_EXT)
	$(COPY) $(GO_SPACE)/bin/$(GOOS)_$(GOARCH)/ssm-cli$(EXE_EXT) \
	    $(GO_SPACE)/bin/prepacked/$(GOOS)_$(GOARCH)/ssm-cli$(EXE_EXT)
	$(COPY) $(GO_SPACE)/bin/$(GOOS)_$(GOARCH)/ssm-document-worker$(EXE_EXT) \
	    $(GO_SPACE)/bin/prepacked/$(GOOS)_$(GOARCH)/ssm-document-worker$(EXE_EXT)
	$(COPY) $(GO_SPACE)/bin/$(GOOS)_$(GOARCH)/ssm-session-worker$(EXE_EXT) \
	    $(GO_SPACE)/bin/prepacked/$(GOOS)_$(GOARCH)/ssm-session-worker$(EXE_EXT)
	$(COPY) $(GO_SPACE)/bin/$(GOOS)_$(GOARCH)/ssm-session-logger$(EXE_EXT) \
	    $(GO_SPACE)/bin/prepacked/$(GOOS)_$(GOARCH)/ssm-session-logger$(EXE_EXT)
	$(COPY) $(GO_SPACE)/bin/amazon-ssm-agent.json.template $(GO_SPACE)/bin/prepacked/$(GOOS)_$(GOARCH)/amazon-ssm-agent.json.template
	$(COPY) $(GO_SPACE)/bin/seelog_unix.xml $(GO_SPACE)/bin/prepacked/$(GOOS)_$(GOARCH)/seelog.xml.template
	$(COPY) $(GO_SPACE)/bin/LICENSE $(GO_SPACE)/bin/prepacked/$(GOOS)_$(GOARCH)/LICENSE
	$(COPY) $(GO_SPACE)/bin/NOTICE.md $(GO_SPACE)/bin/prepacked/$(GOOS)_$(GOARCH)/NOTICE.md

# Predefined prepack recipes for various supported builds
.PHONY: prepack-linux
prepack-linux: GOOS=linux
prepack-linux: GOARCH=amd64
prepack-linux: prepack-any-linux-amd64

.PHONY: prepack-linux-arm64
prepack-linux-arm64: GOOS=linux
prepack-linux-arm64: GOARCH=arm64
prepack-linux-arm64: prepack-any-linux-arm64

.PHONY: prepack-windows
prepack-windows: GOOS=windows
prepack-windows: GOARCH=amd64
prepack-windows: EXE_EXT=.exe
prepack-windows: prepack-any-windows-amd64

.PHONY: prepack-linux-386
prepack-linux-386: GOOS=linux
prepack-linux-386: GOARCH=386
prepack-linux-386: prepack-any-linux-386

.PHONY: prepack-windows-386
prepack-windows-386: GOOS=windows
prepack-windows-386: GOARCH=386
prepack-windows-386: EXE_EXT=.exe
prepack-windows-386: prepack-any-windows-386

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
	$(GO_SPACE)/Tools/src/create_darwin_package.sh

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

.PHONY: copy-tests-src
copy-tests-src:
	@echo "copying test files to $(GOTEMPCOPYPATH)"
	$(COPY) -r $(GO_SPACE)/internal $(GOTEMPCOPYPATH)

.PHONY: build-tests
build-tests: build-tests-linux build-tests-windows

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

.PHONY: --quick-integtest-common
--quick-integtest-common:
# if you want to restrict to some specific package, sample below
# go test -v -gcflags "-N -l" -tags=integration github.com/aws/amazon-ssm-agent/agent/common/task
	go test -gcflags "-N -l" -tags=integration github.com/aws/amazon-ssm-agent/common/...

.PHONY: --quick-test
--quick-test:
# if you want to test a specific package, you can add the package name instead of the dots. Sample below
# go test -gcflags "-N -l" -timeout 20m github.com/aws/amazon-ssm-agent/agent/task
	go test -gcflags "-N -l" -timeout 20m github.com/aws/amazon-ssm-agent/agent/...

--quick-test-core:
# if you want to test a specific package, you can add the package name instead of the dots. Sample below
# go test -gcflags "-N -l" github.com/aws/amazon-ssm-agent/agent/task
	go test -gcflags "-N -l" github.com/aws/amazon-ssm-agent/core/...

--quick-test-common:
# if you want to test a specific package, you can add the package name instead of the dots. Sample below
# go test -gcflags "-N -l" github.com/aws/amazon-ssm-agent/agent/common/task
	go test -gcflags "-N -l" github.com/aws/amazon-ssm-agent/common/...

.PHONY: --quick-e2e
--quick-e2e:
# if you want to restrict to some specific package, sample below
# go test -v -gcflags "-N -l" -timeout 20m -tags=integration github.com/aws/amazon-ssm-agent/agent/fileutil/...
	go test -gcflags "-N -l" -timeout 20m -tags=e2e github.com/aws/amazon-ssm-agent/agent/...

--quick-e2e-core:
# if you want to restrict to some specific package, sample below
# go test -v -gcflags "-N -l" -tags=integration github.com/aws/amazon-ssm-agent/agent/fileutil/...
	go test -gcflags "-N -l" -tags=e2e github.com/aws/amazon-ssm-agent/core/...

--quick-e2e-common:
# if you want to restrict to some specific package, sample below
# go test -v -gcflags "-N -l" -tags=integration github.com/aws/amazon-ssm-agent/agent/fileutil/...
	go test -gcflags "-N -l" -tags=e2e github.com/aws/amazon-ssm-agent/common/...

.PHONY: lint-all
lint-all: copy-package-dep
# if you want to configure what linters are run, edit .golangci.yml
# if you want to restrict to some specific package edit Tools/src/run_golangci-lint.sh
	$(GO_SPACE)/Tools/src/run_golangci-lint.sh

.PHONY: gen-report
gen-report:
	$(GO_SPACE)/Tools/src/gen-report.sh