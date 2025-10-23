# Contributing to envtree

Thank you for considering contributing to envtree! This document provides guidelines and instructions for contributing.

## Code of Conduct

Be respectful, inclusive, and considerate of others. We're all here to make great software together.

## How to Contribute

### Reporting Bugs

If you find a bug, please create an issue with:
- A clear, descriptive title
- Steps to reproduce the issue
- Expected behavior
- Actual behavior
- Go version and OS information
- Any relevant code samples or error messages

### Suggesting Features

Feature suggestions are welcome! Please create an issue describing:
- The problem your feature would solve
- How you envision it working
- Any alternatives you've considered
- Whether you'd be willing to implement it

### Pull Requests

1. **Fork the repository** and create your branch from `main`
2. **Write tests** for any new functionality
3. **Update documentation** if you're changing behavior or adding features
4. **Follow the coding style** of the project
5. **Run tests and linters** before submitting
6. **Write clear commit messages** explaining what and why

#### Development Setup

```bash
# Clone your fork
git clone https://github.com/yourusername/envtree.git
cd envtree

# Install dependencies
make install

# Run tests
make test

# Run linter
make lint

# Run all checks
make check
```

#### Testing

All new code should have tests. Run the test suite with:

```bash
make test           # Basic tests
make test-race      # With race detector
make test-cover     # With coverage report
```

Aim for good test coverage (80%+) for new code.

#### Code Style

- Follow standard Go conventions
- Use `gofmt` to format your code (`make fmt`)
- Run `go vet` to check for common mistakes (`make vet`)
- Use `golangci-lint` if available (`make lint`)
- Write clear, self-documenting code
- Add comments for exported functions and types
- Keep functions focused and small

#### Commit Messages

Use clear, descriptive commit messages:

```
Add support for custom env file names

- Add EnvFileName field to Config struct
- Update docs with usage examples
- Add tests for custom file names
```

## Project Structure

```
envtree/
├── envtree.go        # Main library code
├── envtree_test.go   # Tests
├── doc.go              # Package documentation
├── examples/           # Usage examples
│   └── main.go
├── README.md           # User documentation
├── CONTRIBUTING.md     # This file
├── LICENSE             # MIT license
├── Makefile            # Development tasks
├── go.mod              # Go module definition
└── .gitignore         # Git ignore rules
```

## Development Workflow

1. **Create an issue** describing what you want to work on
2. **Fork the repository** and create a feature branch
3. **Write your code** with tests and documentation
4. **Run checks**: `make check`
5. **Commit your changes** with clear messages
6. **Push to your fork** and create a pull request
7. **Respond to feedback** during code review

## Code Review Process

All submissions require review. We aim to provide feedback within a few days. We look for:

- **Correctness**: Does the code work as intended?
- **Testing**: Are there adequate tests?
- **Documentation**: Is the code well-documented?
- **Style**: Does it follow Go best practices?
- **Compatibility**: Does it maintain backward compatibility?

## Questions?

Feel free to create an issue with your question, or reach out to the maintainers.

## Recognition

All contributors will be recognized in the project. Thank you for making envtree better!