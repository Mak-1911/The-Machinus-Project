# PinchTab Setup Guide

Machinus now uses **PinchTab** for browser automation - a lightweight, token-efficient solution for AI agent browser control.

## Why PinchTab?

- **Token-efficient**: ~800 tokens/page vs 10,000 for screenshots (5-13x cheaper)
- **Accessibility-first**: Stable element refs instead of fragile CSS selectors
- **Multi-instance**: Run parallel isolated Chrome processes
- **Persistent sessions**: Stay logged in across restarts
- **Lightweight**: 12MB binary, no external dependencies

## Quick Start

### 1. Install PinchTab

**macOS / Linux:**
```bash
curl -fsSL https://pinchtab.com/install.sh | bash
```

**npm:**
```bash
npm install -g pinchtab
```

**Docker:**
```bash
docker run -d -p 9867:9867 --name pinchtab pinchtab/pinchtab
```

**Windows (WSL):**
```bash
# Use npm or Docker on Windows
npm install -g pinchtab
```

### 2. Use Machinus (Auto-Start!)

**That's it!** Machinus will automatically start PinchTab when needed:

```
Navigate to https://example.com and extract text
```

**No manual setup required** - PinchTab starts on-demand! 🚀

### Optional: Manual Start

If you prefer to start PinchTab manually:

**Terminal 1 - Start PinchTab:**
```bash
pinchtab
# Or specify custom port:
# pinchtab --port 9867
```

You should see:
```
PinchTab server listening on http://localhost:9867
```

### 3. Configure Machinus (Optional)

**Option A: Environment Variable (Recommended)**
```bash
export PINCHTAB_URL=http://localhost:9867
./bin/machinus
```

**Option B: Default URL**
Machinus defaults to `http://localhost:9867` if `PINCHTAB_URL` is not set.

**Option C: Custom URL**
```bash
export PINCHTAB_URL=http://192.168.1.100:9867
./bin/machinus
```

## Using Browser Automation

### Basic Example

```
Navigate to https://example.com and take a screenshot
```

### Advanced Example

```
Create a browser instance, navigate to https://github.com,
take a snapshot of interactive elements, then extract the text content
```

### Supported Actions

| Action | Description | Parameters |
|--------|-------------|------------|
| `create_instance` | Create new browser instance | `profile` (optional) |
| `navigate` / `goto` | Navigate to URL | `url` (required) |
| `snapshot` | Get page structure | `filter` (optional: "interactive", "all", "visible") |
| `click` | Click element | `ref` (required) |
| `fill` | Fill form field | `ref`, `value` (required) |
| `text` | Extract text content | None |
| `screenshot` | Take screenshot | `path` (optional, default: "screenshot.png") |
| `close` | Close instance | None |

## Architecture

```
┌─────────────┐         HTTP API          ┌──────────────┐
│  Machinus   │ ◄────────────────────────► │  PinchTab    │
│  (Go)       │    http://localhost:9867    │   (Go)       │
└─────────────┘                            └──────────────┘
                                                   │
                                                   ▼
                                           ┌──────────────┐
                                           │   Chrome     │
                                           │  (CDP)       │
                                           └──────────────┘
```

## Auto-Start Feature

Machinus automatically starts PinchTab when browser automation is needed:

1. **Detection**: Checks if PinchTab is running at `http://localhost:9867`
2. **Auto-start**: If not running, automatically starts PinchTab in background
3. **Ready wait**: Waits up to 10 seconds for PinchTab to be ready
4. **Execution**: Proceeds with browser automation

**You'll see:**
```
🔄 PinchTab not detected, starting automatically...
✅ PinchTab started successfully
```

**Benefits:**
- No manual setup required
- PinchTab only runs when needed (saves resources)
- Automatic recovery if PinchTab crashes
- Transparent to the user

**Cross-platform support:**
- Windows: Background process via `cmd /c start /b`
- macOS/Linux: Standard background process
- Docker: Use manual start (see below)

## Token Savings Example

**Screenshot approach (old):**
- Navigate: ~500 tokens
- Screenshot: ~10,000 tokens (vision model)
- **Total: ~10,500 tokens**

**Text extraction (new):**
- Navigate: ~500 tokens
- Extract text: ~800 tokens
- **Total: ~1,300 tokens**

**Savings: 8,200 tokens (78% reduction!)**

## Profiles & Persistence

```bash
# Create instance with named profile
Create a browser instance with profile "work"

# Navigate to login page
Navigate to https://example.com/login

# Fill login form
Fill the email field with user@example.com and password with secret123

# Submit form
Click the login button

# Extract data
Extract all text from the page

# Next time - you're still logged in!
Navigate to https://example.com/dashboard
```

## Troubleshooting

### "pinchtab not found"

**Problem:** Auto-start failed because PinchTab isn't installed.

**Solution:** Install PinchTab:
```bash
npm install -g pinchtab
```

### "Pinchtab binary not found"

**Problem:** PinchTab is installed but the binary has the wrong name (common on Windows).

**Solution:** Copy the binary to the expected location:
```bash
# Find the actual binary
ls ~/.pinchtab/bin/*/

# Copy it to the expected name
cp ~/.pinchtab/bin/*/pinchtab-windows-amd64.exe ~/.pinchtab/bin/pinchtab-windows-x64.exe
```

Or rebuild:
```bash
npm rebuild pinchtab
```

### "Failed to connect to PinchTab"

**Problem:** Machinus can't reach PinchTab server (auto-start failed).

**Solutions:**
1. **Install PinchTab**: `npm install -g pinchtab`
2. **Manual start**: `pinchtab` (in a separate terminal)
3. **Check the URL**: `curl http://localhost:9867`
4. **Verify firewall** isn't blocking port 9867
5. **Custom URL**: Set `PINCHTAB_URL` if using custom port

### "No active instance"

**Problem:** Browser automation without creating instance first.

**Solution:** Always start with `create_instance` action:
```
Create a browser instance and navigate to https://example.com
```

### Element not found

**Problem:** Using old CSS selectors.

**Solution:** Use `snapshot` to get accessibility refs:
```
Navigate to https://example.com
Get a snapshot of interactive elements
Click element with ref e5
```

## Docker Deployment

### Run PinchTab in Docker

```bash
# Start PinchTab container
docker run -d \
  --name pinchtab \
  -p 9867:9867 \
  pinchtab/pinchtab

# Configure Machinus to use it
export PINCHTAB_URL=http://localhost:9867
```

### Docker Compose (Machinus + PinchTab)

```yaml
version: '3.8'
services:
  pinchtab:
    image: pinchtab/pinchtab
    ports:
      - "9867:9867"

  machinus:
    build: .
    environment:
      - PINCHTAB_URL=http://pinchtab:9867
    depends_on:
      - pinchtab
    ports:
      - "8080:8080"
```

## Performance Tips

1. **Use text extraction instead of screenshots** when possible
2. **Reuse instances** instead of creating new ones
3. **Use profiles** for persistent sessions
4. **Filter snapshots** with `interactive` to reduce noise
5. **Close instances** when done to free resources

## Next Steps

- Read [PinchTab Documentation](https://pinchtab.com/docs)
- Check [API Reference](https://pinchtab.com/docs/api-reference)
- Explore [Advanced Examples](https://pinchtab.com/docs/examples)

## Migration from Playwright

The old Playwright browser tool has been backed up to `internal/tools/browser.go.bak`.

**Key differences:**
- **CSS selectors** → **Accessibility refs** (more stable)
- **Screenshots** → **Text extraction** (cheaper)
- **Single instance** → **Multi-instance** (scalable)
- **No persistence** → **Profile persistence** (better UX)

**Action mapping:**
| Playwright | PinchTab |
|------------|----------|
| `goto` | `navigate` |
| `click` (selector) | `click` (ref) |
| `fill` (selector) | `fill` (ref) |
| `text` (selector) | `text` (whole page) |
| `screenshot` | `screenshot` |
| - | `create_instance` (new) |
| - | `snapshot` (new) |
