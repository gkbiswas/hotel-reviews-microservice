# Authentication Guide

## Overview

The Hotel Reviews API uses JWT (JSON Web Token) authentication to secure endpoints. This guide covers registration, login, token management, and best practices.

## Authentication Flow

```mermaid
sequenceDiagram
    participant Client
    participant API
    participant Auth Service
    participant Database

    Client->>API: POST /auth/register
    API->>Database: Create user
    API-->>Client: User + Tokens

    Client->>API: POST /auth/login
    API->>Auth Service: Validate credentials
    Auth Service->>Database: Check user
    Auth Service-->>API: Generate tokens
    API-->>Client: Access + Refresh tokens

    Client->>API: GET /reviews (with token)
    API->>Auth Service: Validate token
    Auth Service-->>API: Token valid
    API-->>Client: Protected resource

    Client->>API: POST /auth/refresh
    API->>Auth Service: Validate refresh token
    Auth Service-->>API: New access token
    API-->>Client: New access token
```

## Registration

### Create a New Account

```bash
curl -X POST https://api.hotelreviews.com/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "johndoe",
    "email": "john.doe@example.com",
    "password": "SecurePassword123!",
    "first_name": "John",
    "last_name": "Doe"
  }'
```

#### Request Body

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `username` | string | Yes | Unique username (3-50 chars, alphanumeric + underscore) |
| `email` | string | Yes | Valid email address |
| `password` | string | Yes | Strong password (min 8 chars, uppercase, lowercase, number, special) |
| `first_name` | string | Yes | User's first name |
| `last_name` | string | Yes | User's last name |

#### Response

```json
{
  "success": true,
  "data": {
    "user": {
      "id": "123e4567-e89b-12d3-a456-426614174000",
      "username": "johndoe",
      "email": "john.doe@example.com",
      "first_name": "John",
      "last_name": "Doe",
      "is_verified": false,
      "created_at": "2024-01-15T10:30:00Z"
    },
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_in": 3600
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### Password Requirements

- Minimum 8 characters
- At least one uppercase letter (A-Z)
- At least one lowercase letter (a-z)
- At least one number (0-9)
- At least one special character (@$!%*?&)

## Login

### Authenticate with Credentials

You can login using either email or username:

#### Login with Email

```bash
curl -X POST https://api.hotelreviews.com/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "john.doe@example.com",
    "password": "SecurePassword123!"
  }'
```

#### Login with Username

```bash
curl -X POST https://api.hotelreviews.com/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "johndoe",
    "password": "SecurePassword123!"
  }'
```

#### Response

```json
{
  "success": true,
  "data": {
    "user": {
      "id": "123e4567-e89b-12d3-a456-426614174000",
      "username": "johndoe",
      "email": "john.doe@example.com",
      "first_name": "John",
      "last_name": "Doe",
      "last_login_at": "2024-01-15T10:30:00Z"
    },
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_in": 3600
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

## Using Access Tokens

### Making Authenticated Requests

Include the access token in the Authorization header:

```bash
curl -X GET https://api.hotelreviews.com/api/v1/reviews \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### Token Format

```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

### Token Payload

The JWT token contains the following claims:

```json
{
  "sub": "123e4567-e89b-12d3-a456-426614174000",  // User ID
  "email": "john.doe@example.com",
  "username": "johndoe",
  "roles": ["user", "reviewer"],
  "iat": 1642249200,  // Issued at
  "exp": 1642252800,  // Expires at
  "iss": "hotel-reviews-api",
  "aud": "hotel-reviews-client"
}
```

## Token Refresh

### Refreshing Expired Tokens

Access tokens expire after 1 hour. Use the refresh token to obtain a new access token:

```bash
curl -X POST https://api.hotelreviews.com/api/v1/auth/refresh \
  -H "Authorization: Bearer CURRENT_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "YOUR_REFRESH_TOKEN"
  }'
```

#### Response

```json
{
  "success": true,
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_in": 3600
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### Automatic Token Refresh

#### JavaScript Example

```javascript
class ApiClient {
  constructor(baseUrl) {
    this.baseUrl = baseUrl;
    this.accessToken = localStorage.getItem('access_token');
    this.refreshToken = localStorage.getItem('refresh_token');
  }

  async makeRequest(url, options = {}) {
    const response = await fetch(`${this.baseUrl}${url}`, {
      ...options,
      headers: {
        'Authorization': `Bearer ${this.accessToken}`,
        'Content-Type': 'application/json',
        ...options.headers
      }
    });

    if (response.status === 401) {
      // Token expired, try to refresh
      const refreshed = await this.refreshAccessToken();
      if (refreshed) {
        // Retry the original request
        return this.makeRequest(url, options);
      }
    }

    return response;
  }

  async refreshAccessToken() {
    try {
      const response = await fetch(`${this.baseUrl}/auth/refresh`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${this.accessToken}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          refresh_token: this.refreshToken
        })
      });

      if (response.ok) {
        const data = await response.json();
        this.accessToken = data.data.access_token;
        localStorage.setItem('access_token', this.accessToken);
        return true;
      }
    } catch (error) {
      console.error('Token refresh failed:', error);
    }
    return false;
  }
}
```

#### Python Example

```python
import requests
from datetime import datetime, timedelta

class ApiClient:
    def __init__(self, base_url):
        self.base_url = base_url
        self.access_token = None
        self.refresh_token = None
        self.token_expires_at = None
    
    def login(self, email, password):
        response = requests.post(
            f"{self.base_url}/auth/login",
            json={"email": email, "password": password}
        )
        
        if response.ok:
            data = response.json()['data']
            self.access_token = data['access_token']
            self.refresh_token = data['refresh_token']
            self.token_expires_at = datetime.now() + timedelta(seconds=data['expires_in'])
            return True
        return False
    
    def make_request(self, method, endpoint, **kwargs):
        # Check if token needs refresh
        if self.token_expires_at and datetime.now() >= self.token_expires_at - timedelta(minutes=5):
            self.refresh_access_token()
        
        headers = kwargs.get('headers', {})
        headers['Authorization'] = f"Bearer {self.access_token}"
        kwargs['headers'] = headers
        
        response = requests.request(method, f"{self.base_url}{endpoint}", **kwargs)
        
        # If unauthorized, try refreshing token
        if response.status_code == 401:
            if self.refresh_access_token():
                headers['Authorization'] = f"Bearer {self.access_token}"
                response = requests.request(method, f"{self.base_url}{endpoint}", **kwargs)
        
        return response
    
    def refresh_access_token(self):
        response = requests.post(
            f"{self.base_url}/auth/refresh",
            headers={'Authorization': f"Bearer {self.access_token}"},
            json={'refresh_token': self.refresh_token}
        )
        
        if response.ok:
            data = response.json()['data']
            self.access_token = data['access_token']
            self.token_expires_at = datetime.now() + timedelta(seconds=data['expires_in'])
            return True
        return False
```

## Logout

### Invalidate Tokens

```bash
curl -X POST https://api.hotelreviews.com/api/v1/auth/logout \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

This will invalidate both the access token and refresh token.

## Security Best Practices

### 1. Token Storage

#### Web Applications
- **DO NOT** store tokens in localStorage (XSS vulnerable)
- Use httpOnly cookies for refresh tokens
- Store access tokens in memory

#### Mobile Applications
- Use secure storage (iOS Keychain, Android Keystore)
- Encrypt tokens before storage
- Clear tokens on app logout

### 2. Token Transmission
- Always use HTTPS
- Include tokens only in Authorization header
- Never include tokens in URLs

### 3. Token Expiration
- Access tokens: 1 hour
- Refresh tokens: 30 days
- Refresh access tokens proactively (5 minutes before expiry)

### 4. Error Handling

Handle authentication errors gracefully:

```javascript
async function handleApiError(response) {
  const errorData = await response.json();
  
  switch (errorData.error_code) {
    case 'UNAUTHORIZED':
      // Redirect to login
      window.location.href = '/login';
      break;
      
    case 'INVALID_TOKEN':
      // Clear tokens and redirect
      localStorage.clear();
      window.location.href = '/login';
      break;
      
    case 'FORBIDDEN':
      // Show permission error
      alert('You do not have permission to perform this action');
      break;
      
    default:
      console.error('API Error:', errorData);
  }
}
```

## Role-Based Access Control (RBAC)

### User Roles

| Role | Description | Permissions |
|------|-------------|-------------|
| `user` | Basic user | Read reviews, create own reviews |
| `reviewer` | Verified reviewer | All user permissions + verified badge |
| `moderator` | Content moderator | Edit/delete any review |
| `admin` | Administrator | Full system access |

### Permission Checks

The API automatically checks permissions based on user roles. Forbidden requests return:

```json
{
  "success": false,
  "error": "Insufficient permissions for this operation",
  "error_code": "FORBIDDEN",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

## Multi-Factor Authentication (MFA)

### Enable MFA

```bash
curl -X POST https://api.hotelreviews.com/api/v1/auth/mfa/enable \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### Login with MFA

```bash
curl -X POST https://api.hotelreviews.com/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "john.doe@example.com",
    "password": "SecurePassword123!",
    "mfa_code": "123456"
  }'
```

## API Keys for Service-to-Service

### Create API Key

```bash
curl -X POST https://api.hotelreviews.com/api/v1/auth/api-keys \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Production Server",
    "scopes": ["reviews.read", "reviews.create"],
    "expires_at": "2025-01-15T00:00:00Z"
  }'
```

### Use API Key

```bash
curl -X GET https://api.hotelreviews.com/api/v1/reviews \
  -H "X-API-Key: YOUR_API_KEY"
```

## Troubleshooting

### Common Authentication Errors

| Error Code | Description | Solution |
|------------|-------------|----------|
| `UNAUTHORIZED` | No token provided | Include Authorization header |
| `INVALID_TOKEN` | Token is malformed or expired | Refresh token or login again |
| `FORBIDDEN` | Insufficient permissions | Check user role/permissions |
| `ACCOUNT_LOCKED` | Too many failed attempts | Wait for lockout period |
| `UNVERIFIED_EMAIL` | Email not verified | Check email for verification link |

### Debug Authentication Issues

1. **Check Token Format**
   ```bash
   # Decode JWT token (without verification)
   echo "YOUR_TOKEN" | cut -d. -f2 | base64 -d | jq
   ```

2. **Verify Token Expiry**
   ```javascript
   const payload = JSON.parse(atob(token.split('.')[1]));
   const isExpired = Date.now() >= payload.exp * 1000;
   ```

3. **Test with cURL**
   ```bash
   # Test with verbose output
   curl -v -H "Authorization: Bearer YOUR_TOKEN" \
     https://api.hotelreviews.com/api/v1/reviews
   ```