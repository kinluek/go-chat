.PHONY: build run stop

# rebuilds the containers.
build:
	docker-compose build

# runs starts the application using the docker compose configuration.
run:
	docker-compose up

# stops the containers.
stop:
	docker-compose down --remove-orphans -v