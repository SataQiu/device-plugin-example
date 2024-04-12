IMAGE_NAME=shidaqiu/device-plugin-example:v0.1

build:
	docker build -t $(IMAGE_NAME) .

push:
	docker push $(IMAGE_NAME)
