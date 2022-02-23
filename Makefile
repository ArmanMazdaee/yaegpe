export DOCKER_BUILDKIT = 1

build: 
	@docker build \
		--target build \
		--output . .

test: 
	@docker build \
		--target test --output . .
	@cat test-results.txt
	@rm test-results.txt

build-image:
	@docker build \
		--tag yaegpe:latest .

start: build-image
	@docker run \
		--rm \
		--publish 8080:8080 \
		--detach \
		--name yaegpe-server \
		yaegpe:latest

logs:
	@docker logs yaegpe-server

stop:
	@docker stop yaegpe-server