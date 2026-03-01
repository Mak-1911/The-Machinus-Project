# Machinus Cloud Agent

A lightweight intelligent cloud server built in Go that accepts chat instructions, plans structured steps, executes sandboxed shell tasks, and persists task history with long-term memory.

## Features

- **Chat-based Task Execution**: Send natural language requests and get structured plans
- **Sandboxed Shell Execution**: Safe command execution with timeouts and dangerous command blocking
- **LLM Planning**: Uses GLM/Z.AI API for intelligent task planning
- **WebSocket Streaming**: Real-time log streaming for long-running tasks
- **Persistent Storage**: SQLite database with full-text search for memory
- **Long-term Memory**: Summarizes and remembers task results for future context

## Architecture

```
User → HTTP/WebSocket → API → Orchestrator → Planner → Tools → Storage
                                           ↓
                                         Memory
```

## Quick Start

### Prerequisites

1. **Go 1.21+**
2. **GLM/Z.AI API Key**

### Configuration

1. Copy `.env.example` to `.env`
2. Update with your values:

```bash
cp .env.example .env
```

Edit `.env`:
```env
DATABASE_URL=./data/machinus.db
LLM_API_KEY=your-zai-api-key
AUTH_TOKEN=your-secret-token
```

### Running

```bash
# Build
go build -o bin/server ./cmd/server

# Create data directory (will be created automatically if needed)
mkdir -p data

# Run
./bin/server

# Or run directly
go run cmd/server/main.go
```

The server will start on `http://localhost:8080` and create the SQLite database automatically.

## API Usage

### REST API

```bash
# Execute a task
curl -H "Authorization: Bearer dev-token" \
  http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Create a new Go project called myapp"}'

# List tasks
curl -H "Authorization: Bearer dev-token" \
  http://localhost:8080/api/tasks

# Get task details
curl -H "Authorization: Bearer dev-token" \
  http://localhost:8080/api/tasks/{task-id}

# Get task logs
curl -H "Authorization: Bearer dev-token" \
  http://localhost:8080/api/tasks/{task-id}/logs
```

### WebSocket

```javascript
const ws = new WebSocket('ws://localhost:8080/ws?token=dev-token');

ws.onopen = () => {
  // Send a message
  ws.send(JSON.stringify({
    message: "Create a new Go project"
  }));
};

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log(data.type, data.message);
};
```

Message types:
- `connected`: Connection established
- `plan`: Execution plan generated
- `log`: Real-time log entry
- `complete`: Task completed
- `error`: Error occurred

## Project Structure

```
.
├── cmd/
│   └── server/
│       └── main.go          # Application entry point
├── internal/
│   ├── agent/               # Core agent logic
│   │   ├── types.go         # Core types and interfaces
│   │   └── orchestrator.go  # Task orchestration
│   ├── api/                 # HTTP/WebSocket server
│   │   └── server.go
│   ├── config/              # Configuration
│   │   └── config.go
│   ├── memory/              # Memory management
│   │   └── memory.go
│   ├── planner/             # LLM-based planner
│   │   └── planner.go
│   ├── storage/             # SQLite storage
│   │   ├── sqlite.go
│   │   ├── postgres.go      # Optional PostgreSQL support
│   │   └── schema.sql
│   └── tools/               # Tool implementations
│       └── shell.go         # Shell execution tool
├── web/                     # (Future) Web UI
├── data/                    # SQLite database location (created automatically)
├── .env.example             # Example environment variables
├── go.mod
└── README.md
```

## Security

- **Sandboxed Execution**: All shell commands run in a temporary directory
- **Timeout Protection**: 30-second default timeout for commands
- **Dangerous Command Blocking**: Blocks `rm -rf /`, `shutdown`, etc.
- **Token Authentication**: Simple Bearer token authentication
- **Command Logging**: All commands are logged to the database

## Development

### Running Tests

```bash
go test ./...
```

### Building

```bash
go build -ldflags="-s -w" -o bin/server ./cmd/server
```

## Configuration Options

| Variable | Default | Description |
|----------|---------|-------------|
| `HOST` | 0.0.0.0 | Server host |
| `PORT` | 8080 | Server port |
| `DATABASE_URL` | ./data/machinus.db | SQLite database file path |
| `LLM_BASE_URL` | https://api.z.ai/api/coding/paas/v4 | LLM API endpoint |
| `LLM_API_KEY` | - | Your API key |
| `LLM_MODEL` | glm-4.7 | Model to use |
| `AUTH_TOKEN` | dev-token | Authentication token |
| `MAX_EXECUTION_TIME` | 30s | Command timeout |
| `ENABLE_SANDBOX` | true | Enable sandboxing |
| `ENABLE_MEMORY` | true | Enable memory system |
| `MAX_MEMORIES` | 3 | Max memories to retrieve |

## Storage

By default, the app uses SQLite for local development. The database file is created at `./data/machinus.db`.

For production deployments, PostgreSQL support is also available. To use PostgreSQL:

1. Set `DATABASE_URL` to a PostgreSQL connection string
2. The app will automatically detect the PostgreSQL format and use the PostgreSQL storage backend

## License

MIT
