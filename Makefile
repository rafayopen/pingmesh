# docker
DOCKER ?= docker
export DOCKER

CWD := $(shell basename ${PWD})
# Docker image name, based on current working directory
IMAGE := ${CWD}
# Version (tag used with docker push)
VERSION := `git describe --tags --long`

# Linux build image name (does not conflict with go build)
LINUX_EXE := ${IMAGE}.exe
# List of docker images
IMAGE_LIST := ${IMAGE}-images.out

.PHONY: info all
info:
	@-echo Use \"make standalone\" to build local binary cmd/${IMAGE}/${IMAGE}
	@-echo Use \"make docker\" to build ${IMAGE}:${VERSION} from ${LINUX_EXE}
	@-echo Use \"make all\" to build both
# "make push" will push it to DockerHub, using credentials in your env

all: standalone docker

##
# Supply default options to docker build
#$
define docker-build
$(DOCKER) build --rm -q
endef

##
# build the standalone pingmesh application
##
cmd/${IMAGE}/${IMAGE}: cmd/*/*.go pkg/*/*.go check-version
	cd cmd/${IMAGE} && go build -v && go test -v && go vet

# build the avgping client 
# (this does not build a docker or Linux exe, there is no point)
cmd/avgping/avgping: cmd/*/*.go pkg/*/*.go
	cd cmd/avgping && go build -v && go test -v && go vet

.PHONY: check-version update-version
check-version: cmd/${IMAGE}/version.go
	@-sh -c "grep ${VERSION} $? >/dev/null 2>/dev/null" || $(MAKE) update-version
update-version:
	@-echo "package main\n// auto-generated, do not edit\nvar buildVersion = \"${VERSION}\"" > cmd/pingmesh/version.go

cmd/${IMAGE}/version.go:
	$(MAKE) update-version

.PHONY: standalone install
standalone:	cmd/${IMAGE}/${IMAGE} cmd/avgping/avgping
install:	cmd/*/*.go pkg/*/*.go
	cd cmd/${IMAGE} && go install -v
	cd cmd/avgping && go install -v

.PHONY: build docker full 
build docker:	${IMAGE_LIST}
${IMAGE_LIST}:	cmd/${IMAGE}/${LINUX_EXE} Dockerfile Makefile
	$(docker-build) -t ${IMAGE} .
	$(DOCKER) tag ${IMAGE} "${IMAGE}:${VERSION}" # tag local image name with version
	$(DOCKER) tag ${IMAGE} "${IMAGE}:latest" # tag local image name with version
	$(DOCKER) images | egrep "${IMAGE} " > ${IMAGE_LIST}
	@-test -s ${IMAGE_LIST} || rm -f ${IMAGE_LIST}

full:	clean docker run

cmd/${IMAGE}/${LINUX_EXE}:	cmd/*/*.go pkg/*/*.go check-version
	cd cmd/${IMAGE} && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ${LINUX_EXE} .

.PHONY: run push
run:	${IMAGE_LIST}
	$(DOCKER) run --rm -it -e PINGMESH_URL="https://www.google.com" -e REP_CITY="Sunnyvale" -e REP_COUNTRY="US" -e AWS_REGION="us-west-2" -e AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID}" -e AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY}" ${IMAGE} -n=5 -d=2

push:	${IMAGE_LIST}
	$(DOCKER) tag ${IMAGE} "${DOCKER_USER}/${IMAGE}:${VERSION}"
	$(DOCKER) push ${DOCKER_USER}/${IMAGE}

.PHONY: clean
clean:
	-rm -rf ${IMAGE_LIST} ${IMAGE} ${LINUX_EXE} cmd/${IMAGE}/${IMAGE} cmd/${IMAGE}/${LINUX_EXE} cmd/avgping/avgping cmd/${IMAGE}/version.go
	-$(DOCKER) rmi ${IMAGE}:${VERSION}

CLI = rafay-cli
NOW = $(shell date +%Y%m%d%H%M)
rafay-push:	${IMAGE_LIST}
	$(DOCKER) tag ${IMAGE} ${IMAGE}:${NOW} # tag local image name with timestamp
	$(CLI) image upload ${IMAGE}:${NOW}
	$(DOCKER) images | egrep "${IMAGE} " > ${IMAGE_LIST} # update with latest tag

test:	cmd/${IMAGE}/${IMAGE} test_pingmesh.sh
	./test_pingmesh.sh
