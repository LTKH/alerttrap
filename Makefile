GOPATH=`pwd`/backend

build:
	@echo "Building Alertstrap..."
	@GOPATH=${GOPATH} go build -o backend/bin/alertstrap backend/alertstrap.go
	@echo "Building Alertsender..."
	@GOPATH=${GOPATH} go build -o backend/bin/alertsender backend/alertsender.go
