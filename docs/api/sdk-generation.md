# SDK Generation Guide

## Overview

The Hotel Reviews API provides tools and instructions for generating Software Development Kits (SDKs) in multiple programming languages. This guide covers SDK generation from the OpenAPI specification, customization options, and best practices.

## Supported Languages

### Client SDKs
- **JavaScript/TypeScript** - For web and Node.js applications
- **Python** - For backend services and data analysis
- **Java** - For enterprise applications and Android
- **Go** - For microservices and CLI tools
- **PHP** - For web applications
- **Ruby** - For Rails applications
- **C#/.NET** - For Windows and cross-platform applications
- **Swift** - For iOS applications
- **Kotlin** - For Android applications

### Server SDKs
- **Node.js** (Express, Fastify)
- **Python** (Flask, Django, FastAPI)
- **Java** (Spring Boot)
- **Go** (Gin, Echo)
- **PHP** (Laravel, Symfony)

## SDK Generation Tools

### OpenAPI Generator

We recommend using [OpenAPI Generator](https://openapi-generator.tech/) for creating SDKs:

#### Installation

```bash
# Using npm
npm install @openapitools/openapi-generator-cli -g

# Using Docker
docker pull openapitools/openapi-generator-cli

# Using Homebrew (macOS)
brew install openapi-generator
```

#### Basic Usage

```bash
# Generate JavaScript SDK
openapi-generator-cli generate \
  -i https://api.hotelreviews.com/api/v1/openapi.yaml \
  -g javascript \
  -o ./hotel-reviews-js-sdk \
  --additional-properties=projectName=hotel-reviews-sdk

# Generate Python SDK
openapi-generator-cli generate \
  -i https://api.hotelreviews.com/api/v1/openapi.yaml \
  -g python \
  -o ./hotel-reviews-python-sdk \
  --additional-properties=packageName=hotel_reviews_sdk
```

## Language-Specific SDK Generation

### JavaScript/TypeScript

#### Generate JavaScript SDK

```bash
openapi-generator-cli generate \
  -i api/openapi.yaml \
  -g javascript \
  -o ./sdks/javascript \
  --additional-properties=projectName=hotel-reviews-sdk,projectVersion=1.0.0,usePromises=true
```

#### Generate TypeScript SDK

```bash
openapi-generator-cli generate \
  -i api/openapi.yaml \
  -g typescript-fetch \
  -o ./sdks/typescript \
  --additional-properties=npmName=@hotelreviews/sdk,npmVersion=1.0.0,supportsES6=true
```

#### Configuration File (js-config.json)

```json
{
  "projectName": "hotel-reviews-sdk",
  "projectVersion": "1.0.0",
  "clientPackage": "HotelReviewsClient",
  "usePromises": true,
  "useES6": true,
  "moduleName": "HotelReviewsApi",
  "npmName": "@hotelreviews/sdk",
  "npmRepository": "https://registry.npmjs.org/"
}
```

#### Usage Example

```javascript
import { Configuration, ReviewsApi, AuthApi } from '@hotelreviews/sdk';

// Configure authentication
const config = new Configuration({
  basePath: 'https://api.hotelreviews.com/api/v1',
  accessToken: 'your-access-token'
});

// Create API instances
const authApi = new AuthApi(config);
const reviewsApi = new ReviewsApi(config);

// Use the SDK
async function getReviews() {
  try {
    const response = await reviewsApi.getReviews({
      limit: 20,
      offset: 0
    });
    return response.data;
  } catch (error) {
    console.error('Error fetching reviews:', error);
  }
}
```

### Python

#### Generate Python SDK

```bash
openapi-generator-cli generate \
  -i api/openapi.yaml \
  -g python \
  -o ./sdks/python \
  -c python-config.json
```

#### Configuration File (python-config.json)

```json
{
  "packageName": "hotel_reviews_sdk",
  "projectName": "hotel-reviews-python-sdk",
  "packageVersion": "1.0.0",
  "packageUrl": "https://github.com/hotelreviews/python-sdk",
  "packageCompany": "Hotel Reviews API",
  "packageAuthor": "API Team",
  "packageEmail": "gkbiswas@gmail.com",
  "clientPackage": "hotel_reviews_sdk"
}
```

#### Usage Example

```python
import hotel_reviews_sdk
from hotel_reviews_sdk.api import reviews_api, auth_api
from hotel_reviews_sdk.model.review_create import ReviewCreate

# Configure authentication
configuration = hotel_reviews_sdk.Configuration(
    host='https://api.hotelreviews.com/api/v1',
    access_token='your-access-token'
)

# Create API client
with hotel_reviews_sdk.ApiClient(configuration) as api_client:
    # Create API instances
    reviews_api_instance = reviews_api.ReviewsApi(api_client)
    
    try:
        # Get reviews
        api_response = reviews_api_instance.get_reviews(
            limit=20,
            offset=0
        )
        print(api_response)
    except hotel_reviews_sdk.ApiException as e:
        print(f"Exception when calling ReviewsApi->get_reviews: {e}")
```

### Java

#### Generate Java SDK

```bash
openapi-generator-cli generate \
  -i api/openapi.yaml \
  -g java \
  -o ./sdks/java \
  -c java-config.json
```

#### Configuration File (java-config.json)

```json
{
  "groupId": "com.hotelreviews",
  "artifactId": "hotel-reviews-sdk",
  "artifactVersion": "1.0.0",
  "packageName": "com.hotelreviews.sdk",
  "clientPackage": "com.hotelreviews.sdk.client",
  "invokerPackage": "com.hotelreviews.sdk",
  "modelPackage": "com.hotelreviews.sdk.model",
  "apiPackage": "com.hotelreviews.sdk.api",
  "library": "okhttp-gson",
  "dateLibrary": "java8"
}
```

#### Usage Example

```java
import com.hotelreviews.sdk.ApiClient;
import com.hotelreviews.sdk.ApiException;
import com.hotelreviews.sdk.Configuration;
import com.hotelreviews.sdk.api.ReviewsApi;
import com.hotelreviews.sdk.model.ReviewList;

public class HotelReviewsExample {
    public static void main(String[] args) {
        // Configure authentication
        ApiClient defaultClient = Configuration.getDefaultApiClient();
        defaultClient.setBasePath("https://api.hotelreviews.com/api/v1");
        defaultClient.setBearerToken("your-access-token");
        
        ReviewsApi apiInstance = new ReviewsApi(defaultClient);
        
        try {
            ReviewList result = apiInstance.getReviews(20, 0, null, null);
            System.out.println(result);
        } catch (ApiException e) {
            System.err.println("Exception when calling ReviewsApi#getReviews");
            System.err.println("Status code: " + e.getCode());
            System.err.println("Reason: " + e.getResponseBody());
            e.printStackTrace();
        }
    }
}
```

### Go

#### Generate Go SDK

```bash
openapi-generator-cli generate \
  -i api/openapi.yaml \
  -g go \
  -o ./sdks/go \
  -c go-config.json
```

#### Configuration File (go-config.json)

```json
{
  "packageName": "hotelreviews",
  "packageVersion": "1.0.0",
  "packageUrl": "github.com/hotelreviews/go-sdk",
  "moduleName": "github.com/hotelreviews/go-sdk"
}
```

#### Usage Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/hotelreviews/go-sdk"
)

func main() {
    configuration := hotelreviews.NewConfiguration()
    configuration.Host = "api.hotelreviews.com"
    configuration.Scheme = "https"
    configuration.DefaultHeader["Authorization"] = "Bearer your-access-token"
    
    apiClient := hotelreviews.NewAPIClient(configuration)
    
    ctx := context.Background()
    
    reviews, response, err := apiClient.ReviewsApi.GetReviews(ctx).
        Limit(20).
        Offset(0).
        Execute()
    
    if err != nil {
        log.Fatalf("Error getting reviews: %v", err)
    }
    
    fmt.Printf("Status: %s\n", response.Status)
    fmt.Printf("Reviews: %+v\n", reviews)
}
```

## Advanced SDK Customization

### Custom Templates

Create custom templates for specific requirements:

#### JavaScript Custom Template

```mustache
// templates/javascript/api.mustache
{{>licenseInfo}}
import ApiClient from '../ApiClient';
{{#imports}}
import {{classname}} from '{{filename}}';
{{/imports}}

/**
 * {{appName}} service.
 * {{appDescription}}
 *
 * The version of the OpenAPI document: {{appVersion}}
 * {{#contact}}
 * Contact: {{contactEmail}}
 * {{/contact}}
 */
export default class {{classname}} {
    constructor(apiClient) {
        this.apiClient = apiClient || ApiClient.instance;
        this.apiClient.timeout = 30000; // 30 second timeout
    }

    {{#operations}}
    {{#operation}}
    /**
     * {{summary}}
     * {{notes}}
     {{#allParams}}
     * @param {{{dataType}}} {{paramName}} {{description}}
     {{/allParams}}
     * @param {Object} opts Optional parameters
     * @return {Promise} a Promise that resolves to the response
     */
    {{nickname}}({{#allParams}}{{#-first}}{{paramName}}{{/-first}}{{^-first}}, {{paramName}}{{/-first}}{{/allParams}}{{#hasOptionalParams}}, opts{{/hasOptionalParams}}) {
        // Add automatic retry logic
        return this.apiClient.callApiWithRetry(
            '{{path}}', '{{httpMethod}}',
            {{#allParams}}{{#-first}}{
                {{#pathParams}}'{{paramName}}': {{paramName}}{{#hasMore}},{{/hasMore}}{{/pathParams}}
            }{{/-first}}{{/allParams}},
            {{#allParams}}{{#queryParams}}{{#-first}}{
                {{/queryParams}}{{/-first}}{{#queryParams}}'{{paramName}}': {{#hasMore}}{{paramName}},{{/hasMore}}{{^hasMore}}{{paramName}}{{/hasMore}}{{/queryParams}}{{#queryParams}}{{#-last}}
            }{{/-last}}{{/queryParams}}{{/allParams}},
            {{#allParams}}{{#headerParams}}{{#-first}}{
                {{/headerParams}}{{/-first}}{{#headerParams}}'{{paramName}}': {{#hasMore}}{{paramName}},{{/hasMore}}{{^hasMore}}{{paramName}}{{/hasMore}}{{/headerParams}}{{#headerParams}}{{#-last}}
            }{{/-last}}{{/headerParams}}{{/allParams}},
            {{#hasBodyParam}}{{#bodyParam}}{{paramName}}{{/bodyParam}}{{/hasBodyParam}}{{^hasBodyParam}}null{{/hasBodyParam}},
            {{#returnType}}{{&returnType}}{{/returnType}}{{^returnType}}null{{/returnType}}
        );
    }

    {{/operation}}
    {{/operations}}
}
```

#### Use Custom Templates

```bash
openapi-generator-cli generate \
  -i api/openapi.yaml \
  -g javascript \
  -o ./sdks/javascript \
  -t ./templates/javascript \
  --additional-properties=usePromises=true
```

### Authentication Integration

#### Auto-Refresh Token Support

```javascript
// Custom ApiClient with token refresh
export class ApiClientWithAuth extends ApiClient {
    constructor(config) {
        super();
        this.config = config;
        this.refreshToken = config.refreshToken;
        this.onTokenRefresh = config.onTokenRefresh;
    }

    async callApi(path, httpMethod, pathParams, queryParams, headerParams, formParams, bodyParam, authNames, contentTypes, accepts, returnType) {
        try {
            return await super.callApi(...arguments);
        } catch (error) {
            if (error.status === 401 && this.refreshToken) {
                // Try to refresh token
                const newToken = await this.refreshAccessToken();
                if (newToken) {
                    // Update auth header and retry
                    this.defaultHeaders['Authorization'] = `Bearer ${newToken}`;
                    return await super.callApi(...arguments);
                }
            }
            throw error;
        }
    }

    async refreshAccessToken() {
        try {
            const response = await fetch(`${this.basePath}/auth/refresh`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ refresh_token: this.refreshToken })
            });

            if (response.ok) {
                const data = await response.json();
                const newToken = data.data.access_token;
                
                if (this.onTokenRefresh) {
                    this.onTokenRefresh(newToken);
                }
                
                return newToken;
            }
        } catch (error) {
            console.error('Token refresh failed:', error);
        }
        return null;
    }
}
```

## SDK Testing

### Generate Test Files

```bash
# Add test generation to SDK creation
openapi-generator-cli generate \
  -i api/openapi.yaml \
  -g javascript \
  -o ./sdks/javascript \
  --additional-properties=generateTests=true
```

### Custom Test Template

```javascript
// test-template.mustache
import { expect } from 'chai';
import { {{classname}} } from '../src/index.js';

describe('{{classname}}', function() {
    let apiInstance;

    beforeEach(function() {
        apiInstance = new {{classname}}();
    });

    {{#operations}}
    {{#operation}}
    describe('{{nickname}}', function() {
        it('should call {{nickname}} successfully', function(done) {
            // TODO: implement test
            done();
        });

        it('should handle {{nickname}} errors', function(done) {
            // TODO: implement error test
            done();
        });
    });

    {{/operation}}
    {{/operations}}
});
```

## SDK Documentation

### Generate Documentation

```bash
# Generate SDK with documentation
openapi-generator-cli generate \
  -i api/openapi.yaml \
  -g javascript \
  -o ./sdks/javascript \
  --additional-properties=withDocs=true
```

### README Template

```markdown
# Hotel Reviews SDK

## Installation

\`\`\`bash
npm install @hotelreviews/sdk
\`\`\`

## Quick Start

\`\`\`javascript
import { ReviewsApi, Configuration } from '@hotelreviews/sdk';

const config = new Configuration({
  basePath: 'https://api.hotelreviews.com/api/v1',
  accessToken: 'your-token'
});

const reviewsApi = new ReviewsApi(config);

const reviews = await reviewsApi.getReviews();
\`\`\`

## Error Handling

\`\`\`javascript
try {
  const reviews = await reviewsApi.getReviews();
} catch (error) {
  if (error.status === 401) {
    // Handle authentication error
  } else if (error.status === 429) {
    // Handle rate limit
  }
}
\`\`\`

## Examples

See [examples directory](./examples/) for complete usage examples.
```

## Automated SDK Generation

### CI/CD Pipeline

```yaml
# .github/workflows/generate-sdks.yml
name: Generate SDKs

on:
  push:
    paths:
      - 'api/openapi.yaml'
  release:
    types: [published]

jobs:
  generate-sdks:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        language: [javascript, python, java, go]
    
    steps:
      - uses: actions/checkout@v2
      
      - name: Setup OpenAPI Generator
        run: npm install @openapitools/openapi-generator-cli -g
      
      - name: Generate SDK
        run: |
          openapi-generator-cli generate \
            -i api/openapi.yaml \
            -g ${{ matrix.language }} \
            -o ./sdks/${{ matrix.language }} \
            -c configs/${{ matrix.language }}-config.json
      
      - name: Test SDK
        run: |
          cd ./sdks/${{ matrix.language }}
          # Run language-specific tests
      
      - name: Publish SDK
        if: github.event_name == 'release'
        run: |
          # Publish to package manager
```

### Version Management

```bash
#!/bin/bash
# scripts/generate-all-sdks.sh

VERSION=$(cat api/openapi.yaml | grep "version:" | head -1 | awk '{print $2}')

LANGUAGES=("javascript" "python" "java" "go" "php" "ruby" "csharp")

for lang in "${LANGUAGES[@]}"; do
  echo "Generating $lang SDK version $VERSION"
  
  openapi-generator-cli generate \
    -i api/openapi.yaml \
    -g $lang \
    -o ./sdks/$lang \
    -c configs/$lang-config.json \
    --additional-properties=packageVersion=$VERSION
  
  # Update version in package files
  case $lang in
    "javascript")
      jq ".version = \"$VERSION\"" ./sdks/$lang/package.json > tmp && mv tmp ./sdks/$lang/package.json
      ;;
    "python")
      sed -i "s/version='.*'/version='$VERSION'/" ./sdks/$lang/setup.py
      ;;
    "java")
      sed -i "s/<version>.*<\/version>/<version>$VERSION<\/version>/" ./sdks/$lang/pom.xml
      ;;
  esac
done

echo "All SDKs generated successfully"
```

## Best Practices

### 1. Version Compatibility

- Use semantic versioning for SDKs
- Maintain backward compatibility
- Document breaking changes clearly

### 2. Error Handling

- Implement comprehensive error handling
- Provide meaningful error messages
- Include retry logic for transient errors

### 3. Authentication

- Support automatic token refresh
- Provide secure token storage guidance
- Implement proper credential management

### 4. Performance

- Include connection pooling
- Implement request queuing
- Add caching capabilities where appropriate

### 5. Documentation

- Generate comprehensive API documentation
- Provide usage examples
- Include troubleshooting guides

## Support

- **SDK Issues**: https://github.com/hotelreviews/sdks/issues
- **Documentation**: https://docs.hotelreviews.com/sdks
- **Community**: https://forum.hotelreviews.com/c/sdks