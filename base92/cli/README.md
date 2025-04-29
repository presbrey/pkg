# Base92 CLI

A command-line utility for encoding and decoding data using the URL-safe Base92 encoding scheme.

## Installation

```bash
go install github.com/presbrey/pkg/base92/cli@latest
```

Or build from source:

```bash
cd cli
go build -o base92
```

## Usage

### Encoding Data

Encode a file:
```bash
base92 encode myfile.txt > encoded.txt
```

Encode from stdin:
```bash
echo "Hello World!" | base92 encode
```

### Decoding Data

Decode a file:
```bash
base92 decode encoded.txt > decoded.txt
```

Decode from stdin:
```bash
echo "kF%!t%MqcjTu=YZ" | base92 decode
```

## Features

- URL-safe Base92 encoding and decoding
- Support for file and stdin/stdout operations
- Compact and efficient representation of binary data
