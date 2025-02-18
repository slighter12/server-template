# server-template


## TODO

- [ ] ~Evaluate the adoption of go-zero framework~
- [ ] Implement load balancing for gRPC
- [ ] Implement etcd for gRPC

## Test HTTP/3 with Docker
```
docker run -ti --rm alpine/curl-http3 curl --http3 -v -k https://host.docker.internal:4433/protocol
```
