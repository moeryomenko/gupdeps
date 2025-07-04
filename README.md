# gupdeps

`gupdeps` is a Go tool for intelligently analyzing and updating Go dependencies. It examines commit messages and changes in your dependencies to determine which updates are safe to apply automatically.

## Features

- **Smart Analysis**: Analyzes commit messages to identify bug fixes, performance improvements, and breaking changes
- **Interactive Mode**: Review and approve updates one-by-one
- **Automatic Mode**: Apply all safe updates at once
- **Git Integration**: Examines commit history to make informed update decisions

## Installation

```bash
# Install directly using go install
go install github.com/moeryomenko/gupdeps/cmd/gupdeps@latest

# Or clone the repository
git clone https://github.com/moeryomenko/gupdeps.git
cd gupdeps
go install ./cmd/gupdeps
```

Make sure your Go bin directory is in your PATH.

## Usage

### Basic Usage

```bash
# Run in current directory
gupdeps

# Specify a different project directory
gupdeps -path /path/to/go/project
```

### Interactive Mode

Interactive mode allows you to review and approve each update individually:

```bash
gupdeps -interactive
```

### Verbose Output

For more detailed logging:

```bash
gupdeps -verbose
```

### Help Information

```bash
gupdeps -help
```

## How It Works

`gupdeps` performs the following steps:

1. Reads your `go.mod` file to identify direct dependencies
2. Checks for available updates
3. For each update, analyzes the commit history:
   - Identifies fixes, performance improvements, and potential breaking changes
   - Makes recommendations based on the analysis
4. Either:
   - Automatically applies "safe" updates
   - In interactive mode, presents the analysis for each dependency for your review

## Update Analysis Logic

Updates are categorized based on commit message patterns:

- **Automatic approval**:
  - Bug fixes (identified by keywords like "fix", "bug", "patch")
  - Performance improvements (identified by keywords like "perf", "optimize")
  - New features (identified by keywords like "feat", "feature", "add")

- **Manual review required**:
  - Breaking changes (identified by keywords like "breaking", "break", "remove")

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is available under dual licensing terms:

- [MIT License](LICENSE-MIT)
- [Apache License, Version 2.0](LICENSE-APACHE)

Choose the license that best suits your needs.
