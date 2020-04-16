GO111MODULE=on
export GO111MODULE

APP=scheduler
IMAGE=automatedhome/$(APP)

.PHONY: build
build: $(APP)

$(APP):
	go build -o $(APP) cmd/main.go

qemu-arm-static:
	./hooks/post_checkout

.PHONY: image
image: qemu-arm-static
	./hooks/pre_build
	docker build -t $(IMAGE) .

.PHONY: push
push: image
	docker push $(IMAGE)
