GOPATH=`pwd`

build:
    @echo "Building Alertstrap..."
    @GOPATH=${GOPATH} go build -o bin/alertstrap alertstrap.go

run:
    @echo "Runing Alertstrap..."
    @go run alertstrap.go -config configs/config.toml
