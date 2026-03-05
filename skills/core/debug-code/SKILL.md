---
name: debug-code
description: Systematic debugging workflow for identifying and fixing code issues
---

# Code Debugging Skill

## Purpose
Provide a structured approach to debugging code problems efficiently.

## When to Use
Use this skill when:
- Code produces errors or unexpected behavior
- Tests are failing
- Features aren't working as expected
- User asks to "debug", "fix", or "troubleshoot" code

## Prerequisites
- File tools (read_file, glob, grep)
- Shell tool for running tests/builds
- Understanding of the codebase structure

## Workflow

### Step 1: Understand the Problem
Ask clarifying questions:
- What error are you seeing?
- What should happen vs what is actually happening?
- When did this start happening?
- What changed recently?

### Step 2: Locate the Relevant Code
Use glob to find related files:
```json
{
  "pattern": "**/*.go"
}
```

Use grep to search for error messages or function names:
```json
{
  "pattern": "error message here",
  "path": "."
}
```

### Step 3: Read the Code
Use read_file to examine the relevant code:
```json
{
  "file_path": "path/to/file.go"
}
```

### Step 4: Identify the Root Cause
Analyze the code to find:
- Logic errors
- Typos or syntax issues
- Missing imports
- Incorrect API usage
- Race conditions or timing issues
- Resource leaks

### Step 5: Propose a Fix
Explain:
- What the problem is
- Why it's happening
- How to fix it
- Show the corrected code

### Step 6: Verify the Fix
- Make the code change
- Run tests
- Check if the error is resolved
- Look for any side effects

## Debugging Strategies

### Strategy 1: Binary Search
If you have a large codebase:
1. Identify the area where the bug might be
2. Add logging/checkpoints
3. Narrow down the location
4. Repeat until found

### Strategy 2: Reproduce the Bug
1. Create a minimal reproduction case
2. Isolate the problem
3. Test potential fixes

### Strategy 3: Check Recent Changes
1. Look at git history
2. Check recent commits
3. Identify what changed
4. Revert suspicious changes

### Strategy 4: Add Logging
1. Add log statements at key points
2. Run the code
3. Examine the logs
4. Trace the execution flow

## Common Issues

### Compilation Errors
- Missing imports
- Syntax errors
- Type mismatches
- Undefined variables

### Runtime Errors
- Null pointer exceptions
- Index out of bounds
- Division by zero
- Failed assertions

### Logic Errors
- Wrong conditional logic
- Off-by-one errors
- Incorrect algorithm
- Missing edge cases

### Performance Issues
- Infinite loops
- Memory leaks
- Inefficient algorithms
- Excessive I/O

## Example

User: "The user registration isn't working"

You:
1. **Understand**: What specific error? Is it crashing or just not working?
2. **Locate**: Find registration code using glob and grep
3. **Read**: Examine the registration handler
4. **Identify**: Database connection string is incorrect
5. **Propose**: Update the connection string
6. **Verify**: Test registration again

## Best Practices

✅ **DO:**
- Read error messages carefully
- Reproduce the bug before fixing
- Make one change at a time
- Test after each fix
- Document the fix

❌ **DON'T:**
- Guess without investigating
- Make multiple changes at once
- Skip testing
- Ignore error messages
- Fix symptoms instead of root cause

## Tools for Debugging

- **read_file**: Examine source code
- **grep**: Search for specific code patterns
- **glob**: Find files by pattern
- **shell**: Run tests, build, or execute commands
- **list**: Check directory structure
- **fileinfo**: Get file information

## Troubleshooting

**Can't find the code:**
- Use glob to search for file types (*.go, *.js, etc.)
- Use grep to search for function names
- Check common directories (src/, lib/, cmd/)

**Error message isn't clear:**
- Search for the error text in the codebase
- Look for stack traces
- Check logs
- Add more logging

**Fix doesn't work:**
- Verify the fix was applied
- Check for caching issues
- Look for other places with similar code
- Consider if it's a different issue
