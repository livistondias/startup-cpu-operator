.PHONY: build
build:
	go build -o bin/manager cmd/main.go

.PHONY: run
run:
	go run cmd/main.go

.PHONY: docker-build
docker-build:
	docker build -t startup-cpu-operator:latest .

.PHONY: deploy
deploy:
	kubectl apply -f config/crd/bases/
	kubectl apply -f config/rbac/
	kubectl apply -f config/manager/

.PHONY: install-crd
install-crd:
	kubectl apply -f config/crd/bases/startupcpuboosts.yaml

.PHONY: uninstall-crd
uninstall-crd:
	kubectl delete -f config/crd/bases/startupcpuboosts.yaml
