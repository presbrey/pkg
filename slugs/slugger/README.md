# Slug Command

A command-line interface for generating URL-safe slugs with various options and modes.

## Installation

```bash
go install github.com/presbrey/pkg/slugs/cmd/slug@latest
```

Or build from source:

```bash
cd slugs/cmd/slug
go build
```

## Usage

```
slug [options] text
```

If no text is provided, you must specify one of the random slug modes (`-u`, `-n`, or `-r`).

## Options

All options use single-character flags:

| Flag | Description | Default |
|------|-------------|---------|
| `-l` | Maximum length of the generated slug | 100 |
| `-d` | Character used to separate words in the slug | `-` |
| `-c` | Convert slug to lowercase | `true` |
| `-s` | Remove common stop words | `false` |
| `-w` | Additional stop words to remove (comma-separated) | `` |
| `-p` | Prefix to add to the slug | `` |
| `-x` | Suffix to add to the slug | `` |
| `-u` | Generate UUID-based slug | `false` |
| `-n` | Generate NanoID-style slug | `false` |
| `-r` | Generate random string slug | `false` |
| `-e` | Length of random slugs | `8` |

## Examples

Generate a text-based slug:
```bash
slug "Hello World Example"
# Output: hello-world-example
```

Generate a UUID-based slug:
```bash
slug -u
# Output: _oc5-3bj0t9tqgmq1kidkq
```

Generate a NanoID-style slug:
```bash
slug -n
# Output: hHWZu_ZW
```

Generate a random slug with custom length:
```bash
slug -r -e 12
# Output: a random 12-character slug
```

Generate a slug with custom delimiter:
```bash
slug -d "_" "Hello World Example"
# Output: hello_world_example
```

Add a prefix and suffix:
```bash
slug -p "prefix" -x "suffix" "Hello World"
# Output: prefix-hello-world-suffix
```

Remove stop words:
```bash
slug -s "The quick brown fox jumps over the lazy dog"
# Output: quick-brown-fox-jumps-over-lazy-dog
```

Add custom stop words:
```bash
slug -s -w "quick,brown,fox" "The quick brown fox jumps over the lazy dog"
# Output: jumps-over-lazy-dog
```
