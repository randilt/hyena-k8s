#!/bin/bash
# Generate shares for a secret using the split-secret tool

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Default values
SECRET=""
PARTS=5
THRESHOLD=3
OUTPUT_DIR="$PROJECT_ROOT/shares"

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --secret)
      SECRET="$2"
      shift 2
      ;;
    --parts)
      PARTS="$2"
      shift 2
      ;;
    --threshold)
      THRESHOLD="$2"
      shift 2
      ;;
    --output)
      OUTPUT_DIR="$2"
      shift 2
      ;;
    *)
      echo "Unknown option: $1"
      echo "Usage: $0 --secret <secret> [--parts <N>] [--threshold <K>] [--output <dir>]"
      exit 1
      ;;
  esac
done

if [ -z "$SECRET" ]; then
  echo "Error: --secret is required"
  echo "Usage: $0 --secret <secret> [--parts <N>] [--threshold <K>] [--output <dir>]"
  exit 1
fi

echo "Generating shares..."
echo "  Secret length: ${#SECRET} characters"
echo "  Parts (N): $PARTS"
echo "  Threshold (K): $THRESHOLD"
echo "  Output directory: $OUTPUT_DIR"

# Build split-secret tool if not exists
SPLIT_SECRET="$PROJECT_ROOT/bin/split-secret"
if [ ! -f "$SPLIT_SECRET" ]; then
  echo "Building split-secret tool..."
  mkdir -p "$PROJECT_ROOT/bin"
  cd "$PROJECT_ROOT"
  go build -o "$SPLIT_SECRET" ./cmd/split-secret
fi

# Run split-secret
"$SPLIT_SECRET" \
  --secret "$SECRET" \
  --parts "$PARTS" \
  --threshold "$THRESHOLD" \
  --output "$OUTPUT_DIR" \
  --base64

echo ""
echo "✓ Shares generated successfully!"
echo ""
echo "Next steps:"
echo "  1. Update the Helm chart shares secret:"
echo "     kubectl create secret generic hyena-shares \\"
for i in $(seq 0 $(($PARTS - 1))); do
  echo "       --from-file=share-$i.bin=$OUTPUT_DIR/share-$i.b64 \\"
done
echo "       --dry-run=client -o yaml | kubectl apply -f -"
echo ""
echo "  2. Or use the shares directly in your values file"
