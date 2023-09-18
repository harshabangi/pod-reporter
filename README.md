# Pod Status API Read Me

This Go application provides an HTTP API for fetching Kubernetes pod status within a cluster. It uses Echo for the API
and the requests package for HTTP requests to pods.

## Endpoints

1. **Get Pod Status by Name**:
    - URL: `/v1/namespaces/:namespace/pods/:pod_name/status`
    - Retrieve pod status by specifying the namespace and pod name.

2. **Get Pod Status by Labels**:
    - URL: `/v1/namespaces/:namespace/pod_status?label=label1=value1&label=label2=value2...`
    - Fetch pod status using label selectors; returns status for the first matching pod.

## Prerequisites

- Running Kubernetes cluster.

## Usage

1. **Build**: Use `go build` to build the application.

2. **Run**: Execute the binary to start the HTTP server (default port: 8080).

3. **Access API**: Fetch pod status via the specified endpoints, setting the desired response format using the `Accept`
   header.

## Dependencies

Key dependencies:

- Echo (for API)
- requests (for HTTP)
- k8s.io/client-go/kubernetes (Kubernetes client)
- k8s.io/client-go/rest (Kubernetes client config)

## Error Handling

The API handles errors gracefully, returning appropriate HTTP status codes and error messages for invalid input, pod not
found, and more.

## Configuration

Designed for Kubernetes clusters with automatic in-cluster configuration detection.
