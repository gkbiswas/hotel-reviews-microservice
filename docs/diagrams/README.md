# Architecture Diagrams

This directory contains comprehensive architecture diagrams for the Hotel Reviews Microservice in Mermaid format.

## 📊 Available Diagrams

### 1. System Context Diagram
**File**: `system-context.mmd`
**Purpose**: Shows the hotel reviews microservice and its external dependencies
**Content**:
- External users (clients, admin, DevOps)
- Hotel Reviews Microservice system boundary
- External systems (S3, PostgreSQL, Redis, monitoring)
- Data flow between systems

### 2. Container Diagram
**File**: `container-diagram.mmd`
**Purpose**: Shows the internal structure of the hotel reviews microservice
**Content**:
- Load balancer configuration
- API layer with multiple instances
- Processing layer with worker pools
- Data layer (PostgreSQL, Redis, S3)
- Monitoring stack (Prometheus, Grafana, Jaeger)

### 3. Component Diagram
**File**: `component-diagram.mmd`
**Purpose**: Shows internal components and their relationships
**Content**:
- HTTP layer (router, middleware, handlers)
- Application layer (services, processing components)
- Domain layer (domain services, entities)
- Infrastructure layer (repositories, clients, processors)

### 4. File Processing Sequence Diagram
**File**: `file-processing-sequence.mmd`
**Purpose**: Shows the complete flow of file processing from upload to completion
**Content**:
- File upload and processing initiation
- Async processing with job queue
- File download from S3
- JSON parsing and validation
- Data enrichment and storage
- Error handling and monitoring

### 5. API Request Sequence Diagram
**File**: `api-request-sequence.mmd`
**Purpose**: Shows the flow of a typical API request through the system
**Content**:
- Load balancer routing
- Middleware chain (auth, rate limiting, logging)
- Service layer processing
- Cache and database interactions
- Response formatting and metrics

### 6. Database Schema ERD
**File**: `database-schema.mmd`
**Purpose**: Shows the complete database schema with relationships
**Content**:
- Core entities (Provider, Hotel, Review, ReviewerInfo)
- Supporting entities (ReviewSummary, ProcessingStatus)
- Relationships and constraints
- Indexes and performance optimizations

## 🔧 How to Use These Diagrams

### Rendering with Mermaid

1. **Online Editor**: Copy the content to [Mermaid Live Editor](https://mermaid.live/)
2. **VS Code**: Install the Mermaid Preview extension
3. **GitHub**: GitHub automatically renders `.mmd` files
4. **Command Line**: Use `mermaid-cli` to generate images

### Command Line Rendering

```bash
# Install mermaid-cli
npm install -g @mermaid-js/mermaid-cli

# Generate PNG images
mmdc -i system-context.mmd -o system-context.png
mmdc -i container-diagram.mmd -o container-diagram.png
mmdc -i component-diagram.mmd -o component-diagram.png
mmdc -i file-processing-sequence.mmd -o file-processing-sequence.png
mmdc -i api-request-sequence.mmd -o api-request-sequence.png
mmdc -i database-schema.mmd -o database-schema.png

# Generate SVG images
mmdc -i system-context.mmd -o system-context.svg
mmdc -i container-diagram.mmd -o container-diagram.svg
mmdc -i component-diagram.mmd -o component-diagram.svg
mmdc -i file-processing-sequence.mmd -o file-processing-sequence.svg
mmdc -i api-request-sequence.mmd -o api-request-sequence.svg
mmdc -i database-schema.mmd -o database-schema.svg
```

### Batch Generation Script

```bash
#!/bin/bash
# Generate all diagrams as PNG and SVG

diagrams=(
    "system-context"
    "container-diagram"
    "component-diagram"
    "file-processing-sequence"
    "api-request-sequence"
    "database-schema"
)

for diagram in "${diagrams[@]}"; do
    echo "Generating $diagram..."
    mmdc -i "$diagram.mmd" -o "$diagram.png"
    mmdc -i "$diagram.mmd" -o "$diagram.svg"
done

echo "All diagrams generated successfully!"
```

## 📱 Integration with Documentation

These diagrams complement the architecture documentation in:
- `../architecture.md` - Comprehensive architecture overview
- `../../README.md` - Project overview and setup
- `../../COVERAGE_REPORT.md` - Test coverage analysis

## 🎨 Diagram Color Coding

- **Blue**: HTTP/API layer components
- **Orange**: Application/Processing layer components  
- **Green**: Domain/Business logic components
- **Pink**: Infrastructure/External components
- **Purple**: Configuration and utilities

## 📈 Diagram Maintenance

When updating the system:
1. Update the corresponding diagram files
2. Regenerate images if needed
3. Update this README if new diagrams are added
4. Ensure consistency across all diagrams

## 🔍 Diagram Details

### System Context Diagram
- **Focus**: External system interactions
- **Audience**: Stakeholders, architects
- **Level**: System level (C4 Level 1)

### Container Diagram
- **Focus**: High-level technology choices
- **Audience**: Technical teams, architects
- **Level**: Container level (C4 Level 2)

### Component Diagram
- **Focus**: Component interactions
- **Audience**: Developers, architects
- **Level**: Component level (C4 Level 3)

### Sequence Diagrams
- **Focus**: Process flows and interactions
- **Audience**: Developers, QA teams
- **Level**: Detailed behavioral view

### Database Schema ERD
- **Focus**: Data model and relationships
- **Audience**: Database administrators, developers
- **Level**: Data architecture view

## 📖 References

- [Mermaid Documentation](https://mermaid.js.org/)
- [C4 Model](https://c4model.com/)
- [Clean Architecture](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Microservices Patterns](https://microservices.io/patterns/)