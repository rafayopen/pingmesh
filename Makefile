# docker
DOCKER ?= docker
export DOCKER

CWD := $(shell basename ${PWD})
# Docker image name, based on current working directory
IMAGE := ${CWD}
# Version (tag used with docker push)
VERSION := `git tag | tail -1`

# Linux build image name (does not conflict with go build)
LINUX_EXE := ${IMAGE}.exe
# List of docker images
IMAGE_LIST := ${IMAGE}-images.out

info:
	echo VERSION ${VERSION}
	@-echo Use \"make standalone\" to build local binary cmd/${IMAGE}/${IMAGE}
	@-echo Use \"make docker\" to build ${IMAGE}:${VERSION} from ${LINUX_EXE}
# "make push" will push it to DockerHub, using credentials in your env

##
# Supply default options to docker build
#$
define docker-build
$(DOCKER) build --rm -q
endef

##
# build the standalone pingmesh application
##
.PHONY: standalone install
cmd/${IMAGE}/${IMAGE}: cmd/*/*.go pkg/*/*.go
	cd cmd/${IMAGE} && go build -v && go test -v && go vet

standalone:	cmd/${IMAGE}/${IMAGE}
install:	cmd/${IMAGE}/${IMAGE}
	cd cmd/${IMAGE} && go install -v

.PHONY: build docker full 
build docker:	${IMAGE_LIST}
${IMAGE_LIST}:	cmd/${IMAGE}/${LINUX_EXE} Dockerfile Makefile
	$(docker-build) -t ${IMAGE} .
	$(DOCKER) tag ${IMAGE} "${IMAGE}:${VERSION}" # tag local image name with version
	$(DOCKER) tag ${IMAGE} "${IMAGE}:latest" # tag local image name with version
	$(DOCKER) images | egrep "${IMAGE} " > ${IMAGE_LIST}
	@-test -s ${IMAGE_LIST} || rm -f ${IMAGE_LIST}

full:	clean docker run

cmd/${IMAGE}/${LINUX_EXE}:	cmd/*/*.go pkg/*/*.go
	cd cmd/${IMAGE} && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ${LINUX_EXE} .

.PHONY: run push
run:	${IMAGE_LIST}
	$(DOCKER) run --rm -it -e ${IMAGE}_URL="https://www.google.com" -e REP_CITY="Sunnyvale" -e REP_COUNTRY="US" -e AWS_REGION="us-west-2" -e AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID}" -e AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY}" ${IMAGE} -n=5 -d=2

push:	${IMAGE_LIST}
	$(DOCKER) tag ${IMAGE} "${DOCKER_USER}/${IMAGE}:${VERSION}"
	$(DOCKER) push ${DOCKER_USER}/${IMAGE}

.PHONY: clean
clean:
	-rm -rf ${IMAGE_LIST} ${IMAGE} ${LINUX_EXE} cmd/${IMAGE}/${IMAGE} cmd/${IMAGE}/${LINUX_EXE}
	-$(DOCKER) rmi ${IMAGE}:${VERSION}

CLI = rafay-cli
NOW = $(shell date +%Y%m%d%H%M)
rafay-push:	${IMAGE_LIST}
	$(DOCKER) tag ${IMAGE} ${IMAGE}:${NOW} # tag local image name with timestamp
	$(CLI) image upload ${IMAGE}:${NOW}
	$(DOCKER) images | egrep "${IMAGE} " > ${IMAGE_LIST} # update with latest tag

test:	cmd/${IMAGE}/${IMAGE} test_pingmesh.sh
	./test_pingmesh.sh
