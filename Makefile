CONTAINER_IMAGE ?= "rochacon/bastrd"
CONTAINER_IMAGE_TOOLBOX ?= "rochacon/bastrd-toolbox"
VERSION ?= $$(git describe --abbrev=10 --always --dirty --tags)

default: binary

all: test binary image toolbox publish_binary publish_image publish_toolbox

binary:
	go build -v -ldflags "-X main.VERSION=$(VERSION)" -tags "netgo osusergo"

image:
	docker build -t $(CONTAINER_IMAGE):$(VERSION) .

publish: publish_binary publish_image publish_image_toolbox

publish_binary:
	gzip -f bastrd
	aws s3 cp ./bastrd.gz s3://bastrd-dev/bastrd.gz --acl public-read

publish_image:
	docker push $(CONTAINER_IMAGE):$(VERSION)

publish_toolbox:
	docker push $(CONTAINER_IMAGE_TOOLBOX):$(VERSION)

test:
	go test $(ARGS) ./...

toolbox:
	docker build -f Dockerfile.toolbox -t $(CONTAINER_IMAGE_TOOLBOX):$(VERSION) .

.PHONY: binary image publish publish_binary publish_image publish_toolbox test toolbox
