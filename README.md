# Drud CLI Metrics Microservice

Runs a service to collect logging information from drud cli runs.

## Deployment (in kubernetes directory)
```
kubectl create -f deployments/drud_cli_metrics.yml
kubectl create -f services/drud_cli_metrics.yml
```

## Building the container image (in app/drud_cli_metrics)

The container must be built on linux (GCE) because the sqlite3 library cannot cross-compile.

```
TAG=0.0.5
OWNER=randyfay
go build --tags netgo --ldflags '-extldflags "-lm -lstdc++ -static"'

docker build -t "drud_cli_metrics:$TAG" .
docker tag "drud_cli_metrics:$TAG" "$OWNER/drud_cli_metrics:$TAG"
docker push "$OWNER/drud_cli_metrics:$TAG"
```

## Miscellaneous

```
kubectl get pods
kubectl exec <pod_name> --stdin --tty /bin/sh
kubectl describe pods
kubectl logs -f <pod_name>  

```

## JSON requests
```
GET http://<cluster_ip>.:30001/v1.0/logitems
curl -X GET -H "Cache-Control: no-cache" "http://192.168.99.100.:30001/v1.0/logitem"


POST http://<cluster_ip>:30001/v1.0/logitems

Body: {"result_code":403, "machine_id":"2301", "info":"nonoe", "client_timestamp": 939393}
curl -X POST -H "Content-Type: application/json" -H "Cache-Control: no-cache"  -d '{"result_code":403, "machine_id":"2301", "info":"nonoe", "client_timestamp": 939393}' "http://192.168.99.100:30001/v1.0/logitem"
```
