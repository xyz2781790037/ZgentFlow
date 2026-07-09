# ZealRAG DocReader

DocReader is ZealRAG's local document parsing service. It is started automatically by:

```bash
make dev-app
```

The development launcher creates a project-local Python environment, installs the locked dependencies, and starts the gRPC service on `127.0.0.1:50061` by default. No Docker service or object storage is required.

## Supported Input

The service handles the document formats exposed by the ZealRAG upload interface, including PDF, Office documents, Markdown, text, HTML, EPUB, CSV, JSON, web archives, and images. Audio and video are not accepted.

Original files and extracted assets are stored under `.local-data/files`. Runtime packages and the Python environment are stored under `.runtime` and are reused across starts.

## Optional Parsers

Built-in parsing works without external services. MinerU and PaddleOCR-VL endpoints can be configured when those engines are needed. Parser overrides are managed by the backend configuration and passed to DocReader per request.

## Development

Run the Python tests from the repository root:

```bash
docreader/.venv/bin/python -m unittest discover -s docreader/tests -p 'test_*.py'
```

Run the Go gRPC client tests with:

```bash
go test ./docreader/client
```

ZealRAG supports Linux x86_64/amd64 for the local development workflow.
