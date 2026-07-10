# API and Standard Library Usage Guidelines

## standard-ts-lib

When modifying `ws-client.ts` or its consumers, always use the utilities provided by `standard-ts-lib` to ensure consistent and robust error handling, as well as safe nullability.

### 1. Error Handling with `Result` and `StatusError`
- Do **not** use `try/catch` and standard Javascript `Error` exceptions for expected asynchronous failure scenarios.
- Do **not** use `WrapPromise` to wrap functions that should natively return a `Result`.
- Functions and APIs (such as `wsClient.request`) should return a `Promise<Result<T, StatusError>>`.
- When encountering an error, return `Err(new StatusError(...))` or use the provided creator functions like `UnknownError("message")`, `UnavailableError("message")`.
- When returning a success, return `Ok(value)`.
- Callers must inspect `.ok` (or `.err`) on the `Result` to safely unwrap or handle errors.

### 2. Nullability with `Optional`
- Do **not** use `T | null` or `T | undefined` for state properties or arguments where possible.
- Use `Optional<T>` from `standard-ts-lib` instead.
- Use `Some(value)` when the value is present.
- Use `None` when the value is absent.
- Safely check presence using methods provided by `Optional` (e.g., checking `.none` or `.some` properties).
