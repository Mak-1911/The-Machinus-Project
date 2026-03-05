---
name: clone-website
description: Create exact replicas of websites by downloading all HTML, CSS, JavaScript, and assets without modifications
---

# Website Cloning Skill

## Purpose
Download complete, working copies of websites for offline use, backup, or analysis.

## When to Use
Use this skill when the user asks to:
- Clone, copy, or replicate a website
- Download a website for offline viewing
- Create a local backup of a web page
- Mirror a website

## Prerequisites
- HTTP tool for downloading files
- File tools (read_file, write_file) for saving content
- Grep tool for extracting URLs
- Sufficient disk space

## Workflow

### Step 1: Download HTML
Use the HTTP tool to download the main HTML file:
```json
{
  "url": "https://example.com",
  "method": "GET"
}
```

Save the response to a file using write_file.

### Step 2: Detect if This is a Single Page Application (SPA)
Check the HTML file for SPA markers:
```json
{
  "pattern": "(%sveltekit.body%|__nuxt__|__NEXT_DATA__|<div id=\"app\"></div>|<div id=\"root\"></div>)",
  "path": "downloaded/index.html"
}
```

**If SPA markers are found:**
- Inform the user: "⚠️ This appears to be a Single Page Application (SPA). SPAs require JavaScript to render content and cannot be fully cloned as static files."
- Ask which approach they want:
  1. Download all files anyway (won't work without browser)
  2. Use browser tool to extract rendered content
  3. Take screenshots for visual reference

**If no SPA markers (traditional website):**
- Continue with the cloning workflow

### Step 3: Extract Asset URLs
Use grep to find CSS, JS, and image references in the HTML:

```json
{
  "pattern": "href=\"(.*\\.css)\"",
  "path": "downloaded/index.html"
}
```

```json
{
  "pattern": "src=\"(.*\\.(png|jpg|jpeg|gif|svg|webp|js))\"",
  "path": "downloaded/index.html"
}
```

### Step 4: Download All Assets
For each asset URL found, download using the HTTP tool:

```json
{
  "url": "https://example.com/css/style.css",
  "method": "GET"
}
```

Save each asset to the appropriate directory structure.

### Step 5: Fix Relative Paths
Use edit_file to update HTML paths from remote to local:

```json
{
  "file_path": "downloaded/index.html",
  "old_string": "href=\"/css/style.css\"",
  "new_string": "href=\"./css/style.css\""
}
```

### Step 6: Verify the Clone
Check that:
- All files are downloaded
- Paths are correct
- Opening index.html shows the site

## Important Notes

### ⚠️ DO NOT Use Browser Tool
- Browser snapshots create massive context (1000+ elements)
- This causes timeouts
- Use HTTP tool instead for downloading

### ⚠️ DO NOT Read Files Into Context
- Reading HTML into context causes timeouts
- Use grep with file_path parameter to search for patterns
- Keep content out of conversation context

### ⚠️ SPAs Cannot Be Fully Cloned
Single Page Applications (SPAs) have limitations:
- HTML is just a skeleton (3KB)
- Content is rendered by JavaScript
- Requires browser runtime to function
- Even if you download all files, they won't work via file:// protocol

For SPAs, recommend:
1. Use goclone (a Go tool with headless browser support)
2. Use browser tool for screenshots (visual reference only)
3. Use browser tool for text extraction (content reference only)

## Example

User: "Clone https://example.com"

You:
1. Download HTML from https://example.com
2. Save to example-com-clone/index.html
3. Check for SPA markers
4. Extract CSS/JS/image URLs
5. Download all assets
6. Fix relative paths
7. Summarize: "Cloned example.com to example-com-clone/ with all assets"

## Troubleshooting

**Site doesn't look right:**
- Check all CSS files were downloaded
- Verify CSS file paths are correct
- Check for missing fonts

**Missing images:**
- Check HTML for `<img>` tags
- Search CSS for `background-image:`
- Verify image files were downloaded

**Broken styles:**
- Verify ALL CSS files downloaded
- Check CSS file loading order
- Look for missing @import statements

**Blank page:**
- Site is likely an SPA
- Check for SPA markers in HTML
- Inform user of limitations
