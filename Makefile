.PHONY: deps vet test build clean

PACKAGES = $(shell glide novendor)
DOCKER_REPO_URL = jack08300/swing-push-worker

deps:
	dep ensure

vet:
	go vet $(PACKAGES)

build: clean
	GOOS=linux go build -o ./build/main *.go

clean:
	rm -rf build/*
	find . -name '*.test' -delete

push-image:
	docker tag swing-push-worker $(DOCKER_REPO_URL):latest
	docker push $(DOCKER_REPO_URL):latest

build-image:
	docker build --rm -t swing-push-worker:latest .