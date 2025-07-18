#!/bin/bash

# Generate Architecture Diagrams Script
# This script generates PNG and SVG images from Mermaid diagram files

set -e

echo "🎨 Generating Architecture Diagrams..."
echo "======================================"

# Check if mermaid-cli is installed
if ! command -v mmdc &> /dev/null; then
    echo "❌ mermaid-cli (mmdc) is not installed."
    echo "📦 Please install it first:"
    echo "   npm install -g @mermaid-js/mermaid-cli"
    echo ""
    echo "🔧 Or using yarn:"
    echo "   yarn global add @mermaid-js/mermaid-cli"
    exit 1
fi

# Change to diagrams directory
cd "$(dirname "$0")"

# Define diagram files
diagrams=(
    "system-context"
    "container-diagram"
    "component-diagram"
    "file-processing-sequence"
    "api-request-sequence"
    "database-schema"
)

# Create output directory
mkdir -p images

# Generate diagrams
for diagram in "${diagrams[@]}"; do
    if [ -f "$diagram.mmd" ]; then
        echo "📊 Generating $diagram..."
        
        # Generate PNG
        mmdc -i "$diagram.mmd" -o "images/$diagram.png" -t dark --width 1200 --height 800
        
        # Generate SVG
        mmdc -i "$diagram.mmd" -o "images/$diagram.svg" -t dark --width 1200 --height 800
        
        echo "✅ Generated $diagram.png and $diagram.svg"
    else
        echo "⚠️  Warning: $diagram.mmd not found"
    fi
done

echo ""
echo "🎉 Diagram generation complete!"
echo "📁 Images saved in: docs/diagrams/images/"
echo ""
echo "Generated files:"
ls -la images/

echo ""
echo "🌐 You can also view the diagrams online:"
echo "   - GitHub: Renders .mmd files automatically"
echo "   - Mermaid Live Editor: https://mermaid.live/"
echo "   - VS Code: Install Mermaid Preview extension"