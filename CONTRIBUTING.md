# Contributing to Machinus

## Setup

```bash
# Clone the repository
git clone https://github.com/machinus/cloud-agent.git
cd cloud-agent

# Install dependencies
go mod download

# Run tests
go test ./...

# Build
go build ./cmd/machinus
```

## Development

### Code Style

- Follow standard Go conventions (`gofmt`)
- Keep functions focused and small
- Add tests for new features
- Document exported types and functions

### Project Structure

```
internal/
├── planner/      # Main agent orchestration
├── subagent/     # Subtask delegation
├── prompt/       # Dynamic prompt building
├── tools/        # Tool implementations
├── types/        # Shared types
└── ui/           # Terminal UI
```

### Adding Tools

Tools go in `internal/tools/`. Implement the `Tool` interface:

```go
type Tool interface {
    Name() string
    Description() string
    Execute(ctx context.Context, args map[string]any) ToolResult
}
```

### Testing

Run tests for a specific package:

```bash
go test ./internal/prompt -v
```

Run all tests:

```bash
go test ./...
```

## Submitting Changes

1. Fork the repository
2. Create a feature branch
3. Make your changes with tests
4. Run `go test ./...` and `go build ./...`
5. Submit a pull request

## Questions?

Open an issue or discussion for questions.
