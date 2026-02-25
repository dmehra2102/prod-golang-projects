package tlsconfig

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"google.golang.org/grpc/credentials"
)

// ServerTLSConfig returns TLS credentials for the server.
// certFile/keyFile: server certificate and private key.
// caFile: CA certificate. If non-empty, enables mTLS — the server will
// require clients to present a certificate signed by this CA.
func ServerTLSConfig(certFile, keyFile, caFile string) (credentials.TransportCredentials, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load server key pair: %w", err)
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	if caFile != "" {
		// mTLS: require and verify client certificate
		caPEM, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
		}
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
		tlsCfg.ClientCAs = caPool
	}

	return credentials.NewTLS(tlsCfg), nil
}

// ClientTLSConfig returns TLS credentials for the client.
// caFile: CA certificate to verify the server against.
// certFile/keyFile: client certificate for mTLS (leave empty for one-way TLS).
func ClientTLSConfig(caFile, certFile, keyFile string) (credentials.TransportCredentials, error) {
	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read CA file: %w", err)
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	tlsCfg := &tls.Config{
		RootCAs:    caPool,
		MinVersion: tls.VersionTLS12,
	}

	if certFile != "" && keyFile != "" {
		// mTLS: attach client certificate
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("load client key pair: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	return credentials.NewTLS(tlsCfg), nil
}