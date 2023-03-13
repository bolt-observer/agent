REVISION := ${DESCRIBE}
REVISION += $(shell git describe)
REVISION += unknown
REVISION := $(word 1, $(REVISION))

BINARIES := $(wildcard cmd/*)
BINARIES_LINUX := $(addsuffix -linux,${BINARIES})
BINARIES_RASP := $(addsuffix -rasp,${BINARIES})
BINARIES_DARWIN := $(addsuffix -darwin,${BINARIES})

.PHONY: release
release: ${BINARIES}

.PHONY: all
all: ${BINARIES_LINUX} ${BINARIES_RASP} ${BINARIES_DARWIN}

.PHONY: ${BINARIES}
${BINARIES}:
	go get ./...
	cd cmd/$(@:cmd/%=%) ; CGO_ENABLED=0 go build -ldflags "-X main.GitRevision=$(REVISION) -extldflags '-static'" -tags timetzdata,plugins -o ../../$(@:cmd/%=%) ; cd ../..

.PHONY: linux
linux: ${BINARIES_LINUX}

.PHONY: ${BINARIES_LINUX}
${BINARIES_LINUX}:
	mkdir -p release
ifeq ($(MULTIARCH),true)
	cd cmd/$(@:cmd/%-linux=%) ; CGO_ENABLED=0 go build -ldflags "-X main.GitRevision=$(REVISION) -extldflags '-static'" -tags timetzdata,plugins -o ../../release/$(@:cmd/%-linux=%)-$(REVISION)-linux ; cd ../..
else
	cd cmd/$(@:cmd/%-linux=%) ; CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X main.GitRevision=$(REVISION) -extldflags '-static'" -tags timetzdata,plugins -o ../../release/$(@:cmd/%-linux=%)-$(REVISION)-linux ; cd ../..
endif

.PHONY: rasp
rasp: ${BINARIES_RASP}

.PHONY: ${BINARIES_RASP}
${BINARIES_RASP}:
	mkdir -p release
	cd cmd/$(@:cmd/%-rasp=%) ; CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "-X main.GitRevision=$(REVISION) -extldflags '-static'" -tags timetzdata,plugins -o ../../release/$(@:cmd/%-rasp=%)-$(REVISION)-rasp ; cd ../..

.PHONY: darwin
darwin: ${BINARIES_DARWIN}

.PHONY: ${BINARIES_DARWIN}
${BINARIES_DARWIN}:
	mkdir -p release
	cd cmd/$(@:cmd/%-darwin=%)  ; CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.GitRevision=$(REVISION) -extldflags '-static'" -tags timetzdata,plugins	 -o ../../release/$(@:cmd/%-darwin=%)-$(REVISION)-darwin ; cd ../..

.PHONY: clean
clean:
	rm -rf release/*
