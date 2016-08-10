GOBIN=$(GOPATH)/bin
APP_DIR_LIST=$(shell go list ./... | grep -v /vendor/)
APP_NAME=tapng-monitor

build: verify_gopath
	CGO_ENABLED=0 go install -tags netgo $(APP_DIR_LIST)
	go fmt $(APP_DIR_LIST)

verify_gopath:
	@if [ -z "$(GOPATH)" ] || [ "$(GOPATH)" = "" ]; then\
		echo "GOPATH not set. You need to set GOPATH before run this command";\
		exit 1 ;\
	fi

docker_build: build_anywhere
	docker build -t $(APP_NAME) .

push_docker: docker_build
	docker tag tapng-container-broker $(REPOSITORY_URL)/$(APP_NAME):latest
	docker push $(REPOSITORY_URL)/$(APP_NAME):latest

kubernetes_deploy: docker_build
	kubectl create -f configmap.yaml
	kubectl create -f deployment.yaml

kubernetes_update: docker_build
	kubectl delete -f deployment.yaml
	kubectl create -f deployment.yaml

deps_fetch_specific: bin/govendor
	@if [ "$(DEP_URL)" = "" ]; then\
		echo "DEP_URL not set. Run this comand as follow:";\
		echo " make deps_fetch_specific DEP_URL=github.com/nu7hatch/gouuid";\
	exit 1 ;\
	fi
	@echo "Fetching specific dependency in newest versions"
	$(GOBIN)/govendor fetch -v $(DEP_URL)

deps_update_tapng: verify_gopath
	$(GOBIN)/govendor update github.com/trustedanalytics/...
	rm -Rf vendor/github.com/trustedanalytics/$(APP_NAME)
	@echo "Done"

prepare_dirs:
	mkdir -p ./temp/src/github.com/trustedanalytics/$(APP_NAME)
	$(eval REPOFILES=$(shell pwd)/*)
	ln -sf $(REPOFILES) temp/src/github.com/trustedanalytics/$(APP_NAME)

build_anywhere: prepare_dirs
	$(eval GOPATH=$(shell cd ./temp; pwd))
	$(eval APP_DIR_LIST=$(shell GOPATH=$(GOPATH) go list ./temp/src/github.com/trustedanalytics/$(APP_NAME)/... | grep -v /vendor/))
	GOPATH=$(GOPATH) CGO_ENABLED=0 go build -tags netgo $(APP_DIR_LIST)
	rm -Rf application && mkdir application
	cp ./$(APP_NAME) ./application/$(APP_NAME)
	rm -Rf ./temp
