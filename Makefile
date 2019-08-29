GOPATH=`pwd`/backend

build:
	@echo "Building Alertstrap..."
	@GOPATH=${GOPATH} go build -o backend/bin/alertstrap backend/alertstrap.go
	@echo "Building Jiramanager..."
	@GOPATH=${GOPATH} go build -o backend/bin/jiramanager backend/jiramanager.go

run:
	@echo "Runing Alertstrap..."
	@backend/bin/alertstrap -config backend/conf/config.toml -dbase backend/conf/dbase.sql
