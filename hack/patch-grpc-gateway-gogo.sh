#!/bin/bash
set -e

# This script patches grpc-gateway generated files to handle gogo/protobuf compatibility
# It changes the return type from proto.Message to any to avoid type checking issues

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "Patching grpc-gateway files for gogo/protobuf compatibility..."

# Find all .pb.gw.go files
GATEWAY_FILES=$(find "$PROJECT_ROOT/pkg/apiclient" -name "*.pb.gw.go")

for file in $GATEWAY_FILES; do
    echo "Patching: $file"
    
    # Change function signatures from proto.Message to any for request_ and local_request_ functions
    # This allows gogo types to pass through without type checking
    sed -i 's/func \(request_[^(]*\)(\([^)]*\)) (proto\.Message,/func \1(\2) (any,/g' "$file"
    sed -i 's/func \(local_request_[^(]*\)(\([^)]*\)) (proto\.Message,/func \1(\2) (any,/g' "$file"
    
    # Patch forward_ function calls to cast resp back to proto.Message
    sed -i 's/forward_\([^(]*\)(\([^,]*\), \([^,]*\), \([^,]*\), \([^,]*\), \([^,]*\), resp,/forward_\1(\2, \3, \4, \5, \6, resp.(proto.Message),/g' "$file"
    
    # Patch streaming recv() closures - they return (msg, error) tuples
    # Change: func() (proto.Message, error) { return resp.Recv() }
    # To:     func() (proto.Message, error) { msg, err := resp.Recv(); return any(msg).(proto.Message), err }
    sed -i 's/{ return resp\.Recv() }/{ msg, err := resp.Recv(); return any(msg).(proto.Message), err }/g' "$file"
    
    echo "  Patched successfully"
done

echo "All grpc-gateway files patched"
