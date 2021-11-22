# Use goreman to run `go get github.com/mattn/goreman`

test1: ./raft --id 1 --cluster 127.0.0.1:8081,127.0.0.1:8082 --port :8080
test2: ./raft --id 2 --cluster 127.0.0.1:8080,127.0.0.1:8082 --port :8081
test3: ./raft --id 3 --cluster 127.0.0.1:8080,127.0.0.1:8081 --port :8082