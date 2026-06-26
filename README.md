# Ticket System - Backend Service

A simple yet complete REST API for a ticket management system built with Golang and SQLite.

## Features

- **User Authentication**: Registration and login with JWT tokens
- **Ticket Management**: Create, view, and update tickets
- **Ownership-Based Authorization**: Users can only see and modify their own tickets
- **Status Management**: Tickets flow through open → in_progress → closed states
- **Secure Passwords**: All passwords are hashed using bcrypt

## Tech Stack

- **Language**: Golang 1.21+
- **Database**: SQLite
- **Authentication**: JWT (JSON Web Tokens)
- **Password Hashing**: bcrypt
- **Deployment**: Docker

## Local Setup

### Prerequisites

- Go 1.21 or higher installed
- SQLite3

### Running Locally

1. **Clone the repository**
   ```bash
   git clone <repo-url>
   cd ticket-system
   ```

2. **Download dependencies**
   ```bash
   go mod download
   ```

3. **Run the application**
   ```bash
   go run main.go
   ```

   The server will start on `http://localhost:8080`

4. **Test the health endpoint**
   ```bash
   curl http://localhost:8080/health
   ```

   Expected response:
   ```json
   {
     "status": "ok"
   }
   ```

## Docker Setup

### Building the Docker Image

```bash
docker build -t ticket-system .
```

### Running with Docker

```bash
docker run -p 8080:8080 ticket-system
```

The application will be accessible at `http://localhost:8080`

## API Endpoints

### Public Endpoints

#### Health Check
```
GET /health
```
Returns the service health status.

#### User Registration
```
POST /auth/register
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "password123"
}
```

#### User Login
```
POST /auth/login
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "password123"
}
```

Response:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "email": "user@example.com"
}
```

### Protected Endpoints (Require Authorization: Bearer <token>)

#### Create a Ticket
```
POST /tickets
Authorization: Bearer <token>
Content-Type: application/json

{
  "title": "Fix login bug"
}
```

#### List User's Tickets
```
GET /tickets
Authorization: Bearer <token>
```

#### Get a Specific Ticket
```
GET /tickets/{id}
Authorization: Bearer <token>
```

#### Update Ticket Status
```
PATCH /tickets/{id}/status
Authorization: Bearer <token>
Content-Type: application/json

{
  "status": "in_progress"
}
```

Valid status transitions:
- `open` → `in_progress` or `closed`
- `in_progress` → `closed`
- `closed` → (no transitions allowed)

## Status Codes

- `200 OK`: Successful GET/PATCH request
- `201 Created`: Successful POST request (registration, ticket creation)
- `400 Bad Request`: Invalid request or status transition
- `401 Unauthorized`: Missing or invalid authentication token
- `404 Not Found`: Resource not found
- `409 Conflict`: Email already registered
- `500 Internal Server Error`: Server error

## Database Schema

### users table
- `id`: INTEGER PRIMARY KEY
- `email`: TEXT UNIQUE
- `password`: TEXT (bcrypt hash)
- `created_at`: TIMESTAMP

### tickets table
- `id`: INTEGER PRIMARY KEY
- `user_id`: INTEGER FOREIGN KEY
- `title`: TEXT
- `status`: TEXT (open, in_progress, closed)
- `created_at`: TIMESTAMP
- `updated_at`: TIMESTAMP

## Deployment

The application is ready for deployment on any free-tier hosting platform that supports Docker or Go:

### Free Hosting Options

1. **Railway.app** (Recommended - free tier available)
2. **Render.com** (Free tier available)
3. **Heroku** (Free tier removed, but alternatives work)
4. **Fly.io** (Free tier available)

### Example: Deploying to Railway.app

1. Push code to GitHub
2. Connect repository to Railway.app
3. Set environment variables if needed
4. Deploy

The application will automatically expose the `/health` endpoint publicly.

## Environment Variables

Optional:
- `JWT_SECRET`: Secret key for JWT signing (default: "your-secret-key-change-in-production")
- `PORT`: Port to run the service on (default: "8080")

Copy `.env.example` to `.env` and modify as needed.

## Assumptions

1. Single-user ticket system (no ticket assignment to other users)
2. In-memory or persistent SQLite database
3. Passwords are hashed with bcrypt
4. JWT tokens expire after 24 hours
5. No admin roles or permissions
6. Simple email/password authentication (no OAuth)
7. No ticket comments or nested resources
8. Tickets can only be created with a title

## Testing

### Example workflow:

```bash
# Register a user
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email": "test@example.com", "password": "test123"}'

# Login
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "test@example.com", "password": "test123"}'

# Use the returned token in subsequent requests
TOKEN="<token-from-login>"

# Create a ticket
curl -X POST http://localhost:8080/tickets \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title": "Sample ticket"}'

# List tickets
curl -X GET http://localhost:8080/tickets \
  -H "Authorization: Bearer $TOKEN"

# Update ticket status
curl -X PATCH http://localhost:8080/tickets/1/status \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"status": "in_progress"}'
```

## Project Structure

```
ticket-system/
├── main.go              # Main application with all endpoints
├── go.mod              # Go module definition
├── go.sum              # Go dependencies lock file
├── Dockerfile          # Docker configuration
├── .env.example        # Environment variables example
└── README.md           # This file
```

## Notes

- The SQLite database file (`tickets.db`) is created automatically on first run
- All timestamps use RFC3339 format
- The API uses standard HTTP status codes and JSON responses
- All passwords are stored as bcrypt hashes (never plain text)
- JWT tokens are required for all ticket operations

## Security Notes

⚠️ **For production use:**
- Change the `JWT_SECRET` environment variable to a strong, random value
- Use HTTPS/TLS for all communications
- Consider rate limiting on authentication endpoints
- Implement CORS policies if needed
- Regular security audits recommended

## Support

For issues or questions, please check the implementation against the specification in the assignment document.

---

**Submission Date**: Today
**Status**: Ready for deployment
