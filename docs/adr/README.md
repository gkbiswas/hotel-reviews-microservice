# Architecture Decision Records (ADRs)

This directory contains Architecture Decision Records for the Hotel Reviews Microservice. ADRs document important architectural decisions, their context, rationale, and consequences.

## What are ADRs?

Architecture Decision Records (ADRs) are lightweight documents that capture important architectural decisions along with their context and consequences. They help teams:

- **Understand** why certain decisions were made
- **Track** the evolution of architecture over time  
- **Onboard** new team members effectively
- **Evaluate** decisions when requirements change
- **Avoid** repeating past discussions

## ADR Format

Each ADR follows a consistent format:
- **Status**: Current state (Proposed, Accepted, Deprecated, Superseded)
- **Context**: The situation and forces that led to the decision
- **Decision**: What was decided
- **Rationale**: Why this decision was made
- **Consequences**: Positive, negative, and neutral impacts

## Current ADRs

| ADR | Title | Status | Date |
|-----|-------|--------|------|
| [000](000-adr-template.md) | ADR Template | Accepted | 2024-01-15 |
| [001](001-clean-architecture-adoption.md) | Clean Architecture Adoption | Accepted | 2024-01-15 |
| [002](002-dependency-injection-strategy.md) | Dependency Injection Strategy | Accepted | 2024-01-15 |
| [003](003-error-handling-strategy.md) | Error Handling Strategy | Accepted | 2024-01-15 |
| [004](004-data-storage-strategy.md) | Data Storage Strategy | Accepted | 2024-01-15 |
| [005](005-testing-strategy.md) | Testing Strategy | Accepted | 2024-01-15 |
| [006](006-api-design-principles.md) | API Design Principles | Accepted | 2024-01-15 |

## Decision Categories

### Core Architecture
- **[ADR-001](001-clean-architecture-adoption.md)**: Foundational architectural pattern
- **[ADR-002](002-dependency-injection-strategy.md)**: Dependency management approach
- **[ADR-004](004-data-storage-strategy.md)**: Data persistence and storage decisions

### Development Practices  
- **[ADR-003](003-error-handling-strategy.md)**: Error handling patterns
- **[ADR-005](005-testing-strategy.md)**: Testing approach and coverage targets
- **[ADR-006](006-api-design-principles.md)**: REST API design standards

## Decision Status Definitions

- **Proposed**: Decision is under consideration
- **Accepted**: Decision has been approved and is being implemented
- **Deprecated**: Decision is no longer recommended but may still be in use
- **Superseded**: Decision has been replaced by a newer decision

## Creating New ADRs

When making significant architectural decisions:

1. **Copy the template**: Use [000-adr-template.md](000-adr-template.md) as a starting point
2. **Number sequentially**: Use the next available ADR number (e.g., 007, 008, etc.)
3. **Follow the format**: Include all required sections with sufficient detail
4. **Get review**: Have the ADR reviewed by relevant stakeholders
5. **Update the index**: Add the new ADR to this README file

### When to Create an ADR

Create an ADR for decisions that:
- **Affect system structure** or component interactions
- **Impact multiple teams** or long-term maintenance
- **Involve trade-offs** between different approaches
- **Set precedents** for future similar decisions
- **Change existing patterns** or established practices

### When NOT to Create an ADR

Don't create ADRs for:
- **Implementation details** that don't affect architecture
- **Temporary workarounds** or short-term fixes
- **Tool choices** that can be easily changed
- **Coding style** decisions (use style guides instead)

## ADR Lifecycle

1. **Draft**: ADR is written and shared for initial feedback
2. **Under Review**: ADR is being evaluated by stakeholders
3. **Accepted**: ADR is approved and implementation can proceed
4. **Implemented**: Decision has been fully implemented
5. **Deprecated**: Decision is no longer recommended
6. **Superseded**: Decision has been replaced by a newer ADR

## Related Documentation

- **[DEVELOPMENT.md](../../DEVELOPMENT.md)**: Development practices and setup
- **[README.md](../../README.md)**: Project overview and getting started
- **[API Documentation](../api/)**: Detailed API specifications
- **[Architecture Overview](../architecture/)**: High-level system design

## References

- [Documenting Architecture Decisions](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions) by Michael Nygard
- [ADR GitHub Organization](https://adr.github.io/) - Tools and examples
- [Architecture Decision Records in Action](https://www.thoughtworks.com/insights/articles/architecture-decision-records-in-action)

---

For questions about ADRs or architectural decisions, please:
1. Review existing ADRs for context
2. Check if your question is covered in related documentation  
3. Open a discussion in the appropriate channel
4. Consider whether a new ADR is needed for significant decisions