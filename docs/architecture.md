# Hotel Reviews Microservice - Architecture Documentation

## Table of Contents
1. [System Overview](#system-overview)
2. [Clean Architecture Layers](#clean-architecture-layers)
3. [Component Architecture](#component-architecture)
4. [Data Flow Architecture](#data-flow-architecture)
5. [Deployment Architecture](#deployment-architecture)
6. [Integration Patterns](#integration-patterns)
7. [Technology Stack](#technology-stack)
8. [Security Architecture](#security-architecture)
9. [Performance Considerations](#performance-considerations)

## System Overview

The Hotel Reviews Microservice is a high-performance, scalable system built using Clean Architecture principles. It processes hotel review data from multiple providers (Booking.com, Expedia, etc.), stores it in a PostgreSQL database, and provides RESTful APIs for accessing and analyzing review data.

### Key Features
- **Multi-provider data ingestion** from S3 files
- **Real-time review processing** with validation and enrichment
- **Scalable batch processing** with job status tracking
- **Comprehensive analytics** and reporting
- **High availability** with caching and monitoring
- **Event-driven architecture** for loose coupling

## Clean Architecture Layers

```
┌─────────────────────────────────────────────────────────────────┐
│                        CLEAN ARCHITECTURE                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                 EXTERNAL INTERFACES                     │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │   REST API  │  │   S3 Files  │  │  PostgreSQL │     │   │
│  │  │   (Gin)     │  │   (JSON)    │  │  (Database) │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  └─────────────────────────────────────────────────────────┘   │
│                               │                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              INFRASTRUCTURE LAYER                       │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │ HTTP Server │  │ S3 Client   │  │ Repository  │     │   │
│  │  │ (handlers)  │  │ (AWS SDK)   │  │ (GORM)      │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │JSON Processor│  │ Cache       │  │ Events      │     │   │
│  │  │(File Parser)│  │ (Redis)     │  │ (Publisher) │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  └─────────────────────────────────────────────────────────┘   │
│                               │                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              APPLICATION LAYER                          │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │   Handlers  │  │ Processing  │  │ Validation  │     │   │
│  │  │ (HTTP Logic)│  │   Engine    │  │ (Business)  │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │    Jobs     │  │ Middleware  │  │  Metrics    │     │   │
│  │  │ (Async)     │  │(Auth/Log)   │  │ (Telemetry) │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  └─────────────────────────────────────────────────────────┘   │
│                               │                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                 DOMAIN LAYER                            │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │  Entities   │  │  Services   │  │   Ports     │     │   │
│  │  │ (Models)    │  │ (Business)  │  │(Interfaces) │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │ Validation  │  │ Aggregates  │  │ Value Obj   │     │   │
│  │  │ (Rules)     │  │ (Complex)   │  │ (Immutable) │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Layer Responsibilities

#### 1. Domain Layer (Core Business Logic)
- **Entities**: `Review`, `Hotel`, `Provider`, `ReviewerInfo`
- **Services**: Business logic for review processing, validation, analytics
- **Ports**: Interface contracts for external dependencies
- **Rules**: Business validation and enrichment logic

#### 2. Application Layer (Use Cases)
- **Handlers**: HTTP request/response handling
- **Processing Engine**: Asynchronous file processing
- **Middleware**: Authentication, logging, metrics
- **Validation**: Input validation and sanitization

#### 3. Infrastructure Layer (External Concerns)
- **Repository**: Database operations with GORM
- **S3 Client**: File storage and retrieval
- **JSON Processor**: File parsing and validation
- **Cache**: Redis for performance optimization
- **Events**: Message publishing for decoupling

## Component Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    COMPONENT INTERACTION                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐    ┌─────────────────────────────────────┐    │
│  │   Client    │────│            API Gateway             │    │
│  │ (Web/Mobile)│    │         (Rate Limiting)            │    │
│  └─────────────┘    └─────────────────────────────────────┘    │
│                                       │                        │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                HTTP HANDLERS                            │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │   Reviews   │  │   Hotels    │  │ Providers   │     │   │
│  │  │   Handler   │  │   Handler   │  │  Handler    │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │ Processing  │  │ Analytics   │  │   Health    │     │   │
│  │  │  Handler    │  │  Handler    │  │  Handler    │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  └─────────────────────────────────────────────────────────┘   │
│                               │                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                BUSINESS SERVICES                        │   │
│  │                                                         │   │
│  │  ┌─────────────────────────────────────────────────┐   │   │
│  │  │              ReviewService                      │   │   │
│  │  │  ┌─────────────┐  ┌─────────────┐  ┌─────────┐ │   │   │
│  │  │  │  Validation │  │ Enrichment  │  │Analytics│ │   │   │
│  │  │  │   Engine    │  │   Engine    │  │ Engine  │ │   │   │
│  │  │  └─────────────┘  └─────────────┘  └─────────┘ │   │   │
│  │  └─────────────────────────────────────────────────┘   │   │
│  └─────────────────────────────────────────────────────────┘   │
│                               │                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              PROCESSING ENGINE                          │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │ Job Queue   │  │ Worker Pool │  │ Status      │     │   │
│  │  │ (Async)     │  │ (Goroutines)│  │ Tracking    │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │Backpressure │  │ Progress    │  │ Error       │     │   │
│  │  │ Controller  │  │ Tracker     │  │ Handling    │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  └─────────────────────────────────────────────────────────┘   │
│                               │                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                DATA ACCESS LAYER                        │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │ Repository  │  │ Cache       │  │ S3 Client   │     │   │
│  │  │ (GORM)      │  │ (Redis)     │  │ (AWS SDK)   │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  └─────────────────────────────────────────────────────────┘   │
│                               │                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                 EXTERNAL SYSTEMS                        │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │ PostgreSQL  │  │ Redis Cache │  │ AWS S3      │     │   │
│  │  │ (Primary)   │  │ (Session)   │  │ (Files)     │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │ Prometheus  │  │ Grafana     │  │ Jaeger      │     │   │
│  │  │ (Metrics)   │  │ (Dashboard) │  │ (Tracing)   │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Data Flow Architecture

### 1. Review Processing Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                    REVIEW PROCESSING FLOW                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐         │
│  │   Provider  │    │   AWS S3    │    │ Processing  │         │
│  │ (Booking.com│───▶│  (JSON      │───▶│   Engine    │         │
│  │  Expedia)   │    │   Files)    │    │ (Async)     │         │
│  └─────────────┘    └─────────────┘    └─────────────┘         │
│                                                │                │
│                                                ▼                │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                FILE PROCESSING PIPELINE                 │   │
│  │                                                         │   │
│  │  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐ │   │
│  │  │   Download  │───▶│    Parse    │───▶│   Validate  │ │   │
│  │  │   (S3 API)  │    │  (JSON)     │    │ (Business)  │ │   │
│  │  └─────────────┘    └─────────────┘    └─────────────┘ │   │
│  │                                                         │   │
│  │  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐ │   │
│  │  │   Enrich    │───▶│   Batch     │───▶│   Store     │ │   │
│  │  │ (Sentiment) │    │ (1000 rows) │    │ (Database)  │ │   │
│  │  └─────────────┘    └─────────────┘    └─────────────┘ │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                │                │
│                                                ▼                │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    DATA STORAGE                         │   │
│  │                                                         │   │
│  │  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐ │   │
│  │  │ PostgreSQL  │    │ Redis Cache │    │ Event Bus   │ │   │
│  │  │ (Primary)   │    │ (Summary)   │    │ (Notify)    │ │   │
│  │  └─────────────┘    └─────────────┘    └─────────────┘ │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                │                │
│                                                ▼                │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                POST-PROCESSING                          │   │
│  │                                                         │   │
│  │  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐ │   │
│  │  │   Update    │    │   Send      │    │   Metrics   │ │   │
│  │  │  Summary    │    │ Notification│    │ Collection  │ │   │
│  │  └─────────────┘    └─────────────┘    └─────────────┘ │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 2. API Request Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                      API REQUEST FLOW                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐         │
│  │   Client    │───▶│   Load      │───▶│   API       │         │
│  │ (Frontend)  │    │ Balancer    │    │ Gateway     │         │
│  └─────────────┘    └─────────────┘    └─────────────┘         │
│                                                │                │
│                                                ▼                │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                   MIDDLEWARE CHAIN                      │   │
│  │                                                         │   │
│  │  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐ │   │
│  │  │    CORS     │───▶│   Request   │───▶│  Recovery   │ │   │
│  │  │             │    │   Logger    │    │ (Panics)    │ │   │
│  │  └─────────────┘    └─────────────┘    └─────────────┘ │   │
│  │                                                         │   │
│  │  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐ │   │
│  │  │ Rate Limit  │───▶│    Auth     │───▶│ Validation  │ │   │
│  │  │ (Throttle)  │    │ (JWT/API)   │    │ (Input)     │ │   │
│  │  └─────────────┘    └─────────────┘    └─────────────┘ │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                │                │
│                                                ▼                │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                  BUSINESS LOGIC                         │   │
│  │                                                         │   │
│  │  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐ │   │
│  │  │   Handler   │───▶│   Service   │───▶│ Repository  │ │   │
│  │  │ (HTTP)      │    │ (Business)  │    │ (Data)      │ │   │
│  │  └─────────────┘    └─────────────┘    └─────────────┘ │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                │                │
│                                                ▼                │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                   DATA ACCESS                           │   │
│  │                                                         │   │
│  │  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐ │   │
│  │  │    Cache    │    │  Database   │    │   Events    │ │   │
│  │  │   (Redis)   │    │(PostgreSQL) │    │ (Publish)   │ │   │
│  │  └─────────────┘    └─────────────┘    └─────────────┘ │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                │                │
│                                                ▼                │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    RESPONSE                             │   │
│  │                                                         │   │
│  │  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐ │   │
│  │  │   Format    │───▶│   Headers   │───▶│   Client    │ │   │
│  │  │   (JSON)    │    │ (CORS/etc)  │    │ (Response)  │ │   │
│  │  └─────────────┘    └─────────────┘    └─────────────┘ │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Deployment Architecture

### 1. Container Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    CONTAINER DEPLOYMENT                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    LOAD BALANCER                        │   │
│  │                 (Nginx/HAProxy)                         │   │
│  │              Port 80/443 (SSL)                         │   │
│  └─────────────────────────────────────────────────────────┘   │
│                               │                                 │
│                               ▼                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                APPLICATION TIER                         │   │
│  │                                                         │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │   API-1     │  │   API-2     │  │   API-3     │     │   │
│  │  │  (Port      │  │  (Port      │  │  (Port      │     │   │
│  │  │   8080)     │  │   8081)     │  │   8082)     │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  │                                                         │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │ Processing  │  │ Processing  │  │ Processing  │     │   │
│  │  │ Worker-1    │  │ Worker-2    │  │ Worker-3    │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  └─────────────────────────────────────────────────────────┘   │
│                               │                                 │
│                               ▼                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                  DATA TIER                              │   │
│  │                                                         │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │ PostgreSQL  │  │ Redis       │  │ AWS S3      │     │   │
│  │  │ (Primary)   │  │ (Cache)     │  │ (Files)     │     │   │
│  │  │ Port 5432   │  │ Port 6379   │  │ HTTPS       │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  └─────────────────────────────────────────────────────────┘   │
│                               │                                 │
│                               ▼                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                MONITORING TIER                          │   │
│  │                                                         │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │ Prometheus  │  │ Grafana     │  │ Jaeger      │     │   │
│  │  │ (Metrics)   │  │ (Dashboard) │  │ (Tracing)   │     │   │
│  │  │ Port 9090   │  │ Port 3000   │  │ Port 14268  │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 2. Kubernetes Deployment

```
┌─────────────────────────────────────────────────────────────────┐
│                  KUBERNETES ARCHITECTURE                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                     INGRESS                             │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │   Route     │  │    TLS      │  │    Rate     │     │   │
│  │  │   Rules     │  │ Termination │  │   Limiting  │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  └─────────────────────────────────────────────────────────┘   │
│                               │                                 │
│                               ▼                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    SERVICES                             │   │
│  │                                                         │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │   API       │  │ Processing  │  │ Monitoring  │     │   │
│  │  │ Service     │  │  Service    │  │  Service    │     │   │
│  │  │(ClusterIP)  │  │(ClusterIP)  │  │(ClusterIP)  │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  └─────────────────────────────────────────────────────────┘   │
│                               │                                 │
│                               ▼                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                   DEPLOYMENTS                           │   │
│  │                                                         │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │   API       │  │ Processing  │  │ Monitoring  │     │   │
│  │  │ Deployment  │  │ Deployment  │  │ Deployment  │     │   │
│  │  │(3 replicas) │  │(2 replicas) │  │(1 replica)  │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  └─────────────────────────────────────────────────────────┘   │
│                               │                                 │
│                               ▼                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                     PODS                                │   │
│  │                                                         │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │   API       │  │   API       │  │   API       │     │   │
│  │  │   Pod-1     │  │   Pod-2     │  │   Pod-3     │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  │                                                         │   │
│  │  ┌─────────────┐  ┌─────────────┐                      │   │
│  │  │ Processing  │  │ Processing  │                      │   │
│  │  │   Pod-1     │  │   Pod-2     │                      │   │
│  │  └─────────────┘  └─────────────┘                      │   │
│  └─────────────────────────────────────────────────────────┘   │
│                               │                                 │
│                               ▼                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                PERSISTENT STORAGE                       │   │
│  │                                                         │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │ PostgreSQL  │  │ Redis       │  │ Config      │     │   │
│  │  │    PVC      │  │   PVC       │  │  Maps       │     │   │
│  │  │  (100GB)    │  │  (10GB)     │  │ (Secrets)   │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Integration Patterns

### 1. Event-Driven Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                  EVENT-DRIVEN INTEGRATION                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐         │
│  │   Review    │───▶│   Event     │───▶│ Downstream  │         │
│  │  Service    │    │   Bus       │    │  Services   │         │
│  │ (Publisher) │    │ (Message)   │    │ (Consumers) │         │
│  └─────────────┘    └─────────────┘    └─────────────┘         │
│                                                                 │
│  Event Types:                                                   │
│  • review.created                                              │
│  • review.updated                                              │
│  • review.deleted                                              │
│  • processing.started                                          │
│  • processing.completed                                        │
│  • processing.failed                                           │
│  • hotel.created                                               │
│  • hotel.updated                                               │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 2. Cache-Aside Pattern

```
┌─────────────────────────────────────────────────────────────────┐
│                    CACHE-ASIDE PATTERN                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐         │
│  │   Client    │───▶│ Application │───▶│ Cache       │         │
│  │ (Request)   │    │  Service    │    │ (Redis)     │         │
│  └─────────────┘    └─────────────┘    └─────────────┘         │
│                               │                │                │
│                               ▼                │                │
│                    ┌─────────────┐             │                │
│                    │  Database   │◄────────────┘                │
│                    │(PostgreSQL) │                              │
│                    └─────────────┘                              │
│                                                                 │
│  Flow:                                                          │
│  1. Check cache first                                           │
│  2. If miss, query database                                     │
│  3. Store result in cache                                       │
│  4. Return to client                                            │
│  5. Invalidate on updates                                       │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 3. Circuit Breaker Pattern

```
┌─────────────────────────────────────────────────────────────────┐
│                   CIRCUIT BREAKER PATTERN                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐         │
│  │   Client    │───▶│ Circuit     │───▶│ External    │         │
│  │ (Request)   │    │ Breaker     │    │ Service     │         │
│  └─────────────┘    └─────────────┘    └─────────────┘         │
│                                                                 │
│  States:                                                        │
│  • CLOSED   - Normal operation                                  │
│  • OPEN     - Failing fast                                      │
│  • HALF_OPEN - Testing recovery                                 │
│                                                                 │
│  Configuration:                                                 │
│  • Failure threshold: 50%                                       │
│  • Recovery timeout: 60s                                        │
│  • Success threshold: 3                                         │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Technology Stack

### Core Technologies
- **Language**: Go 1.21+
- **Framework**: Gin (HTTP), GORM (ORM)
- **Database**: PostgreSQL 15+
- **Cache**: Redis 7+
- **Storage**: AWS S3
- **Containerization**: Docker, Kubernetes

### Monitoring & Observability
- **Metrics**: Prometheus
- **Visualization**: Grafana
- **Tracing**: Jaeger
- **Logging**: Structured logging with slog
- **Health Checks**: Custom health endpoints

### Testing & Quality
- **Testing**: Testify framework
- **Coverage**: 90%+ test coverage
- **Mocking**: Testify mocks
- **Linting**: golangci-lint
- **Security**: gosec, dependency scanning

### DevOps & Deployment
- **CI/CD**: GitHub Actions
- **Infrastructure**: Terraform
- **Service Mesh**: Istio (optional)
- **Load Balancing**: Nginx/HAProxy
- **SSL/TLS**: Let's Encrypt

## Security Architecture

### 1. Authentication & Authorization

```
┌─────────────────────────────────────────────────────────────────┐
│                   SECURITY ARCHITECTURE                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐         │
│  │   Client    │───▶│   API       │───▶│   Auth      │         │
│  │ (JWT Token) │    │ Gateway     │    │ Service     │         │
│  └─────────────┘    └─────────────┘    └─────────────┘         │
│                                                │                │
│                                                ▼                │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                SECURITY LAYERS                          │   │
│  │                                                         │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │    TLS      │  │    JWT      │  │    RBAC     │     │   │
│  │  │ (Transport) │  │ (Identity)  │  │ (Permissions)│     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  │                                                         │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │ Rate Limit  │  │ Input Valid │  │ SQL Inject  │     │   │
│  │  │ (DDoS)      │  │ (Sanitize)  │  │ (Prevention)│     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 2. Data Protection

```
┌─────────────────────────────────────────────────────────────────┐
│                     DATA PROTECTION                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                  DATA AT REST                           │   │
│  │                                                         │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │ Database    │  │ S3 Bucket   │  │ Config      │     │   │
│  │  │ Encryption  │  │ Encryption  │  │ Secrets     │     │   │
│  │  │ (AES-256)   │  │ (AES-256)   │  │ (Vault)     │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                DATA IN TRANSIT                          │   │
│  │                                                         │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │    TLS      │  │   mTLS      │  │   VPN       │     │   │
│  │  │ (External)  │  │ (Internal)  │  │ (Network)   │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                DATA IN MEMORY                           │   │
│  │                                                         │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │   │
│  │  │ Zero Copy   │  │ Secure      │  │ Memory      │     │   │
│  │  │ (Buffers)   │  │ Cleanup     │  │ Protection  │     │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘     │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Performance Considerations

### 1. Scalability Patterns

- **Horizontal Scaling**: Multiple API instances behind load balancer
- **Database Sharding**: Partition by provider_id or hotel_id
- **Read Replicas**: Separate read/write database instances
- **Caching Strategy**: Multi-level caching (Redis, CDN, Application)
- **Connection Pooling**: Optimized database connections

### 2. Performance Optimizations

- **Batch Processing**: Process reviews in configurable batches
- **Async Operations**: Non-blocking file processing
- **Memory Management**: Efficient memory usage with streaming
- **Query Optimization**: Indexed searches, query caching
- **Compression**: Gzip compression for API responses

### 3. Monitoring & Alerting

- **SLA Monitoring**: 99.9% uptime target
- **Performance Metrics**: Response times, throughput
- **Resource Usage**: CPU, memory, disk, network
- **Business Metrics**: Reviews processed, error rates
- **Alerting**: Threshold-based alerts with escalation

---

## Summary

This architecture provides a robust, scalable, and maintainable foundation for the Hotel Reviews Microservice. The clean architecture ensures separation of concerns, while the comprehensive monitoring and security measures provide production-ready capabilities. The event-driven design enables future expansion and integration with other services in the ecosystem.

The system is designed to handle high throughput review processing while maintaining data consistency and providing real-time analytics capabilities for business intelligence and decision making.