# Drud CLI Metrics Microservice

Runs a service to collect logging information from drud cli runs.

## Deployment:
```
kubectl create -f deployments/drud_cli_metrics.yml
kubectl create -f services/drud_cli_metrics.yml
```

## Build Container

The container must be built on linux (GCE) because the sqlite3 library cannot cross-compile.

```
TAG=0.0.5
OWNER=randyfay
go build --tags netgo --ldflags '-extldflags "-lm -lstdc++ -static"'

docker build -t "drud_cli_metrics:$TAG" .
docker tag "drud_cli_metrics:$TAG" "$OWNER/drud_cli_metrics:$TAG"
docker push "$OWNER/drud_cli_metrics:$TAG"
```
