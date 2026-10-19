# Contributing to PDF Handler

Contributions are welcome: bug reports, feature ideas, documentation fixes,
tests, and code changes. For installation and usage, see the [README](README.md).

## Reporting issues

Search [existing issues](https://github.com/rafaelsilvabr/pdfHandler/issues)
before opening a new one. If the problem is already reported, add useful
details to that discussion.

For a [bug report](https://github.com/rafaelsilvabr/pdfHandler/issues/new), include:

- A clear description of the problem and the operation affected.
- Steps to reproduce it, including the API request or UI actions.
- Expected behavior and what actually happened.
- The release version or commit, installation method, operating system, and
  browser when relevant.
- Relevant configuration, error messages, and logs with sensitive data removed.
- A small sample file or screenshot when it helps reproduce the problem.

Use synthetic or anonymized documents. Do not attach private PDFs, passwords,
tokens, or other sensitive information.

For a feature request, describe the task you want to accomplish, the current
limitation, and an example of the behavior you propose. Discuss larger changes
in an issue before starting implementation so the scope can be agreed on.

### Security reports

Report vulnerabilities through GitHub's [private vulnerability reporting](https://github.com/rafaelsilvabr/pdfHandler/security/advisories/new),
rather than a public issue or pull request. Include reproduction steps and the
affected version without exposing real documents or credentials.

## Development setup

You need Git and Go 1.26 or newer. Install Ghostscript and Poppler so their
commands (`gs`, `pdftoppm`, and `pdfinfo`) are on `PATH`. The container smoke
test also needs Docker, Bash, and curl. On Windows, WSL provides a Linux
environment for the shell commands and external tools below.

Fork the repository on GitHub, clone your fork, and create a branch:

```bash
git clone https://github.com/YOUR_USERNAME/pdfHandler.git
cd pdfHandler
git switch -c docs/describe-your-change
go mod download
go run .
```

Choose a branch name that describes your change. Open <http://localhost:8080>.
The API and static UI run in the same process; there is no frontend build step.

The main project areas are:

| Location | Purpose |
|---|---|
| `main.go` | Routes, middleware, and server setup |
| `config.go` | Application configuration |
| `handlers/` | PDF operations, shared helpers, and handler tests |
| `static/` | Web UI markup, styles, and JavaScript |
| `scripts/smoke.sh` | Functional checks against a running instance |
| `k8s/` | Example Kubernetes deployment |
| `.github/workflows/` | CI and repository automation |

See [DESIGN.md](DESIGN.md) for architecture and implementation choices.

## Checking your changes

For Go changes, format the files you edited and run:

```bash
gofmt -l .        # must print nothing
go vet ./...
go build ./...
go test -race -cover ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

The race detector requires CGO and a supported C compiler. If your environment
cannot run a check, state that limitation in the pull request.

Go tests cover validation, handler output, and middleware. PDF fixtures are
generated at test time. Tests that need Ghostscript or Poppler skip when the
tools are missing, so install both to exercise those operations.

For handler, Dockerfile, or bundled-tool changes, also run the functional smoke
test. From a Bash shell, with Ghostscript, Poppler, and curl installed:

```bash
docker build -t pdfhandler:contrib .
docker run -d --name pdfhandler-contrib -p 8080:8080 \
  --read-only --tmpfs /tmp:rw,size=2g \
  --user 1001:1001 --cap-drop ALL \
  --security-opt no-new-privileges pdfhandler:contrib
bash scripts/smoke.sh http://127.0.0.1:8080
docker rm -f pdfhandler-contrib
```

Remove the test container after the smoke test, including when a check fails.
These checks exercise the image under the same non-root and filesystem
constraints as the example deployment.

For UI changes, check the affected workflow in a browser at desktop and mobile
widths, including keyboard navigation, validation, loading, error, and download
states. Documentation-only changes need a check of links, examples, and Markdown
formatting.

[CI](.github/workflows/ci.yml) runs build and test checks, govulncheck, and
container functional tests on pull requests targeting `main` and pushes to
`main` or version tags. These three CI jobs must pass before merging.

## Implementation guidelines

- Validate parameters and return a clear client error for invalid input.
- Name temporary files yourself; never build paths from client-supplied names.
  Use the existing upload and extension-validation helpers in `handlers/util.go`.
- Keep library errors and tool output in server logs; return fixed public error
  messages through the shared error helpers.
- Bound processing time, input counts, and generated output. Clean up temporary
  files when the request finishes.
- Explain the reason for a change in comments and commit messages when it is
  not clear from the code.

When adding a PDF operation:

1. Add its handler in `handlers/`, reusing helpers such as `saveUploadedPDF`,
   `runTool`, `failInternal`, `failBadRequest`, and `failUpload` where applicable.
2. Register the route in `newRouter` in `main.go`.
3. Add validation and output tests in `handlers/`, plus a functional check in
   `scripts/smoke.sh`.
4. Add the tool definition and icon in `static/app.js`, its controls in
   `static/index.html`, and styles in `static/styles.css` where needed.
5. Update the feature list and API documentation in the README.

## Opening a pull request

1. Keep the change focused on one problem or feature. Add regression coverage
   for bug fixes and tests for new behavior.
2. Update documentation when behavior, configuration, or API fields change.
3. Run the relevant checks above and review your diff for unrelated changes,
   generated files, and sensitive data.
4. Commit and push your branch to your fork, then open a
   [pull request](https://github.com/rafaelsilvabr/pdfHandler/compare) targeting
   `main`. Draft pull requests are welcome for work in progress.
5. Give the PR a descriptive title. Explain the problem, resulting behavior,
   and any compatibility implications; link related issues and list the checks
   you ran, including failures, skipped checks, or environment limitations.
   Include screenshots for visible UI changes when useful.
6. Address review feedback and check the CI results before requesting another
   review.

Contributions are distributed under the project's [MIT license](LICENSE).
