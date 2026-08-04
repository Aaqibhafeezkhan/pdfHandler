# PDF Handler

A self-hosted PDF toolkit. One Go binary serves both the JSON API and a
self-contained web UI — no database, no object storage, no external service.
Files are processed in a temp directory and deleted when the response is sent.

Built for a homelab Kubernetes cluster, but it runs anywhere Docker does.

## Features

| Operation | What it does |
|---|---|
| **Merge** | Combine several PDFs into one |
| **Split** | Break a PDF into chunks of N pages, returned as a zip |
| **Compress** | Shrink a PDF at four quality levels (screen → prepress) |
| **Rotate** | Turn all pages or a selection by a multiple of 90° |
| **PDF → image** | Render pages to PNG or JPG at 72/150/300 dpi, returned as a zip |
| **Image → PDF** | Combine JPG/PNG/TIFF/WebP images into one PDF, one per page |
| **Protect** | Encrypt a PDF with a password |
| **Unlock** | Remove password protection |
| **Preview** | Page count plus base64 thumbnails, used by the UI |

## Quick start

```bash
docker run --rm -p 8080:8080 r4faelrs/pdfhandler:latest
```

Open <http://localhost:8080>.

### Kubernetes

```bash
kubectl apply -f k8s/
```

This creates the `pdfhandler-ns` namespace, a Deployment, a NodePort Service on
`30880`, and an nginx Ingress for `pdf.home.lab`. Adjust the hostname and the
image tag in `k8s/deployment.yaml` to match your cluster.

### Building from source

Requires Go 1.25+, plus `ghostscript` and `poppler-utils` on `PATH`:

```bash
go build -o pdfhandler .
./pdfhandler
```

Or build the container, which bundles both tools:

```bash
docker build -t pdfhandler .
```

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | Port to listen on |
| `GIN_MODE` | `debug` | Set to `release` outside development |
| `HOME` | — | pdfcpu resolves its config dir from this; the image sets it to `/tmp` so it works as a non-root user |

## API

All endpoints take `multipart/form-data` and return the resulting file as an
attachment, except `/api/preview` (JSON) and `/healthz`.

| Endpoint | Form fields | Returns |
|---|---|---|
| `POST /api/merge` | `files[]` (2–50 PDFs) | `merged.pdf` |
| `POST /api/split` | `file`, `span` (pages per chunk) | `split.zip` |
| `POST /api/compress` | `file`, `quality` (`screen`\|`ebook`\|`printer`\|`prepress`) | `compressed.pdf` |
| `POST /api/rotate` | `file`, `degrees` (multiple of 90), `pages` (`all` or `1,3,5-7`) | `rotated.pdf` |
| `POST /api/pdf-to-image` | `file`, `format` (`png`\|`jpg`), `dpi` (`72`\|`150`\|`300`) | `images.zip` |
| `POST /api/image-to-pdf` | `files[]` (1–100 images) | `output.pdf` |
| `POST /api/protect` | `file`, `password` | `protected.pdf` |
| `POST /api/unlock` | `file`, `password` | `unlocked.pdf` |
| `POST /api/preview` | `file`, `limit` (max 50) | JSON: page count + thumbnails |
| `GET /healthz` | — | `{"status":"ok"}` |

Errors come back as `{"error": "..."}` with a 4xx or 5xx status.

```bash
curl -F "files[]=@a.pdf" -F "files[]=@b.pdf" \
     -o merged.pdf http://localhost:8080/api/merge

curl -F "file=@big.pdf" -F "quality=screen" \
     -o small.pdf http://localhost:8080/api/compress
```

## Security

Every endpoint hands an untrusted file to ghostscript, poppler or pdfcpu, so
requests are narrowed before any of that runs:

- Uploads must start with the `%PDF-` magic bytes; image uploads must carry an
  allow-listed extension, derived from `filepath.Base` and pattern-matched
  rather than taken from `filepath.Ext` directly.
- Parameters are validated and rejected, not coerced.
- The request body is capped at 100 MB via `http.MaxBytesReader`; oversize
  uploads get a 413.
- Each ghostscript/poppler run is bounded by a 90 s timeout tied to the request
  context, and ghostscript runs with `-dSAFER`.
- Tool output and pdfcpu errors go to the log, never the response — they embed
  temp paths and fragments of the uploaded document.
- Responses carry a CSP and the usual hardening headers; `POST` responses are
  `no-store`.
- The container runs as a non-root user with a read-only root filesystem, all
  capabilities dropped, and no ServiceAccount token.

`go build` is checked against `govulncheck` — no known advisories reach this
code outside the standard library.

### Known limitations

- **No authentication.** Anyone who can reach the port can use it. Put it behind
  your ingress' auth, a VPN, or a trusted network.
- **The sample Ingress serves plain HTTP.** `/api/protect` and `/api/unlock`
  carry passwords in the request body, so terminate TLS before exposing it
  beyond a trusted LAN — `k8s/ingress.yaml` has a commented block for this.
- **No rate limiting.** Per-request timeouts and the CPU limit bound each job,
  but many concurrent compressions can still saturate a node.
- **No automated tests.** Changes are verified by hand.

## How it works

Gin routes both the API and the static UI from a single binary. Operations that
pdfcpu handles natively (merge, split, rotate, encrypt, image import) run
in-process; compression shells out to ghostscript and rasterisation to poppler's
`pdftoppm`, since neither has a good pure-Go equivalent.

See [DESIGN.md](DESIGN.md) for the reasoning behind those choices, the
alternatives considered, and the Kubernetes design.

```
main.go              Routing, middleware, server timeouts
handlers/            One file per operation; util.go holds the shared
                     validation, sandboxing and error helpers
static/index.html    The entire UI — HTML, CSS and JS in one file
k8s/                 Namespace, Deployment, Service, Ingress
```

## Built with

[Gin](https://github.com/gin-gonic/gin) ·
[pdfcpu](https://github.com/pdfcpu/pdfcpu) ·
[Ghostscript](https://www.ghostscript.com/) ·
[Poppler](https://poppler.freedesktop.org/)

## License

MIT — see [LICENSE](LICENSE).
