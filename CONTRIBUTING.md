# Contributing to Floq

We welcome contributions from the community! Whether it's bug reports, feature suggestions, documentation improvements, or code — every contribution matters.

## Code of Conduct

By participating in this project, you agree to maintain a respectful and inclusive environment. Be kind, constructive, and professional in all interactions.

## How to Contribute

### Reporting Bugs

1. Check [existing issues](https://github.com/VDV001/floq/issues) to avoid duplicates
2. Create a new issue with a clear title and detailed description
3. Include steps to reproduce, expected vs actual behavior, and environment details

### Suggesting Features

Open an issue with the `enhancement` label. Describe the problem you're solving, proposed solution, and any alternatives you considered.

### Submitting Code

1. **Fork** the repository
2. **Create a branch** from `main`: `git checkout -b feat/your-feature`
3. **Write code** following project conventions (see below)
4. **Write tests** — we require tests for all new backend logic
5. **Run checks locally**:
   ```bash
   # Backend
   cd backend && go test ./... && go vet ./...

   # Frontend
   cd frontend && npm run lint && npm run build
   ```
6. **Commit** using [Conventional Commits](https://www.conventionalcommits.org/): `feat: add prospect dedup`, `fix: correct CSV delimiter detection`
7. **Open a Pull Request** against `main`

### Code Style

**Backend (Go 1.26):**
- Clean Architecture: domain → usecase → repository → handler
- No cross-context imports between bounded contexts
- No business logic in handlers
- Domain invariants via factory methods (`NewLead`, `NewProspect`, etc.)
- Error wrapping: `fmt.Errorf("scope.Method: %w", err)`
- Table-driven tests in `*_test.go` next to source

**Frontend (Next.js 16, TypeScript):**
- Strict TypeScript, no `any` or `eslint-disable`
- Components per page in `src/components/{pagename}/`
- State + logic in custom hooks `src/hooks/use{PageName}.ts`
- Page files are thin orchestrators (~50-180 lines)

## Contributor License Agreement (CLA)

**Before your first Pull Request can be merged, you must sign our [CLA](CLA.md).**

### Why CLA?

Floq is licensed under AGPL-3.0. The CLA ensures that:

- You confirm you have the right to contribute the code
- The project maintainer retains the ability to relicense if needed (e.g., offering a commercial license alongside AGPL)
- Your contributions are properly attributed

### How to Sign

When you open your first Pull Request, a CLA bot will automatically ask you to sign. You only need to sign once — it covers all future contributions.

Alternatively, you can sign proactively by reading [CLA.md](CLA.md) and adding your name to the `CONTRIBUTORS` section via a Pull Request.

## Pull Request Guidelines

- Keep PRs focused — one feature or fix per PR
- Write a clear description of what and why
- Reference related issues (`Closes #123`)
- Ensure all CI checks pass
- Be responsive to review feedback

## Questions?

If you're unsure about anything, open an issue or start a discussion. We're happy to help!
