.PHONY: build
build:
	go build -o bin/manager cmd/main.go

.PHONY: run
run:
	go run cmd/main.go

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: test
test:
	go test ./... -v -count=1

.PHONY: generate
generate:
	controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."
	controller-gen rbac:roleName=startup-cpu-operator-role crd paths="./..." output:crd:dir=config/crd/bases output:rbac:dir=config/rbac

.PHONY: docker-build
docker-build:
	docker build -t startup-cpu-operator:latest .

.PHONY: docker-build-multiarch
docker-build-multiarch:
	docker buildx build --platform linux/amd64,linux/arm64 -t startup-cpu-operator:latest .

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
