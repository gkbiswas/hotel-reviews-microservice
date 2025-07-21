# Istio Service Mesh Setup for Hotel Reviews Platform

## Overview

This document outlines the setup and configuration of Istio service mesh for the Hotel Reviews microservices platform. Istio provides advanced traffic management, security, and observability features for our distributed architecture.

## Prerequisites

- Kubernetes cluster (1.24+)
- kubectl configured
- Helm 3.x
- Istio CLI (istioctl)

## Installation Guide

### 1. Install Istio

```bash
# Download and install Istio
curl -L https://istio.io/downloadIstio | sh -
cd istio-*
export PATH=$PWD/bin:$PATH

# Install Istio with demo profile (adjust for production)
istioctl install --set values.defaultRevision=default

# Enable sidecar injection for hotel-reviews namespaces
kubectl label namespace hotel-reviews-staging istio-injection=enabled
kubectl label namespace hotel-reviews-production istio-injection=enabled

# Install Istio addons (Kiali, Prometheus, Grafana, Jaeger)
kubectl apply -f samples/addons/
```

### 2. Verify Installation

```bash
# Check Istio components
kubectl get pods -n istio-system

# Verify sidecar injection
kubectl get namespace -L istio-injection
```

## Service Mesh Configuration

### Gateway Configuration

```yaml
# istio-gateway.yaml
apiVersion: networking.istio.io/v1alpha3
kind: Gateway
metadata:
  name: hotel-reviews-gateway
  namespace: hotel-reviews-production
spec:
  selector:
    istio: ingressgateway
  servers:
  - port:
      number: 80
      name: http
      protocol: HTTP
    hosts:
    - api.hotel-reviews.com
    tls:
      httpsRedirect: true
  - port:
      number: 443
      name: https
      protocol: HTTPS
    tls:
      mode: SIMPLE
      credentialName: hotel-reviews-tls
    hosts:
    - api.hotel-reviews.com
---
apiVersion: networking.istio.io/v1alpha3
kind: Gateway
metadata:
  name: hotel-reviews-staging-gateway
  namespace: hotel-reviews-staging
spec:
  selector:
    istio: ingressgateway
  servers:
  - port:
      number: 80
      name: http
      protocol: HTTP
    hosts:
    - staging-api.hotel-reviews.com
    tls:
      httpsRedirect: true
  - port:
      number: 443
      name: https
      protocol: HTTPS
    tls:
      mode: SIMPLE
      credentialName: hotel-reviews-staging-tls
    hosts:
    - staging-api.hotel-reviews.com
```

### Virtual Services for Traffic Management

```yaml
# user-service-virtualservice.yaml
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: user-service
  namespace: hotel-reviews-production
spec:
  hosts:
  - user-service
  http:
  - match:
    - headers:
        canary:
          exact: "true"
    route:
    - destination:
        host: user-service
        subset: canary
      weight: 100
  - route:
    - destination:
        host: user-service
        subset: stable
      weight: 100
  - fault:
      delay:
        percentage:
          value: 0.1
        fixedDelay: 100ms
  - timeout: 10s
  - retries:
      attempts: 3
      perTryTimeout: 3s
---
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: hotel-service
  namespace: hotel-reviews-production
spec:
  hosts:
  - hotel-service
  http:
  - match:
    - uri:
        prefix: "/api/v1/hotels/search"
    route:
    - destination:
        host: search-service
        port:
          number: 8080
  - route:
    - destination:
        host: hotel-service
        subset: stable
      weight: 90
    - destination:
        host: hotel-service
        subset: canary
      weight: 10
  - timeout: 30s
  - retries:
      attempts: 3
      perTryTimeout: 10s
---
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: review-service
  namespace: hotel-reviews-production
spec:
  hosts:
  - review-service
  http:
  - match:
    - headers:
        user-type:
          exact: "premium"
    route:
    - destination:
        host: review-service
        subset: premium
  - route:
    - destination:
        host: review-service
        subset: standard
  - fault:
      abort:
        percentage:
          value: 0.1
        httpStatus: 503
  - timeout: 15s
```

### Destination Rules for Load Balancing

```yaml
# destination-rules.yaml
apiVersion: networking.istio.io/v1alpha3
kind: DestinationRule
metadata:
  name: user-service
  namespace: hotel-reviews-production
spec:
  host: user-service
  trafficPolicy:
    loadBalancer:
      simple: LEAST_CONN
    connectionPool:
      tcp:
        maxConnections: 100
      http:
        http1MaxPendingRequests: 50
        maxRequestsPerConnection: 10
    circuitBreaker:
      consecutiveErrors: 5
      interval: 30s
      baseEjectionTime: 30s
      maxEjectionPercent: 50
  subsets:
  - name: stable
    labels:
      version: stable
    trafficPolicy:
      loadBalancer:
        simple: ROUND_ROBIN
  - name: canary
    labels:
      version: canary
    trafficPolicy:
      loadBalancer:
        simple: RANDOM
---
apiVersion: networking.istio.io/v1alpha3
kind: DestinationRule
metadata:
  name: hotel-service
  namespace: hotel-reviews-production
spec:
  host: hotel-service
  trafficPolicy:
    loadBalancer:
      simple: ROUND_ROBIN
    connectionPool:
      tcp:
        maxConnections: 200
      http:
        http1MaxPendingRequests: 100
        maxRequestsPerConnection: 20
    circuitBreaker:
      consecutiveErrors: 10
      interval: 30s
      baseEjectionTime: 30s
      maxEjectionPercent: 30
  subsets:
  - name: stable
    labels:
      version: stable
  - name: canary
    labels:
      version: canary
---
apiVersion: networking.istio.io/v1alpha3
kind: DestinationRule
metadata:
  name: review-service
  namespace: hotel-reviews-production
spec:
  host: review-service
  trafficPolicy:
    loadBalancer:
      simple: LEAST_CONN
    connectionPool:
      tcp:
        maxConnections: 150
      http:
        http1MaxPendingRequests: 75
        maxRequestsPerConnection: 15
  subsets:
  - name: standard
    labels:
      tier: standard
  - name: premium
    labels:
      tier: premium
    trafficPolicy:
      loadBalancer:
        simple: ROUND_ROBIN
      connectionPool:
        tcp:
          maxConnections: 300
```

## Security Configuration

### Authentication and Authorization

```yaml
# authentication-policy.yaml
apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata:
  name: default
  namespace: hotel-reviews-production
spec:
  mtls:
    mode: STRICT
---
apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata:
  name: user-service-permissive
  namespace: hotel-reviews-production
spec:
  selector:
    matchLabels:
      app: user-service
  mtls:
    mode: PERMISSIVE
---
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
  name: hotel-service-authz
  namespace: hotel-reviews-production
spec:
  selector:
    matchLabels:
      app: hotel-service
  rules:
  - from:
    - source:
        principals: ["cluster.local/ns/hotel-reviews-production/sa/gateway"]
    - source:
        principals: ["cluster.local/ns/hotel-reviews-production/sa/review-service"]
  - to:
    - operation:
        methods: ["GET", "POST"]
        paths: ["/api/v1/hotels/*"]
  - when:
    - key: request.headers[authorization]
      values: ["Bearer *"]
---
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
  name: review-service-authz
  namespace: hotel-reviews-production
spec:
  selector:
    matchLabels:
      app: review-service
  rules:
  - from:
    - source:
        principals: ["cluster.local/ns/hotel-reviews-production/sa/user-service"]
  - to:
    - operation:
        methods: ["GET", "POST", "PUT", "DELETE"]
        paths: ["/api/v1/reviews/*"]
  - when:
    - key: request.headers[user-id]
      notValues: [""]
```

### JWT Authentication

```yaml
# jwt-authentication.yaml
apiVersion: security.istio.io/v1beta1
kind: RequestAuthentication
metadata:
  name: jwt-auth
  namespace: hotel-reviews-production
spec:
  selector:
    matchLabels:
      app: api-gateway
  jwtRules:
  - issuer: "https://auth.hotel-reviews.com"
    jwksUri: "https://auth.hotel-reviews.com/.well-known/jwks.json"
    audiences:
    - "hotel-reviews-api"
    outputPayloadToHeader: "x-jwt-payload"
---
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
  name: require-jwt
  namespace: hotel-reviews-production
spec:
  selector:
    matchLabels:
      app: api-gateway
  rules:
  - from:
    - source:
        requestPrincipals: ["*"]
  - when:
    - key: request.headers[authorization]
      values: ["Bearer *"]
```

## Traffic Management Patterns

### Blue-Green Deployment

```yaml
# blue-green-deployment.yaml
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: hotel-service-blue-green
  namespace: hotel-reviews-production
spec:
  hosts:
  - hotel-service
  http:
  - match:
    - headers:
        deployment:
          exact: "green"
    route:
    - destination:
        host: hotel-service
        subset: green
      weight: 100
  - route:
    - destination:
        host: hotel-service
        subset: blue
      weight: 100
---
apiVersion: networking.istio.io/v1alpha3
kind: DestinationRule
metadata:
  name: hotel-service-blue-green
  namespace: hotel-reviews-production
spec:
  host: hotel-service
  subsets:
  - name: blue
    labels:
      version: blue
  - name: green
    labels:
      version: green
```

### Canary Deployment with Traffic Splitting

```yaml
# canary-deployment.yaml
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: review-service-canary
  namespace: hotel-reviews-production
spec:
  hosts:
  - review-service
  http:
  - match:
    - headers:
        canary-user:
          exact: "true"
    route:
    - destination:
        host: review-service
        subset: canary
      weight: 100
  - route:
    - destination:
        host: review-service
        subset: stable
      weight: 95
    - destination:
        host: review-service
        subset: canary
      weight: 5
```

### Circuit Breaker Configuration

```yaml
# circuit-breaker.yaml
apiVersion: networking.istio.io/v1alpha3
kind: DestinationRule
metadata:
  name: analytics-service-circuit-breaker
  namespace: hotel-reviews-production
spec:
  host: analytics-service
  trafficPolicy:
    connectionPool:
      tcp:
        maxConnections: 50
      http:
        http1MaxPendingRequests: 25
        maxRequestsPerConnection: 5
        consecutiveGatewayErrors: 5
        interval: 30s
        baseEjectionTime: 30s
        maxEjectionPercent: 50
    circuitBreaker:
      consecutiveErrors: 5
      interval: 30s
      baseEjectionTime: 30s
      maxEjectionPercent: 50
```

## Observability Configuration

### Distributed Tracing

```yaml
# tracing-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: istio-tracing
  namespace: istio-system
data:
  mesh: |
    defaultConfig:
      tracing:
        sampling: 1.0
        max_path_tag_length: 256
        custom_tags:
          request_id:
            header:
              name: x-request-id
          user_id:
            header:
              name: x-user-id
          hotel_id:
            header:
              name: x-hotel-id
      proxyStatsMatcher:
        inclusionRegexps:
        - ".*circuit_breakers.*"
        - ".*upstream_rq_retry.*"
        - ".*upstream_rq_pending.*"
```

### Service Monitoring

```yaml
# service-monitor.yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: istio-proxy-metrics
  namespace: hotel-reviews-production
spec:
  selector:
    matchLabels:
      app: hotel-reviews
  endpoints:
  - port: http-monitoring
    path: /stats/prometheus
    interval: 30s
```

### Telemetry v2 Configuration

```yaml
# telemetry-v2.yaml
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: istio-control-plane
spec:
  values:
    telemetry:
      v2:
        enabled: true
        prometheus:
          configOverride:
            metric_relabeling_configs:
            - source_labels: [__name__]
              regex: 'istio_request_duration_milliseconds'
              target_label: __name__
              replacement: 'istio_request_duration_seconds'
            - source_labels: [__name__]
              regex: 'istio_request_duration_seconds'
              target_label: __tmp_duration
              replacement: '${1}'
            - source_labels: [__tmp_duration]
              regex: '(.+)'
              target_label: __name__
              replacement: 'istio_request_duration_seconds'
```

## Rate Limiting

```yaml
# rate-limiting.yaml
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: rate-limit-filter
  namespace: hotel-reviews-production
spec:
  workloadSelector:
    labels:
      app: api-gateway
  configPatches:
  - applyTo: HTTP_FILTER
    match:
      context: SIDECAR_INBOUND
      listener:
        filterChain:
          filter:
            name: "envoy.filters.network.http_connection_manager"
    patch:
      operation: INSERT_BEFORE
      value:
        name: envoy.filters.http.local_ratelimit
        typed_config:
          "@type": type.googleapis.com/udpa.type.v1.TypedStruct
          type_url: type.googleapis.com/envoy.extensions.filters.http.local_ratelimit.v3.LocalRateLimit
          value:
            stat_prefix: rate_limiter
            token_bucket:
              max_tokens: 1000
              tokens_per_fill: 100
              fill_interval: 60s
            filter_enabled:
              default_value:
                numerator: 100
                denominator: HUNDRED
            filter_enforced:
              default_value:
                numerator: 100
                denominator: HUNDRED
```

## Fault Injection for Testing

```yaml
# fault-injection.yaml
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: fault-injection-test
  namespace: hotel-reviews-staging
spec:
  hosts:
  - hotel-service
  http:
  - match:
    - headers:
        fault-test:
          exact: "delay"
    fault:
      delay:
        percentage:
          value: 50
        fixedDelay: 2s
    route:
    - destination:
        host: hotel-service
  - match:
    - headers:
        fault-test:
          exact: "abort"
    fault:
      abort:
        percentage:
          value: 10
        httpStatus: 503
    route:
    - destination:
        host: hotel-service
  - route:
    - destination:
        host: hotel-service
```

## Multi-Cluster Service Mesh

```yaml
# multi-cluster-setup.yaml
apiVersion: networking.istio.io/v1alpha3
kind: Gateway
metadata:
  name: cross-network-gateway
  namespace: istio-system
spec:
  selector:
    istio: eastwestgateway
  servers:
  - port:
      number: 15443
      name: tls
      protocol: TLS
    tls:
      mode: ISTIO_MUTUAL
    hosts:
    - "*.local"
---
apiVersion: networking.istio.io/v1alpha3
kind: ServiceEntry
metadata:
  name: remote-hotel-service
  namespace: hotel-reviews-production
spec:
  hosts:
  - hotel-service.hotel-reviews-production.global
  location: MESH_EXTERNAL
  ports:
  - number: 8080
    name: http
    protocol: HTTP
  resolution: DNS
  addresses:
  - 172.16.0.1  # Remote cluster service IP
  endpoints:
  - address: hotel-service.hotel-reviews-production.svc.cluster.local
    network: remote-network
    ports:
      http: 8080
```

## Deployment Scripts

```bash
#!/bin/bash
# deploy-service-mesh.sh

set -e

echo "Deploying Istio Service Mesh configuration..."

# Apply gateway configurations
kubectl apply -f istio-gateway.yaml

# Apply virtual services
kubectl apply -f user-service-virtualservice.yaml

# Apply destination rules
kubectl apply -f destination-rules.yaml

# Apply security policies
kubectl apply -f authentication-policy.yaml
kubectl apply -f jwt-authentication.yaml

# Apply observability configurations
kubectl apply -f tracing-config.yaml
kubectl apply -f service-monitor.yaml

# Apply rate limiting
kubectl apply -f rate-limiting.yaml

echo "Service mesh configuration deployed successfully!"

# Verify deployment
echo "Verifying service mesh status..."
kubectl get gateways -n hotel-reviews-production
kubectl get virtualservices -n hotel-reviews-production
kubectl get destinationrules -n hotel-reviews-production
kubectl get peerauthentication -n hotel-reviews-production
kubectl get authorizationpolicies -n hotel-reviews-production

echo "Service mesh deployment complete!"
```

## Monitoring and Troubleshooting

### Key Metrics to Monitor

```yaml
# istio-metrics-dashboard.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: istio-metrics-dashboard
  namespace: istio-system
data:
  dashboard.json: |
    {
      "dashboard": {
        "title": "Istio Service Mesh Metrics",
        "panels": [
          {
            "title": "Request Rate",
            "targets": [
              {
                "expr": "sum(rate(istio_requests_total[5m])) by (source_service_name, destination_service_name)"
              }
            ]
          },
          {
            "title": "Success Rate",
            "targets": [
              {
                "expr": "sum(rate(istio_requests_total{response_code!~\"5.*\"}[5m])) / sum(rate(istio_requests_total[5m]))"
              }
            ]
          },
          {
            "title": "P99 Latency",
            "targets": [
              {
                "expr": "histogram_quantile(0.99, sum(rate(istio_request_duration_milliseconds_bucket[5m])) by (le, source_service_name, destination_service_name))"
              }
            ]
          }
        ]
      }
    }
```

### Troubleshooting Commands

```bash
# Check Istio proxy configuration
istioctl proxy-config cluster <pod-name> -n hotel-reviews-production

# Verify virtual service configuration
istioctl proxy-config route <pod-name> -n hotel-reviews-production

# Check authentication policies
istioctl authn tls-check <pod-name>.<namespace>.svc.cluster.local

# Debug traffic routing
istioctl analyze -n hotel-reviews-production

# Check proxy logs
kubectl logs <pod-name> -c istio-proxy -n hotel-reviews-production

# Verify service mesh configuration
istioctl validate -f istio-gateway.yaml
```

## Performance Optimization

### Envoy Configuration Tuning

```yaml
# envoy-optimization.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: envoy-config-optimization
  namespace: istio-system
data:
  custom_bootstrap.json: |
    {
      "stats_sinks": [
        {
          "name": "envoy.stat_sinks.statsd",
          "typed_config": {
            "@type": "type.googleapis.com/envoy.extensions.stat_sinks.statsd.v3.StatsdSink",
            "address": {
              "socket_address": {
                "address": "127.0.0.1",
                "port_value": 9125
              }
            }
          }
        }
      ],
      "stats_config": {
        "stats_tags": [
          {
            "tag_name": "service_name",
            "regex": "^cluster\\.((.+?)\\.).*",
            "fixed_value": "\\1"
          }
        ]
      }
    }
```

This comprehensive Istio service mesh setup provides advanced traffic management, security, and observability for the Hotel Reviews microservices platform.