# Contributing to Graphium

Thank you for your interest in contributing to Graphium! 🧬

## Getting Started

1. **Fork the repository**
2. **Clone your fork**
```bash
   git clone https://github.com/YOUR_USERNAME/graphium
   cd graphium
```

3. **Set up development environment**
```bash
   task dev:setup
```

## Development Workflow

### Making Changes

1. **Create a feature branch**
```bash
   git checkout -b feature/your-feature-name
```

2. **Make your changes**
   - Write clean, documented code
   - Follow Go best practices
   - Add tests for new features

3. **Test your changes**
```bash
   task test
   task lint
```

4. **Generate code**
```bash
   task generate
```

5. **Commit your changes**
```bash
   git add .
   git commit -m "feat: add awesome feature"
```

   We follow [Conventional Commits](https://www.conventionalcommits.org/):
   - `feat:` - New features
   - `fix:` - Bug fixes
   - `docs:` - Documentation changes
   - `test:` - Test additions or changes
   - `refactor:` - Code refactoring
   - `chore:` - Maintenance tasks

6. **Push to your fork**
```bash
   git push origin feature/your-feature-name
```

7. **Create a Pull Request**

## Code Style

- Run `task fmt` before committing
- Follow standard Go conventions
- Keep functions small and focused
- Write clear comments for complex logic

## Testing

- Write unit tests for new features
- Ensure all tests pass: `task test`
- Aim for >80% code coverage

## Pull Request Process

1. Update documentation if needed
2. Add tests for new features
3. Ensure CI passes
4. Request review from maintainers
5. Address review feedback

## Questions?

Open an issue or join our discussions!

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
