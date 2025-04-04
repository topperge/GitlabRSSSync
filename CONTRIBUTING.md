# Contributing to GitlabRSSSync

Thank you for considering contributing to GitlabRSSSync! This document outlines the guidelines for contributing to this project.

## Code of Conduct

By participating in this project, you agree to maintain a respectful and inclusive environment for everyone.

## How to Contribute

### Reporting Bugs

If you find a bug, please create an issue in the GitLab repository with the following information:

1. A clear, descriptive title
2. Description of the bug, including steps to reproduce
3. Expected behavior
4. Screenshots (if applicable)
5. Environment details (OS, Go version, Redis version, etc.)
6. Any additional context

### Suggesting Enhancements

To suggest an enhancement:

1. Create an issue with a clear title and detailed description
2. Explain the current behavior and how you'd like it to be improved
3. Provide examples of how the enhancement would benefit users
4. If possible, outline a potential implementation approach

### Pull Requests

1. Fork the repository
2. Create a new branch for your changes (`git checkout -b feature/your-feature-name`)
3. Make your changes
4. Run tests to ensure they pass (`go test -v ./...`)
5. Commit your changes with a descriptive commit message
6. Push to your branch (`git push origin feature/your-feature-name`)
7. Create a pull request to the main repository

### Pull Request Guidelines

- Follow the Go style guide and formatting conventions
- Include tests for new functionality
- Update documentation as needed
- Ensure all tests pass
- Keep PRs focused on a single change/feature

## Development Setup

1. Clone the repository
2. Install dependencies with `go mod download`
3. Set up Redis locally or use Docker
4. Copy `config.yaml.example` to `config.yaml` and customize
5. Set environment variables as described in the README
6. Run the application with `go run main.go`

## Testing

- Run tests with `go test -v ./...`
- Check test coverage with `go test -cover ./...`
- Run linting with `go vet ./...` and `go fmt ./...`

## CI/CD Pipeline

The project uses GitLab CI/CD for continuous integration and deployment. Every pull request will trigger:

1. Code linting and formatting checks
2. Unit tests with code coverage
3. A review deployment for feature branches

Merges to the main branch can trigger a production deployment (requires manual approval).

## Versioning

We use [Semantic Versioning](https://semver.org/) for version numbers:

- MAJOR version for incompatible API changes
- MINOR version for backward-compatible functionality
- PATCH version for backward-compatible bug fixes

## License

By contributing to this project, you agree that your contributions will be licensed under the project's license.

## Questions?

If you have any questions or need help, please create an issue in the GitLab repository.
