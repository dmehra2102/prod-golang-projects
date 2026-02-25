proto:
	protoc \
		--go_out=gen \
		--go_opt=paths=source_relative \
		--go-grpc_out=gen \
		--go-grpc_opt=paths=source_relative \
		-I proto \
		-I googleapis \
		proto/order/v1/order.proto

certs:
	mkdir -p certs
# 	# CA
	openssl genrsa -out certs/ca.key 4096
	openssl req -new -x509 -days 3650 -key certs/ca.key \
		-subj "/C=US/O=Dev CA/CN=dev-ca" \
		-out certs/ca.crt
# 	# Server
	openssl genrsa -out certs/server.key 4096
	openssl req -new -key certs/server.key \
		-subj "/C=US/O=OrderService/CN=localhost" \
		-out certs/server.csr
	openssl x509 -req -days 365 -in certs/server.csr \
		-CA certs/ca.crt -CAkey certs/ca.key -CAcreateserial \
		-extfile <(printf "subjectAltName=DNS:localhost,IP:127.0.0.1") \
		-out certs/server.crt
# 	# Client (for mTLS)
	openssl genrsa -out certs/client.key 4096
	openssl req -new -key certs/client.key \
		-subj "/C=US/O=OrderClient/CN=order-client" \
		-out certs/client.csr
	openssl x509 -req -days 365 -in certs/client.csr \
		-CA certs/ca.crt -CAkey certs/ca.key -CAcreateserial \
		-out certs/client.crt

test:
	go test ./... -v -race -count=1

run-server:
	go run ./cmd/server/main.go

run-client:
	go run ./cmd/client/main.go

.PHONY: proto certs test run-server run-client