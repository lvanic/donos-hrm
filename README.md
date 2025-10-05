# Donos HRM

Simple Go web application for submitting and tracking complaints with Google OAuth login.

## Prerequisites

- Go 1.22+
- Google Cloud project with OAuth 2.0 Client ID (Web application)

## Configuration

Set environment variables (via `.env` for local development or system env vars in production):

- `GOOGLE_CLIENT_ID`
- `GOOGLE_CLIENT_SECRET`
- `BASE_URL` (e.g. `https://your-domain.com` or `http://localhost:8080`)
- optional `PORT` (default `8080`)

Create a `.env` file in the project root for local development:

```
GOOGLE_CLIENT_ID=your-client-id
GOOGLE_CLIENT_SECRET=your-client-secret
BASE_URL=http://localhost:8080
```

## Run

```sh
go run ./cmd/app
```

Navigate to `/login` to authenticate with Google.

## Notes

- Complaints are stored in-memory for demo purposes.
- Sessions are kept in-memory; restart clears them.
- OAuth callback must match `BASE_URL/auth/google/callback` in Google Cloud console.

